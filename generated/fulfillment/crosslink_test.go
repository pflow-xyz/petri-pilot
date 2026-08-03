package fulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/eventsource"
	"github.com/pflow-xyz/petri-pilot/generated/fulfillment/credit"
	"github.com/pflow-xyz/petri-pilot/generated/fulfillment/inventory"
	"github.com/pflow-xyz/petri-pilot/generated/fulfillment/order"
)

// fulfillment is the tree's FUSED+GUARDED case. warehouse exercises the two
// link kinds separately — one fused transition, one guarded transition — so
// the coordinator branch that has to do both at once was generated, compiled
// and never executed. Here one transition is both:
//
//	event: order.place_order  fused with  inventory.reserve_stock
//	guard: order.place_order  gated on    credit.cleared > 0
//
// credit fires nothing. So the coordinator must assemble a marking for an
// entity that is not a member, decide from it, fire the two that ARE members,
// and fence the third in the same atomic append — while each of the three logs
// stays independently replayable.

const (
	// The three markings a credit aggregate can be in are reached through
	// credit's own local transitions; neither is a cross-entity command.
	creditClear  = credit.TransitionClearCredit
	creditRevoke = credit.TransitionRevokeCredit
)

// clearCredit puts the third entity into the state that satisfies the
// condition, through its own entity API — proving the guarded-on entity is
// still an ordinary entity that nothing took a transition away from.
func clearCredit(t *testing.T, store eventsource.Store, id string) {
	t.Helper()
	if _, err := credit.NewApplication(store).Execute(context.Background(), id, creditClear, nil); err != nil {
		t.Fatalf("clearing credit for %s: %v", id, err)
	}
}

// TestCommandIsBothFusedAndGuarded pins the shape itself. If a future bundle
// edit made this command merely fused or merely guarded, every test below
// would still pass while testing nothing new, so the fixture asserts what it
// is before asserting how it behaves.
func TestCommandIsBothFusedAndGuarded(t *testing.T) {
	var cmd *Command
	for i := range Commands {
		if Commands[i].FlatTransition == "order_reserves_stock" {
			cmd = &Commands[i]
		}
	}
	if cmd == nil {
		t.Fatal("order_reserves_stock is not a cross-entity command")
	}
	if cmd.Kind != "fused+guarded" {
		t.Errorf("kind = %q, want fused+guarded — this fixture exists for that shape", cmd.Kind)
	}
	if len(cmd.Members) != 2 {
		t.Errorf("members = %v, want the two fused entities", cmd.Members)
	}
	if len(cmd.Reads) != 1 || cmd.Reads[0].SubnetID != "credit" {
		t.Fatalf("reads = %v, want exactly credit", cmd.Reads)
	}
	// The read entity must NOT also be a member: the whole point is a sibling
	// that is fenced without firing.
	for _, m := range cmd.Members {
		if m.SubnetID == "credit" {
			t.Error("credit is a member; then nothing is fenced-without-firing and the gap is not covered")
		}
	}

	// And the same shape in the flattened model: one transition, two emits,
	// one read arc from a third subnet's place.
	m := FlatModel()
	var emits int
	var found bool
	for _, t2 := range m.Transitions {
		if t2.ID == "order_reserves_stock" {
			found, emits = true, len(t2.Emits)
			if t2.Guard != "" {
				t.Errorf("guard = %q; a structural lowering leaves it empty", t2.Guard)
			}
		}
	}
	if !found {
		t.Fatal("order_reserves_stock missing from the flattened model")
	}
	if emits != 2 {
		t.Errorf("fused transition emits %d events, want one per member entity", emits)
	}
	var read bool
	for i := range m.Arcs {
		a := &m.Arcs[i]
		if a.From == "credit/cleared" && a.To == "order_reserves_stock" {
			read = true
			if !a.IsRead() {
				t.Errorf("credit/cleared -> order_reserves_stock is %q, want a read arc", a.Type)
			}
		}
	}
	if !read {
		t.Error("the guard link did not lower onto the FUSED transition")
	}
}

