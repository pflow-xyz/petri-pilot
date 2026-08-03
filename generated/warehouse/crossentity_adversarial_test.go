package warehouse

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pflow-xyz/go-pflow/eventsource"
	"github.com/pflow-xyz/petri-pilot/generated/warehouse/inventory"
	"github.com/pflow-xyz/petri-pilot/generated/warehouse/order"
)

// These tests attack the cross-entity guard enforcement rather than
// demonstrating it. Each one is written to FAIL if the property it names is
// merely incidental — a store that is never consulted, a fence that is never
// checked, a refusal that only one entry point makes.

// TestReplayIgnoresSiblingStateAndWitness is the strong form of "replay is
// pure". crosslink_test.go's TestReplayIsPureAfterGuardedCommand only proves
// replay does not ERROR when siblings are unreachable, which a code path that
// reads siblings opportunistically would also satisfy. This one makes the
// sibling stream LIE — it reports a marking under which ship could never have
// been authorised — and additionally destroys the witness metadata on the
// order events. If the replayed marking is still right under both, replay
// depends on neither.
func TestReplayIgnoresSiblingStateAndWitness(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "r1", "inventory": "r1"}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}
	if err := app.FireOrderShip(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	want := map[string]int{
		order.PlaceDraft:   0,
		order.PlacePlaced:  0,
		order.PlaceShipped: 1,
	}
	check := func(t *testing.T, label string, agg *order.Aggregate) {
		t.Helper()
		places := agg.Places()
		for id, n := range want {
			if places[id] != n {
				t.Errorf("%s: marking[%s] = %d, want %d (full marking %v)", label, id, places[id], n, places)
			}
		}
	}

	// (a) Every stream but the order's answers with garbage.
	lying := lyingStore{Store: store, truthful: StreamID("order", "r1")}
	agg, err := order.NewApplication(lying).Load(ctx, "r1")
	if err != nil {
		t.Fatalf("replay failed against a lying sibling store: %v", err)
	}
	check(t, "lying siblings", agg)

	// (b) The witness metadata is gone. If applying an event consulted it, a
	// command's decision would be un-replayable once metadata was dropped by,
	// say, a store migration — and one entity's log would stop being
	// sufficient to rebuild that entity.
	events, err := store.Read(ctx, StreamID("order", "r1"), 0)
	if err != nil {
		t.Fatal(err)
	}
	stripped := order.NewAggregate("r1")
	for _, e := range events {
		clone := *e
		clone.Metadata = map[string]string{MetadataCommand: "", MetadataWitness: "{not json"}
		if err := stripped.Apply(&clone); err != nil {
			t.Fatalf("applying an event with no witness failed: %v", err)
		}
	}
	check(t, "witness destroyed", stripped)
}

// TestFusedCommandIsAllOrNothing: a fused command evaluates its members in
// order, and the first member (inventory) can be enabled while the second
// (order) is not. Nothing may be appended in that case — not even to the
// member that would have succeeded.
//
// The reachable state for it: fire the fused command once (order draft ->
// placed, inventory available -> reserved), then release the stock through
// inventory's own local API. Now inventory.reserve_stock is enabled again but
// order.place_order is not.
func TestFusedCommandIsAllOrNothing(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "a1", "inventory": "a1"}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}
	inv := inventory.NewApplication(store)
	if _, err := inv.Execute(ctx, "a1", inventory.TransitionRelease, nil); err != nil {
		t.Fatal(err)
	}

	// Sanity: the first member really would fire on its own, so a partial
	// append is a live possibility rather than a hypothetical. CanFire cannot
	// say so — reserve_stock is a cross-entity command, so it answers false
	// unconditionally — hence the check is on the marking its arc consumes.
	invAgg, err := inv.Load(ctx, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if invAgg.Places()[inventory.PlaceAvailable] < 1 {
		t.Fatalf("setup wrong: inventory cannot re-reserve (%v), so this proves nothing", invAgg.Places())
	}

	before, _ := store.Read(ctx, StreamID("inventory", "a1"), 0)
	if err := app.FireOrderReservesStock(ctx, ids, nil); err == nil {
		t.Fatal("fused command succeeded with order/place_order disabled")
	}
	after, _ := store.Read(ctx, StreamID("inventory", "a1"), 0)
	if len(after) != len(before) {
		t.Errorf("refused fused command appended %d event(s) to the enabled member's log", len(after)-len(before))
	}
}

