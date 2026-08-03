package warehouse

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/eventsource"
	"github.com/pflow-xyz/petri-pilot/generated/warehouse/inventory"
	"github.com/pflow-xyz/petri-pilot/generated/warehouse/order"
)

// warehouse is the tree's guard-link case: the bundle declares
//
//	guard: order.ship  gated on  inventory.reserved > 0
//
// which is a cross-subnet precondition that consumes nothing. These tests
// follow it all the way from the bundle document into runtime behaviour.

// TestGuardLinkReachesFlatModel: composition lowered the link over the
// flattened place ID, and the generator embedded the result.
//
// The lowering is STRUCTURAL, not an expression: "> 0" becomes a read arc
// inventory/reserved -> order/ship, and Transition.Guard stays empty. That
// distinction is load-bearing for the generator — a command table keyed on
// Transition.Guard alone sees nothing here — so it is pinned rather than left
// implicit.
func TestGuardLinkReachesFlatModel(t *testing.T) {
	m := FlatModel()
	if m == nil {
		t.Fatal("no flat model embedded")
	}

	var found bool
	for _, tr := range m.Transitions {
		if tr.ID == "order/ship" {
			found = true
			if tr.Guard != "" {
				t.Errorf("order/ship guard = %q; a structural lowering leaves it empty", tr.Guard)
			}
		}
	}
	if !found {
		t.Fatal("order/ship missing from the flattened model")
	}

	var read bool
	for i := range m.Arcs {
		a := &m.Arcs[i]
		if a.From == "inventory/reserved" && a.To == "order/ship" {
			read = true
			if !a.IsRead() {
				t.Errorf("inventory/reserved -> order/ship is %q, want a read arc", a.Type)
			}
		}
	}
	if !read {
		t.Error("the guard link did not lower to an arc on inventory/reserved")
	}
}

// TestEventLinkFusedTransition: the other link kind fused two transitions into
// one, with one event per member.
func TestEventLinkFusedTransition(t *testing.T) {
	m := FlatModel()

	for _, tr := range m.Transitions {
		if tr.ID != "order_reserves_stock" {
			continue
		}
		if len(tr.Emits) != 2 {
			t.Errorf("fused transition emits %v, want one event per member entity", tr.Emits)
		}
		return
	}
	t.Error("the event link did not produce a fused transition")
}

// TestCrossEntityGuardIsEnforced is the inversion of a test that used to pin
// the gap. order/ship carries a precondition on inventory's marking; the order
// aggregate replays only its own log and cannot see it, so the entity API must
// refuse the transition outright and the composition root must be the only way
// to fire it.
//
// Firing ship with nothing reserved used to SUCCEED. That was the bug.
func TestCrossEntityGuardIsEnforced(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	orderApp := order.NewApplication(store)

	// The entity path refuses ship, and says why.
	if _, err := orderApp.Execute(ctx, "w1", order.TransitionShip, nil); err == nil {
		t.Fatal("order.Execute(ship) succeeded: the cross-entity precondition is unenforced")
	} else if !strings.Contains(err.Error(), "order/ship") {
		t.Errorf("refusal should name the command that owns ship, got %q", err)
	}
	agg := order.NewAggregate("w1")
	if agg.CanFire(order.TransitionShip) {
		t.Error("order.CanFire(ship) is true while Execute refuses — Enabled/Execute divergence")
	}

	// The composition root refuses too while nothing is reserved.
	ids := map[string]string{"order": "w1", "inventory": "w1"}
	err = app.FireOrderShip(ctx, ids, nil)
	if err == nil {
		t.Fatal("FireOrderShip succeeded with inventory/reserved == 0")
	}
	var notEnabled *NotEnabledError
	if !errAs(err, &notEnabled) {
		t.Fatalf("refusal should be NotEnabledError, got %T: %v", err, err)
	}
	if !strings.Contains(notEnabled.Reason, "inventory/reserved") {
		t.Errorf("refusal reason %q does not name the place it read", notEnabled.Reason)
	}

	// Place the order, which reserves stock in the same atomic step, and ship
	// now succeeds — the precondition is satisfied, not merely unchecked.
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}
	if err := app.FireOrderShip(ctx, ids, nil); err != nil {
		t.Fatalf("ship must be enabled once stock is reserved: %v", err)
	}
}

// TestFusedMemberCannotFireStandalone pins the second hole. Firing
// inventory.reserve_stock through the entity API moved available -> reserved
// with no order event, so the event link's rendezvous held only on the
// coordinator path — a member could desynchronise the composition using its
// own API.
func TestFusedMemberCannotFireStandalone(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	inv := inventory.NewApplication(store)

	if _, err := inv.Execute(ctx, "f1", inventory.TransitionReserveStock, nil); err == nil {
		t.Fatal("inventory.Execute(reserve_stock) succeeded standalone: the rendezvous is bypassable")
	}
	if _, err := inventory.NewAggregate("f1").Fire(inventory.TransitionReserveStock, nil); err == nil {
		t.Fatal("inventory.Fire(reserve_stock) succeeded standalone")
	}

	// Nothing was written: a refusal must not leave a partial history.
	events, _ := store.Read(ctx, StreamID("inventory", "f1"), 0)
	if len(events) != 0 {
		t.Errorf("refused standalone firing appended %d events", len(events))
	}
}