// TestFusedGuardedRefusesWhenTheThirdEntityIsNotReady is the refusal half.
// Both members are enabled from the initial marking — order has a draft,
// inventory has stock — so the only thing that can stop the command is the
// condition on an entity that fires nothing. Nothing may be appended anywhere.
func TestFusedGuardedRefusesWhenTheThirdEntityIsNotReady(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "n1", "inventory": "n1", "credit": "n1"}

	err = app.FireOrderReservesStock(ctx, ids, nil)
	if err == nil {
		t.Fatal("fused command succeeded with credit/cleared == 0: the third entity's condition is unenforced")
	}
	var notEnabled *NotEnabledError
	if !errAs(err, &notEnabled) {
		t.Fatalf("refusal should be NotEnabledError, got %T: %v", err, err)
	}
	if !strings.Contains(notEnabled.Reason, "credit/cleared") {
		t.Errorf("refusal reason %q does not name the place it read", notEnabled.Reason)
	}

	// A refusal writes nothing — not to the members that would have fired, and
	// not to the sibling that was only read.
	for _, entity := range []string{"order", "inventory", "credit"} {
		events, _ := store.Read(ctx, StreamID(entity, "n1"), 0)
		if len(events) != 0 {
			t.Errorf("%s log has %d events after a refused command, want 0", entity, len(events))
		}
	}
}

// TestFusedGuardedSucceedsWhenTheThirdEntityIsReady is the acceptance half,
// and the assertion that matters is the SHAPE of what landed: one event in
// each member's log, and none in the sibling's. A coordinator that fenced the
// sibling by writing to it would satisfy "the command succeeded" while giving
// credit an event no credit transition ever fired.
func TestFusedGuardedSucceedsWhenTheThirdEntityIsReady(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "y1", "inventory": "y1", "credit": "y1"}
	clearCredit(t, store, "y1")

	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatalf("command must be enabled once credit is cleared: %v", err)
	}

	for _, m := range []struct{ entity, event string }{
		{"order", "order.place_order"},
		{"inventory", "inventory.reserve_stock"},
	} {
		events, _ := store.Read(ctx, StreamID(m.entity, "y1"), 0)
		if len(events) != 1 {
			t.Fatalf("%s log has %d events, want exactly 1", m.entity, len(events))
		}
		if events[0].Type != m.event {
			t.Errorf("%s event type = %q, want %q", m.entity, events[0].Type, m.event)
		}
		if events[0].Metadata[MetadataCommand] != "order_reserves_stock" {
			t.Errorf("%s event is not attributed to its command: %v", m.entity, events[0].Metadata)
		}
	}

	// The fenced sibling: read, fenced, never written.
	events, _ := store.Read(ctx, StreamID("credit", "y1"), 0)
	if len(events) != 1 {
		t.Fatalf("credit log has %d events, want 1 (only its own clear_credit)", len(events))
	}
	if events[0].Type != "credit.clear_credit" {
		t.Errorf("the coordinator wrote %q to the entity it only reads", events[0].Type)
	}
}

// TestThirdEntityConditionIsRecheckedNotCached: credit can move back, and the
// command has to see it. Clear credit, fire successfully for one aggregate,
// then revoke and try a fresh aggregate pair against the same credit stream.
func TestThirdEntityConditionIsRecheckedNotCached(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	clearCredit(t, store, "c9")

	if err := app.FireOrderReservesStock(ctx, map[string]string{"order": "first", "inventory": "first", "credit": "c9"}, nil); err != nil {
		t.Fatalf("first firing should be allowed: %v", err)
	}

	// Revoke through credit's own API — a purely local transition.
	if _, err := credit.NewApplication(store).Execute(ctx, "c9", creditRevoke, nil); err != nil {
		t.Fatal(err)
	}

	ids := map[string]string{"order": "second", "inventory": "second", "credit": "c9"}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err == nil {
		t.Fatal("command succeeded after credit was revoked")
	}
	for _, entity := range []string{"order", "inventory"} {
		events, _ := store.Read(ctx, StreamID(entity, "second"), 0)
		if len(events) != 0 {
			t.Errorf("%s/second has %d events after a refused command, want 0", entity, len(events))
		}
	}
}