// TestReadFenceIsActuallyChecked. TestSiblingReadIsFenced in crosslink_test.go
// does NOT test the fence: it advances inventory before FireOrderShip reads
// it, so the command is refused by the condition and never reaches
// MultiAppend. The fence only matters for a writer that lands BETWEEN the read
// and the append, which needs the append itself intercepted.
//
// racyStore does that: it slips an unrelated event into the inventory stream
// as MultiAppend is entered. The recorded condition still held when it was
// evaluated; the question is whether the command notices that the history it
// decided against is no longer the one it is appending to.
func TestReadFenceIsActuallyChecked(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "f2", "inventory": "f2"}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	racy := &racyStore{Store: store, stream: StreamID("inventory", "f2")}
	fenced, err := NewApp(racy)
	if err != nil {
		t.Fatal(err)
	}
	err = fenced.FireOrderShip(ctx, ids, nil)
	if err == nil {
		t.Fatal("ship committed even though inventory moved between the read and the append: the fence is a no-op")
	}
	if !errors.Is(err, eventsource.ErrConcurrencyConflict) {
		t.Errorf("want a concurrency conflict from the fence, got %v", err)
	}
	if !racy.raced {
		t.Fatal("the racy store never intercepted an append — the test proved nothing")
	}
	events, _ := store.Read(ctx, StreamID("order", "f2"), 0)
	if len(events) != 1 {
		t.Errorf("order log has %d events after a fenced-out ship, want 1", len(events))
	}
}

// TestCanFireAgreesWithExecute sweeps every transition of both entities over
// every state reachable in this bundle and requires CanFire to predict
// Execute exactly. A cross-entity refusal that reported "enabled" and then
// failed — or the reverse — would be the divergence this work exists to
// close, and it would be invisible to any test that only asks about ship.
func TestCanFireAgreesWithExecute(t *testing.T) {
	type entity struct {
		name        string
		transitions []string
		canFire     func(*testing.T, eventsource.Store, string, string) bool
		execute     func(context.Context, eventsource.Store, string, string) error
	}
	entities := []entity{
		{
			name:        "order",
			transitions: []string{order.TransitionPlaceOrder, order.TransitionShip},
			canFire: func(t *testing.T, s eventsource.Store, id, tr string) bool {
				agg, err := order.NewApplication(s).Load(context.Background(), id)
				if err != nil {
					t.Fatal(err)
				}
				return agg.CanFire(tr)
			},
			execute: func(ctx context.Context, s eventsource.Store, id, tr string) error {
				_, err := order.NewApplication(s).Execute(ctx, id, tr, nil)
				return err
			},
		},
		{
			name:        "inventory",
			transitions: []string{inventory.TransitionReserveStock, inventory.TransitionRelease},
			canFire: func(t *testing.T, s eventsource.Store, id, tr string) bool {
				agg, err := inventory.NewApplication(s).Load(context.Background(), id)
				if err != nil {
					t.Fatal(err)
				}
				return agg.CanFire(tr)
			},
			execute: func(ctx context.Context, s eventsource.Store, id, tr string) error {
				_, err := inventory.NewApplication(s).Execute(ctx, id, tr, nil)
				return err
			},
		},
	}

	// Each setup leaves the shared store in a different composed state.
	ids := map[string]string{"order": "c1", "inventory": "c1"}
	setups := []struct {
		name  string
		build func(context.Context, eventsource.Store, *App) error
	}{
		{"initial", func(context.Context, eventsource.Store, *App) error { return nil }},
		{"placed+reserved", func(ctx context.Context, _ eventsource.Store, app *App) error {
			return app.FireOrderReservesStock(ctx, ids, nil)
		}},
		{"shipped", func(ctx context.Context, _ eventsource.Store, app *App) error {
			if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
				return err
			}
			return app.FireOrderShip(ctx, ids, nil)
		}},
		{"placed+released", func(ctx context.Context, s eventsource.Store, app *App) error {
			if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
				return err
			}
			_, err := inventory.NewApplication(s).Execute(ctx, "c1", inventory.TransitionRelease, nil)
			return err
		}},
	}

	ctx := context.Background()
	for _, setup := range setups {
		for _, e := range entities {
			for _, tr := range e.transitions {
				t.Run(setup.name+"/"+e.name+"/"+tr, func(t *testing.T) {
					store := eventsource.NewMemoryStore()
					app, err := NewApp(store)
					if err != nil {
						t.Fatal(err)
					}
					if err := setup.build(ctx, store, app); err != nil {
						t.Fatal(err)
					}

					predicted := e.canFire(t, store, "c1", tr)
					err = e.execute(ctx, store, "c1", tr)
					if predicted != (err == nil) {
						t.Errorf("CanFire(%s) = %v but Execute returned %v", tr, predicted, err)
					}
				})
			}
		}
	}
}

