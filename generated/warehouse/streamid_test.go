package warehouse

import (
	"context"
	"testing"

	"github.com/pflow-xyz/go-pflow/eventsource"
	"github.com/pflow-xyz/petri-pilot/generated/warehouse/inventory"
	"github.com/pflow-xyz/petri-pilot/generated/warehouse/order"
)

// The entity API and the composition root must address the SAME event stream
// for an aggregate. They did not: the entity wrote stream "<id>" while app.go's
// StreamID wrote "order/<id>", so a command firing and an entity firing were
// recorded in two different logs and neither could see the other.
//
// The claim is stated in both directions, because one direction alone would
// pass with the streams still split: what the root writes, the entity reads,
// and what the entity writes, the root reads.
func TestEntityAndCoordinatorShareOneStream(t *testing.T) {
	store := eventsource.NewMemoryStore()
	ctx := context.Background()
	app, err := NewApp(store)
	if err != nil {
		t.Fatal(err)
	}

	const id = "stream-1"
	ids := map[string]string{"order": id, "inventory": id}
	if err := app.FireOrderReservesStock(ctx, ids, nil); err != nil {
		t.Fatalf("place order: %v", err)
	}

	// Root -> entity: the entity's own Load sees what the command appended.
	agg, err := order.NewApplication(store).Load(ctx, id)
	if err != nil {
		t.Fatalf("load order/%s: %v", id, err)
	}
	if agg.Places()[order.PlacePlaced] != 1 {
		t.Errorf("the order entity does not see the command's event: %v", agg.Places())
	}

	// Entity -> root: a purely local transition (release is not a command)
	// lands where the root will read it.
	if _, err := inventory.NewApplication(store).Execute(ctx, id, inventory.TransitionRelease, nil); err != nil {
		t.Fatalf("release: %v", err)
	}
	viaRoot, err := store.Read(ctx, StreamID("inventory", id), 0)
	if err != nil {
		t.Fatalf("read %s: %v", StreamID("inventory", id), err)
	}
	if len(viaRoot) != 2 {
		t.Errorf("%s holds %d events, want 2 (reserve + release)", StreamID("inventory", id), len(viaRoot))
	}

	// And nothing is written to the bare id any more.
	stray, _ := store.Read(ctx, id, 0)
	if len(stray) != 0 {
		t.Errorf("%d events landed on the unprefixed stream %q; the two paths have diverged again", len(stray), id)
	}
}