// TestReplayIsPureAfterGuardedCommand is the load-bearing claim of this
// design, stated against the command that actually reads another entity.
// (app.go's generated TestReplayIsPure makes the same claim for the fused
// command; this one is the harder case, because ship's decision came from a
// stream the order aggregate is never allowed to open.)
//
// order/ship is decided by reading inventory. If rebuilding the order
// aggregate needed that inventory stream — or the witness recorded in the
// event — then no entity could be restored on its own, backups would have to
// be consistent across entities, and replay would stop being a pure fold over
// one log. So: replay the order log alone against a store that ERRORS on every
// other stream, and require the marking to be right anyway.
func TestReplayIsPureAfterGuardedCommand(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "p1", "inventory": "p1"}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}
	if err := app.FireOrderShip(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	orderStream := StreamID("order", "p1")
	isolated := order.NewApplication(onlyStream{Store: store, allowed: orderStream})
	agg, err := isolated.Load(ctx, "p1")
	if err != nil {
		t.Fatalf("replay is not pure — it read outside %s: %v", orderStream, err)
	}

	places := agg.Places()
	want := map[string]int{
		order.PlaceDraft:   0,
		order.PlacePlaced:  0,
		order.PlaceShipped: 1,
	}
	for id, n := range want {
		if places[id] != n {
			t.Errorf("replayed marking[%s] = %d, want %d (full marking %v)", id, places[id], n, places)
		}
	}
}

// TestWitnessRecordsWhatWasRead: the evidence for a cross-entity decision
// travels with the event, so the decision stays auditable when the sibling log
// is gone. This is ReplayWitness's precondition — without it, the only record
// of why ship was allowed lives in a stream the order aggregate may never see
// again.
func TestWitnessRecordsWhatWasRead(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, _ := NewApp(store)
	ids := map[string]string{"order": "w2", "inventory": "w2"}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}
	if err := app.FireOrderShip(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	events, _ := store.Read(ctx, StreamID("order", "w2"), 0)
	if len(events) != 2 {
		t.Fatalf("order log has %d events, want 2", len(events))
	}
	ship := events[1]
	if ship.Metadata[MetadataCommand] != "order/ship" {
		t.Errorf("ship event not attributed to its command: %v", ship.Metadata)
	}

	var w Witness
	if err := json.Unmarshal([]byte(ship.Metadata[MetadataWitness]), &w); err != nil {
		t.Fatalf("witness does not parse: %v", err)
	}
	if w.Places["inventory/reserved"] != 1 {
		t.Errorf("witness places = %v, want inventory/reserved == 1", w.Places)
	}
	if w.Condition == "" {
		t.Error("witness does not record the condition it evaluated")
	}
	var fenced bool
	for _, s := range w.Streams {
		if s.Stream == StreamID("inventory", "w2") {
			fenced = true
			if s.Version != 0 {
				t.Errorf("witness recorded inventory at version %d, want 0 (one reserve event)", s.Version)
			}
		}
	}
	if !fenced {
		t.Error("witness does not record the version of the stream it read")
	}
}

// TestSiblingReadIsStaleChecked: a sibling that moved BEFORE the command runs
// is caught by the condition, which is evaluated against the marking that
// actually exists rather than a cached one.
//
// This does NOT exercise the read fence, despite what the name it used to
// carry implied: the release below lands before FireOrderShip reads inventory,
// so the command is refused by the condition and never reaches MultiAppend.
// Deleting the fence outright leaves this test green. The fence — a zero-event
// StreamAppend at the version read — only matters for a writer that commits
// BETWEEN the read and the append, and that needs the append intercepted; see
// TestReadFenceIsActuallyChecked in crossentity_adversarial_test.go.
func TestSiblingReadIsStaleChecked(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, _ := NewApp(store)
	ids := map[string]string{"order": "s1", "inventory": "s1"}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	// Move inventory on behind the coordinator's back: release returns the
	// reserved token, so the precondition no longer holds. release is a purely
	// local transition, so the entity API is the right way to fire it.
	inv := inventory.NewApplication(store)
	if _, err := inv.Execute(ctx, "s1", inventory.TransitionRelease, nil); err != nil {
		t.Fatal(err)
	}

	if err := app.FireOrderShip(ctx, ids, nil); err == nil {
		t.Fatal("ship succeeded after the stock it depends on was released")
	}
	events, _ := store.Read(ctx, StreamID("order", "s1"), 0)
	if len(events) != 1 {
		t.Errorf("order log has %d events after a refused ship, want 1", len(events))
	}
}

// TestFlatModelIsValidJSON guards the embedding itself.
func TestFlatModelIsValidJSON(t *testing.T) {
	m := FlatModel()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("flat model does not round-trip: %v", err)
	}
	if !strings.Contains(string(b), "inventory/reserved") {
		t.Error("flattened place IDs missing from the embedded model")
	}
}

// onlyStream fails any read outside one stream, so "replay does not consult
// siblings" becomes an assertion the test can make rather than a property
// asserted about the code by reading it.
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