// TestEnabledTransitionsNeverOffersACommand: the list a caller drives a UI
// from must never contain a transition the entity will refuse. Checked in
// every composed state, including the one where the underlying state machine
// would happily report ship enabled.
func TestEnabledTransitionsNeverOffersACommand(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "e1", "inventory": "e1"}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	// order is now in "placed": the raw net enables ship here.
	agg, err := order.NewApplication(store).Load(ctx, "e1")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range agg.EnabledTransitions() {
		if command, owned := order.CrossEntityCommand(id); owned {
			t.Errorf("EnabledTransitions offers %s, which command %q owns", id, command)
		}
		if _, err := order.NewApplication(store).Execute(ctx, "e1", id, nil); err != nil {
			t.Errorf("EnabledTransitions offered %s but Execute refused: %v", id, err)
		}
	}
}

// lyingStore answers every stream but one with a fabricated history.
type lyingStore struct {
	eventsource.Store
	truthful string
}

func (s lyingStore) Read(ctx context.Context, streamID string, fromVersion int) ([]*eventsource.Event, error) {
	if streamID == s.truthful {
		return s.Store.Read(ctx, streamID, fromVersion)
	}
	// A history that never happened, in the shape of one that could have.
	return []*eventsource.Event{
		{ID: "lie", StreamID: streamID, Type: "inventory.release", Version: 0},
	}, nil
}

func (s lyingStore) ReadAll(context.Context, eventsource.EventFilter) ([]*eventsource.Event, error) {
	return nil, errors.New("lyingStore: ReadAll is not available during a pure replay")
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

// TestEntityHTTPRefusesACommand closes the last entry point. The refusal lives
// in Application.Execute, and the entity's HTTP handler is a separate caller of
// it — a handler that had its own load-then-fire path would sail straight past
// the check. This asserts the surface a real client actually uses: a 409 that
// names the command, and nothing written.
func TestEntityHTTPRefusesACommand(t *testing.T) {
	ctx := context.Background()
	store := eventsource.NewMemoryStore()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{"order": "h1", "inventory": "h1"}
	// Reserve stock, so the cross-entity precondition genuinely HOLDS. The
	// entity must still refuse: the point is not that ship is impossible, it
	// is that this entity is not the one entitled to decide.
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader(`{"aggregate_id":"h1"}`)
	req := httptest.NewRequest(http.MethodPost, "/ship", body)
	rec := httptest.NewRecorder()
	order.HandleShip(order.NewApplication(store))(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409; body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "order/ship") {
		t.Errorf("response does not name the owning command: %s", rec.Body.String())
	}
	events, _ := store.Read(ctx, StreamID("order", "h1"), 0)
	if len(events) != 1 {
		t.Errorf("order log has %d events after a refused HTTP ship, want 1", len(events))
	}
}