// TestReplayIsPureAfterFusedGuardedCommand is the load-bearing claim, stated
// against the hardest command in the tree: its decision came partly from its
// members' own markings and partly from a stream neither member is ever
// allowed to open. Replay each member's log ALONE, against a store that errors
// on every other stream, and require the marking to be right anyway.
func TestReplayIsPureAfterFusedGuardedCommand(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "p1", "inventory": "p1", "credit": "p1"}
	clearCredit(t, store, "p1")
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	{
		stream := StreamID("order", "p1")
		agg, err := order.NewApplication(onlyStream{Store: store, allowed: stream}).Load(ctx, "p1")
		if err != nil {
			t.Fatalf("order replay is not pure — it read outside %s: %v", stream, err)
		}
		want := map[string]int{order.PlaceDraft: 0, order.PlacePlaced: 1, order.PlaceShipped: 0}
		for id, n := range want {
			if agg.Places()[id] != n {
				t.Errorf("order marking[%s] = %d, want %d (full %v)", id, agg.Places()[id], n, agg.Places())
			}
		}
	}
	{
		stream := StreamID("inventory", "p1")
		agg, err := inventory.NewApplication(onlyStream{Store: store, allowed: stream}).Load(ctx, "p1")
		if err != nil {
			t.Fatalf("inventory replay is not pure — it read outside %s: %v", stream, err)
		}
		want := map[string]int{inventory.PlaceAvailable: 0, inventory.PlaceReserved: 1}
		for id, n := range want {
			if agg.Places()[id] != n {
				t.Errorf("inventory marking[%s] = %d, want %d (full %v)", id, agg.Places()[id], n, agg.Places())
			}
		}
	}
	{
		// The fenced sibling too: being read by someone else's command must
		// not make credit depend on anyone else's log.
		stream := StreamID("credit", "p1")
		agg, err := credit.NewApplication(onlyStream{Store: store, allowed: stream}).Load(ctx, "p1")
		if err != nil {
			t.Fatalf("credit replay is not pure — it read outside %s: %v", stream, err)
		}
		if agg.Places()[credit.PlaceCleared] != 1 {
			t.Errorf("credit marking = %v, want cleared == 1", agg.Places())
		}
	}
}

// TestWitnessRecordsTheFencedSibling: the evidence for the decision has to
// name the entity that authorised it and the version it stood at, or the
// decision is unauditable once the credit log is archived.
func TestWitnessRecordsTheFencedSibling(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, _ := NewApp(store)
	ids := map[string]string{"order": "w1", "inventory": "w1", "credit": "w1"}
	clearCredit(t, store, "w1")
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	events, _ := store.Read(ctx, StreamID("order", "w1"), 0)
	if len(events) != 1 {
		t.Fatalf("order log has %d events, want 1", len(events))
	}
	var w Witness
	if err := json.Unmarshal([]byte(events[0].Metadata[MetadataWitness]), &w); err != nil {
		t.Fatalf("witness does not parse: %v", err)
	}
	if w.Places["credit/cleared"] != 1 {
		t.Errorf("witness places = %v, want credit/cleared == 1", w.Places)
	}
	if w.Condition == "" {
		t.Error("witness does not record the condition it evaluated")
	}
	streams := map[string]int{}
	for _, s := range w.Streams {
		streams[s.Stream] = s.Version
	}
	// All three participants, members and the read-only sibling alike.
	for _, entity := range []string{"order", "inventory", "credit"} {
		if _, ok := streams[StreamID(entity, "w1")]; !ok {
			t.Errorf("witness does not record participant %s: %v", entity, w.Streams)
		}
	}
	if v := streams[StreamID("credit", "w1")]; v != 0 {
		t.Errorf("witness recorded credit at version %d, want 0 (one clear_credit event)", v)
	}
}

// TestFenceCoversTheNonMemberSibling is the ordering claim this whole fixture
// exists for. The condition held when it was read; a writer then commits to
// credit BETWEEN the read and the append. The command must not land — the
// marking it decided against is no longer the one it is writing to.
//
// A coordinator that fenced only its FUSION MEMBERS (the obvious way to write
// it, since those are the streams it appends events to) passes every other
// test here and fails this one.
func TestFenceCoversTheNonMemberSibling(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "f1", "inventory": "f1", "credit": "f1"}
	clearCredit(t, store, "f1")
	_ = app

	racy := &racyStore{Store: store, stream: StreamID("credit", "f1")}
	fenced, err := NewApp(racy)
	if err != nil {
		t.Fatal(err)
	}
	err = fenced.FireOrderReservesStock(ctx, ids, nil)
	if err == nil {
		t.Fatal("command committed even though credit moved between the read and the append: the sibling is not fenced")
	}
	if !errors.Is(err, eventsource.ErrConcurrencyConflict) {
		t.Errorf("want a concurrency conflict from the fence, got %v", err)
	}
	if !racy.raced {
		t.Fatal("the racy store never intercepted an append — the test proved nothing")
	}
	for _, entity := range []string{"order", "inventory"} {
		events, _ := store.Read(ctx, StreamID(entity, "f1"), 0)
		if len(events) != 0 {
			t.Errorf("%s log has %d events after a fenced-out command, want 0", entity, len(events))
		}
	}
}

// TestGuardedOnEntityKeepsItsOwnTransitions: putting a guard link ONTO an
// entity's place must not take anything away from that entity. credit fires
// nothing in the command, so both of its transitions stay local and usable.
func TestGuardedOnEntityKeepsItsOwnTransitions(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	capp := credit.NewApplication(store)

	if _, err := capp.Execute(ctx, "k1", creditClear, nil); err != nil {
		t.Fatalf("credit lost its own clear_credit: %v", err)
	}
	if _, err := capp.Execute(ctx, "k1", creditRevoke, nil); err != nil {
		t.Fatalf("credit lost its own revoke_credit: %v", err)
	}
	// And nothing lifted them: credit appears in no command's member list.
	for _, c := range Commands {
		for _, m := range c.Members {
			if m.SubnetID == "credit" {
				t.Errorf("credit.%s was lifted into command %q; credit is only READ by it", m.LocalTransition, c.FlatTransition)
			}
		}
	}
}

// onlyStream fails any read outside one stream, so "replay does not consult
// siblings" becomes an assertion the test can make rather than a property
// asserted about the code by reading it. (Copied from
// generated/warehouse/crosslink_test.go — same harness, harder command.)
type onlyStream struct {
	eventsource.Store
	allowed string
}

func (s onlyStream) Read(ctx context.Context, streamID string, fromVersion int) ([]*eventsource.Event, error) {
	if streamID != s.allowed {
		return nil, &crossStreamReadError{stream: streamID, allowed: s.allowed}
	}
	return s.Store.Read(ctx, streamID, fromVersion)
}

func (s onlyStream) ReadAll(ctx context.Context, filter eventsource.EventFilter) ([]*eventsource.Event, error) {
	return nil, &crossStreamReadError{stream: "<ReadAll>", allowed: s.allowed}
}

type crossStreamReadError struct{ stream, allowed string }

func (e *crossStreamReadError) Error() string {
	return "replay read " + e.stream + "; only " + e.allowed + " is available"
}

// racyStore advances one stream at the moment MultiAppend is entered, standing
// in for a concurrent writer that commits between a command's read and its
// append.
type racyStore struct {
	eventsource.Store
	stream string
	raced  bool
}

func (s *racyStore) MultiAppend(ctx context.Context, appends []eventsource.StreamAppend) error {
	if !s.raced {
		s.raced = true
		if _, err := s.Store.Append(ctx, s.stream, -2, []*eventsource.Event{
			{Type: "concurrent.writer", StreamID: s.stream},
		}); err != nil {
			return err
		}
	}
	return s.Store.(eventsource.MultiAppender).MultiAppend(ctx, appends)
}
