package sim_test

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"testing"

	"github.com/pflow-xyz/petri-pilot/generated/cafe"
	"github.com/pflow-xyz/petri-pilot/pkg/runtime/sim"
)

// Fitness gates for capacity planning.
//
// "Is this model fit for answering a staffing question?" was, until these
// existed, an opinion. Every gate below is one of the three defects measured in
// the mass-action café, or an independent cross-check against textbook queueing
// theory. Each names the number the broken café produced, so a reader can see
// the tolerance has teeth rather than being wide enough to admit anything.
//
// The teeth were checked, not assumed: the engine was temporarily put back to
// mass action (compile's input arcs forced kinetic) and the gates re-run.
// GATES 1, 2, 4, 5 and 6 fail against it. GATE 3 does not, and that is not a
// hole — it guards the other half of the fix, the per-drink queue, and the
// comment on it says so. One claim in GATE 5 turned out to be blunt when
// measured; it is still there, but the sharp assertion is now beside it and the
// comment explains which is which. A gate nobody has watched fail is a guess.
//
// One thing these gates could not catch was themselves. Every headcount sweep
// below used to override the pantry's restock rates, so that the milk supply —
// which the shipped model really was limited by — could not decide a staffing
// answer. That made all five green against a shop that was never shipped: 224
// served and 42 walked out here, 190 and 75 in the model an operator would run.
// A gate may not run a scenario the operator cannot. The model was sized so the
// intended constraint binds, the override is gone, and GATE 6 asserts what it
// used to assume.
//
// The old café, measured over eight hours:
//
//	per-drink brew time     4.8 min at 1 barista, 2.2 min at 8 — service got
//	                        faster the more work was in progress
//	utilization             39% busy at 3 baristas against 2.55 erlangs of
//	                        offered load, which is arithmetically impossible
//	served mix              ordered 81/122/65 espresso/latte/cappuccino,
//	                        served 110/137/8.5 — more espressos than anyone
//	                        asked for, and cappuccino essentially never made
//
// These drive the composed app over HTTP, like cafe_e2e_test.go, because the
// scenario endpoint is the thing an operator actually asks.

const (
	gateHours = 8.0
	gateReals = 24
	// A fine sample grid. This used to be load-bearing for the metrics — they
	// were the average of the reported points, so a 60-point grid over eight
	// hours measured the queue once every eight minutes and called it the mean,
	// and 480 was how much of that error was bought off. Metrics.Mean and P95
	// are time-weighted now and do not move with this number at all; what still
	// reads off the grid is Depleted.At, which GATE 5 uses, so the resolution
	// stays.
	gateSamples = 480
	// Fixed, so a gate that moves is the model moving and not the dice. The
	// value is arbitrary; that it never changes is not.
	gateSeed = 20260807
)

var gateDrinks = []string{"espresso", "latte", "cappuccino"}

// runGate posts a scenario and returns its answer.
func runGate(t *testing.T, h http.Handler, s sim.Scenario) *sim.Result {
	t.Helper()
	body, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	rec := post(t, h, "/api/scenario", string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var res sim.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Metrics == nil {
		t.Fatal("no metrics on an SSA run")
	}
	return &res
}

// staffing is the scenario these gates vary: n baristas, everything else
// exactly as the shipped model declares it.
//
// "Exactly" is load-bearing and used not to be. These gates were first written
// with the three restock rates overridden to 20, because at the shipped
// deliveries the milk supply — not the baristas — decided the answer over most
// of the sweep, and a staffing gate that measures the pantry is measuring the
// wrong thing. Overriding it was the wrong fix: it certified a shop nobody
// ships. At n=8 the gate table read 224 served and 42 walked out while the
// shipped model produced 190 and 75, and GATE 5's sharp assertion passed at
// 0.835 against a shipped 0.660. A gate may not run a scenario the operator
// cannot. The pantry was sized up in the model instead — see
// cafe-pantry.json's note — and TestGateTheShopIsStaffBound now asserts what
// the override used to assume.
func staffing(n int, extraRates map[string]float64) sim.Scenario {
	var rates map[string]float64
	if len(extraRates) > 0 {
		rates = map[string]float64{}
		for id, r := range extraRates {
			rates[id] = r
		}
	}
	return sim.Scenario{
		Marking:      map[string]int{"staff/available": n},
		Rates:        rates,
		Horizon:      gateHours,
		Samples:      gateSamples,
		Realizations: gateReals,
		Seed:         gateSeed,
	}
}

// demand is what the model's declared rates say about the load it is under.
//
// Derived, never hardcoded: a future edit to an order rate or a brew rate should
// move what these gates expect, not break them. The one number a reader will
// recognise — 2.55 erlangs — is a consequence of the rates, not an input.
type demand struct {
	orders  map[string]float64 // arrivals per hour, per drink
	service map[string]float64 // hours a barista is held, per drink
	start   map[string]float64 // rate at which a queued order becomes a brew
	abandon map[string]float64 // rate at which a queued customer gives up
	// arrivals is the total order rate; offered is the load in erlangs, which is
	// the number of baristas a shop would need if nobody ever had to wait.
	arrivals, offered float64
}

// abandonFloor is how many customers the declared patience loses over the
// horizon in a shop with a barista always free: nobody queues, but an order
// still spends the pickup interval in pending_X, and a customer can give up
// inside it. It is 12/h of patience against 720/h of pickup, so about 1.6% of
// arrivals — four or five customers over eight hours.
//
// This is a property of the rates and not of staffing, so hiring cannot buy it
// back, and GATE 5 uses it as a two-sided band: a sweep that reaches zero
// walkouts is not a better shop, it is a model in which a queue drains faster
// than any rate it declares.
//
// It used to be a sixth of all arrivals. The queue arc was kinetic, so a
// waiting order was picked up at 60/h *per waiting customer* against patience
// of 12/h per waiting customer — the two scaled together and cancelled, leaving
// exactly five in six started at every queue length and every headcount.
// Walkouts converged on 16.6% with every barista idle, "get walkouts under 10%"
// had no answer, and GATE 4 read that artifact as a property to preserve.
func (d demand) abandonFloor() float64 {
	var lost float64
	for drink, arrivals := range d.orders {
		a, s := d.abandon[drink], d.start[drink]
		lost += arrivals * gateHours * a / (a + s)
	}
	return lost
}

func modelDemand(t *testing.T) demand {
	t.Helper()
	rates := sim.Rates(cafe.FlatModel())
	d := demand{
		orders:  map[string]float64{},
		service: map[string]float64{},
		start:   map[string]float64{},
		abandon: map[string]float64{},
	}
	for _, drink := range gateDrinks {
		order, ok := rates["counter/order_"+drink]
		if !ok {
			t.Fatalf("no order transition for %s", drink)
		}
		// A barista is held from the fused start to the fused finish, so the
		// service time is the finish rate's mean, not the whole lead time.
		finish, ok := rates[drink+"_finished"]
		if !ok || finish <= 0 {
			t.Fatalf("no usable finish rate for %s", drink)
		}
		start, ok := rates[drink+"_started"]
		if !ok {
			t.Fatalf("no start transition for %s", drink)
		}
		abandon, ok := rates["counter/abandon_"+drink]
		if !ok {
			t.Fatalf("no abandon transition for %s", drink)
		}
		d.orders[drink] = order
		d.service[drink] = 1 / finish
		d.start[drink] = start
		d.abandon[drink] = abandon
		d.arrivals += order
		d.offered += order / finish
	}
	return d
}

// GATE 1 — service time is invariant to headcount.
//
// Little's law on the brewing stage: the mean number of drinks in progress
// divided by the completion rate is the time one drink takes. That is a
// property of the espresso machine, so hiring must not move it.
//
// The old engine put the barista pool in the propensity product, which made
// finish_X fire at rate * brewing_X * busy — two drinks in progress finished
// each other early. Measured per-drink brew time fell from 4.8 min at one
// barista to 2.2 min at eight. A 12% band on each drink and a 1.15 flatness
// ratio both reject that by a wide margin.
func TestGateServiceTimeIsInvariantToHeadcount(t *testing.T) {
	h := testApp(t)
	d := modelDemand(t)

	observed := map[string][]float64{} // drink -> service time in minutes, by n

	for n := 1; n <= 8; n++ {
		res := runGate(t, h, staffing(n, nil))
		for _, drink := range gateDrinks {
			inFlight := res.Metrics.Mean["counter/brewing_"+drink]
			completions := res.Metrics.Throughput[drink+"_finished"] / gateHours
			if completions <= 0 {
				t.Fatalf("n=%d: no %s was ever finished", n, drink)
			}
			minutes := inFlight / completions * 60
			observed[drink] = append(observed[drink], minutes)

			declared := d.service[drink] * 60
			if rel := math.Abs(minutes-declared) / declared; rel > 0.12 {
				t.Errorf("n=%d: %s takes %.2f min to brew, %.0f%% off the model's declared %.2f",
					n, drink, minutes, rel*100, declared)
			}
		}
	}

	for _, drink := range gateDrinks {
		lo, hi := observed[drink][0], observed[drink][0]
		for _, v := range observed[drink] {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		t.Logf("%-11s brew time over 1..8 baristas: %.2f - %.2f min (declared %.2f)",
			drink, lo, hi, d.service[drink]*60)
		if hi/lo > 1.15 {
			t.Errorf("%s brews in %.2f min with one barista and %.2f with eight — "+
				"headcount is changing how long the machine takes", drink, observed[drink][0], observed[drink][7])
		}
	}
}

// GATE 2 — utilization matches offered load.
//
// The cross-check that does not go through the simulator's own bookkeeping.
// M/M/c says a stable pool carries all the work offered to it, so busy servers
// equals offered load: utilization is load/n, and below the load the pool
// saturates and the queue grows without bound.
//
// Abandonment is switched off for this gate, and only for this gate. With
// customers walking out, the pool never sees the full offered load and the
// comparison against the textbook has a fudge factor in it; with abandonment
// off, load/n is exact and there is nothing to tune. What the shipped model
// does with impatient customers is GATE 4's subject.
//
// The old engine reported 39% busy at three baristas against 2.55 erlangs —
// 85% is the only arithmetic that can be right, and the 10% band below rejects
// 39% by a factor of two.
func TestGateUtilizationMatchesOfferedLoad(t *testing.T) {
	h := testApp(t)
	d := modelDemand(t)

	patient := map[string]float64{}
	for _, drink := range gateDrinks {
		patient["counter/abandon_"+drink] = 0
	}

	t.Logf("offered load %.2f erlangs (%.0f arrivals/h x %.2f min mean service)",
		d.offered, d.arrivals, d.offered/d.arrivals*60)

	// Overloaded: fewer baristas than the work needs. The pool must be pinned
	// near 1 and the queue must still be growing at the horizon.
	for n := 1; n <= int(d.offered); n++ {
		res := runGate(t, h, staffing(n, patient))
		got := res.Metrics.Utilization["staff"]
		if got < 0.95 {
			t.Errorf("n=%d is below the offered load of %.2f but only %.0f%% busy; "+
				"a saturated pool has nothing to be idle for", n, d.offered, got*100)
		}
		queued := 0.0
		for _, drink := range gateDrinks {
			queued += res.Final["counter/pending_"+drink]
		}
		if queued < 10 {
			t.Errorf("n=%d: queue ended at %.1f under an offered load of %.2f; "+
				"an overloaded shop's queue grows", n, queued, d.offered)
		}
		t.Logf("n=%d (overloaded): utilization %.0f%%, queue ends at %.0f", n, got*100, queued)
	}

	// At and above the load the queue clears, so the pool carries exactly the
	// work offered to it and utilization is arithmetic. Every measurement sits
	// a little below theory — 82.8% against 85.0% at three baristas — because
	// the run starts with an empty shop and the first hour is spent filling it.
	// That gap is now the transient and nothing else: the metric is a
	// time-weighted integral over the trajectory, so it no longer also carries
	// the discretization error that made the same run read 81.7% on the default
	// 60-point grid and 74.1% on a coarse one, always low. Half this band used
	// to go on that. The band is unchanged, because the number it has to reject
	// is the mass-action engine's 39%.
	for n := int(math.Ceil(d.offered)); n <= 8; n++ {
		res := runGate(t, h, staffing(n, patient))
		got := res.Metrics.Utilization["staff"]
		want := d.offered / float64(n)
		if rel := math.Abs(got-want) / want; rel > 0.10 {
			t.Errorf("n=%d: %.1f%% busy, want %.1f%% (%.2f erlangs over %d servers) — off by %.0f%%",
				n, got*100, want*100, d.offered, n, rel*100)
		}
		t.Logf("n=%d: utilization %.1f%%, theory %.1f%%", n, got*100, want*100)
	}
}

// GATE 3 — served mix tracks ordered mix.
//
// Abandonment is the same rate on every queue and a barista is drink-blind, so
// the fraction of customers who give up is the same for every drink and the
// served mix must be the ordered mix. This is the gate the old café failed most
// visibly: one fungible orders_pending place meant which drink got made was
// decided by the recipes' ingredient combinatorics, so the shop served 110
// espressos against 81 ordered and made 8 cappuccinos against 65. Ordered
// shares are .30/.45/.24; that shop served .43/.53/.03, so a 5-point band on
// each share rejects it several times over.
//
// Worth being precise about what this gate guards, because it was checked:
// reverting the *engine* to mass action does not break it. The mix survives
// because pending_espresso/latte/cappuccino are three places, so an order for a
// cappuccino is the only thing that can start one, whatever the rate law says.
// It is the queue split this gate pins, and putting the queue back into one
// fungible place is the change it exists to refuse.
func TestGateServedMixTracksOrderedMix(t *testing.T) {
	h := testApp(t)
	d := modelDemand(t)

	for _, n := range []int{3, 8} {
		res := runGate(t, h, staffing(n, nil))

		var ordered, served float64
		perDrink := map[string][2]float64{}
		for _, drink := range gateDrinks {
			o := res.Metrics.Throughput["counter/order_"+drink]
			s := res.Metrics.Throughput["counter/serve_"+drink]
			perDrink[drink] = [2]float64{o, s}
			ordered += o
			served += s
		}
		if served == 0 {
			t.Fatalf("n=%d: nothing was served at all", n)
		}

		for _, drink := range gateDrinks {
			o, s := perDrink[drink][0], perDrink[drink][1]
			t.Logf("n=%d %-11s ordered %.0f served %.0f (share %.2f vs demand %.2f)",
				n, drink, o, s, s/served, d.orders[drink]/d.arrivals)

			// A drink can only be served because an order produced the token it
			// consumed, so this is structural, not a tolerance on noise. The
			// half-drink slack is the mean of a count, not a fudge factor.
			if s > o+0.5 {
				t.Errorf("n=%d: served %.1f %s against %.1f ordered — the shop is making drinks nobody asked for",
					n, s, drink, o)
			}
			want := d.orders[drink] / d.arrivals
			if got := s / served; math.Abs(got-want) > 0.05 {
				t.Errorf("n=%d: %s is %.0f%% of what was served but %.0f%% of what was ordered",
					n, drink, got*100, want*100)
			}
		}
	}
}

// GATE 4 — abandonment falls with headcount, and staffing has a knee.
//
// Two claims an operator is actually buying. Every extra barista must save
// customers — otherwise the model cannot answer "should I hire?" at all — and
// the saving must run out, otherwise the model says hire forever and is worse
// than useless as a budget.
//
// This is also the table a human reads.
func TestGateStaffingHasAKnee(t *testing.T) {
	h := testApp(t)
	d := modelDemand(t)

	served := make([]float64, 9)
	walked := make([]float64, 9)

	t.Logf("offered load %.2f erlangs; %d realizations x %.0fh, seed %d",
		d.offered, gateReals, gateHours, gateSeed)
	arrivals := d.arrivals * gateHours
	t.Logf(" n  served  left  lost  utilization  mean queue")
	for n := 1; n <= 8; n++ {
		res := runGate(t, h, staffing(n, nil))
		served[n] = res.Final["counter/orders_complete"]
		walked[n] = res.Final["counter/walked_out"]

		queue := 0.0
		for _, drink := range gateDrinks {
			queue += res.Metrics.Mean["counter/pending_"+drink]
		}
		t.Logf("%2d  %6.0f  %4.0f  %3.0f%%  %10.0f%%  %10.2f",
			n, served[n], walked[n], walked[n]/arrivals*100,
			res.Metrics.Utilization["staff"]*100, queue)
	}

	// Hiring never costs customers, at any headcount.
	for n := 2; n <= 8; n++ {
		if walked[n] > walked[n-1]+0.5 {
			t.Errorf("barista %d cost customers: %.0f walked out against %.0f with one fewer",
				n, walked[n], walked[n-1])
		}
	}
	// And it strictly saves them while the pool is still the binding
	// constraint — up to one past the offered load.
	for n := 2; n <= int(math.Ceil(d.offered))+1; n++ {
		if walked[n] >= walked[n-1] {
			t.Errorf("barista %d saved nobody at an offered load of %.2f: %.0f walked out against %.0f with one fewer",
				n, d.offered, walked[n], walked[n-1])
		}
	}

	// And the sweep has to contain an answer. The question an operator brings to
	// this model is "how many baristas do I need to stop losing people", so some
	// headcount in the sweep must get losses under a tenth of arrivals, and it
	// must not be the first one — a shop that is comfortable with one barista
	// was never being asked a staffing question.
	//
	// This is the assertion the model could not have passed. With the queue arc
	// kinetic, pickup and patience both scaled with the queue and cancelled: a
	// sixth of every arrival walked out at every headcount, measured 21% at four
	// baristas and 16.6% at eight, converging on 1/6 from above and never
	// crossing 10%. GATE 4 used to assert that floor was preserved.
	const target = 0.10
	enough := 0
	for n := 1; n <= 8; n++ {
		if walked[n]/arrivals < target {
			enough = n
			break
		}
	}
	switch {
	case enough == 0:
		t.Errorf("no headcount up to 8 got walkouts under %.0f%% of %.0f arrivals (best %.0f at n=8, %.0f%%) — "+
			"the model cannot answer the question it exists to answer",
			target*100, arrivals, walked[8], walked[8]/arrivals*100)
	case enough == 1:
		t.Errorf("one barista already loses only %.0f%% of %.0f arrivals at an offered load of %.2f erlangs; "+
			"there is no staffing question here to get wrong", walked[1]/arrivals*100, arrivals, d.offered)
	default:
		t.Logf("walkouts fall under %.0f%% of arrivals at %d baristas (%.0f of %.0f)",
			target*100, enough, walked[enough], arrivals)
	}

	// The knee. The first hire is worth several of the last; if it were not,
	// the model would be reporting staffing as free.
	first := served[2] - served[1]
	last := served[8] - served[7]
	t.Logf("marginal drinks served: 1->2 baristas %+.0f, 7->8 %+.0f", first, last)
	if last >= first/4 {
		t.Errorf("the eighth barista adds %.0f drinks against the second's %.0f — "+
			"this model says hiring never stops paying", last, first)
	}
}

// GATE 5 — the pantry does not accelerate.
//
// A well-stocked shop is not a faster shop. Under mass action the stock levels
// entered the firing rate as C(beans,15) * C(milk,50) * cups, so a delivery
// made the coffee pour quicker and a latte beat a cappuccino for using more
// milk. Marked non-kinetic, the stock gates the brew and is drawn down by it
// and does nothing else.
//
// Asserted two ways, because the obvious way turns out to be blunt.
//
// The obvious way is the invariant: two shops differing only in stock must run
// identically. SSA is seeded per realization, so if the stock genuinely never
// reaches a propensity that is exact equality, not a tolerance. It is a sharp
// statement of the property and it is worth pinning. But it does *not* catch
// mass action, and the comment here said it did until that was measured: run
// against the old rate law, both sides still agree. C(2000,20) is 4e47 and
// C(600,20) is 2e39, and against rates of order 10/h both are simply infinite —
// a brew starts the instant an order exists either way, so the two shops go on
// matching. Any pantry big enough to run the shop is already saturated. The
// factor of 1e8 between them is real and completely unobservable.
//
// Where mass action *is* observable is the race it wins. An order in the queue
// leaves by one of two doors, started or abandoned, and how many take the second
// one is decided by the rates the model declares — 12/h of patience against
// 720/h of pickup, so about 1.6% of arrivals in a shop where the pool is never
// the constraint. Multiply the start side by C(beans,15) * C(milk,50) * cups and
// nobody ever walks out at all: the old engine reported exactly zero from four
// baristas up. So the second assertion measures that loss against the floor the
// declared rates give, and it is two-sided — zero means the stock is in the rate
// law, and several times the floor means something else is holding orders in the
// queue, which is what the undersized pantry used to do.
func TestGatePantryDoesNotAccelerate(t *testing.T) {
	h := testApp(t)
	d := modelDemand(t)

	noRestock := map[string]float64{
		"pantry/restock_coffee_beans": 0,
		"pantry/restock_milk":         0,
		"pantry/restock_cups":         0,
	}
	// Half an hour, because the shop has to be able to run the whole horizon out
	// of the lean pantry without restocking — a long horizon would force the
	// "lean" side to be stocked like the full one and there would be no contrast
	// left to test.
	run := func(beans, milk, cups int) *sim.Result {
		return runGate(t, h, sim.Scenario{
			Marking: map[string]int{
				"staff/available":     2,
				"pantry/coffee_beans": beans,
				"pantry/milk":         milk,
				"pantry/cups":         cups,
			},
			Rates:        noRestock,
			Horizon:      0.5,
			Samples:      gateSamples,
			Realizations: gateReals,
			Seed:         gateSeed,
		})
	}

	full := run(2000, 1000, 500)
	lean := run(600, 700, 60)

	// If the lean shop ran dry the comparison is between a fast shop and a
	// stopped one, which proves nothing about the rate law. Sufficiency is
	// measured against the heaviest arc drawn from each place — a hardcoded
	// floor would say nothing about a model whose recipes had changed.
	for _, p := range []string{"pantry/coffee_beans", "pantry/milk", "pantry/cups"} {
		if want := 4 * heaviestDraw(t, p); lean.Final[p] < want {
			t.Fatalf("the lean pantry ended with %.0f %s against a heaviest recipe draw of %.0f — "+
				"size it up, this gate needs the stock to never bind", lean.Final[p], p, want/4)
		}
		// Depletions are read per place, not as a list: the run reports
		// staff/available hitting zero too, and that is both baristas being busy
		// for a moment rather than the shop running out of anything.
		for _, d := range lean.Depleted {
			if d.Place == p {
				t.Fatalf("the lean pantry ran out of %s at %.2fh; nothing can be concluded from a stopped shop",
					p, d.At)
			}
		}
	}

	for _, id := range sortedIDs(full.Metrics.Throughput) {
		if got, want := lean.Metrics.Throughput[id], full.Metrics.Throughput[id]; got != want {
			t.Errorf("%s fired %.2f times with a lean pantry and %.2f with a full one — stock is setting the pace",
				id, got, want)
		}
	}
	t.Logf("full pantry served %.1f, lean pantry served %.1f (%d realizations, identical firings)",
		full.Final["counter/orders_complete"], lean.Final["counter/orders_complete"], gateReals)

	// The second assertion, and the one with teeth: how many customers an
	// uncongested shop loses is decided by the rates it declares and by nothing
	// else. Read at eight baristas — 31% utilization and a mean queue of 0.05,
	// so a free pair of hands is all but certain and what is left is the pickup
	// interval racing the customer's patience. That is 12/h against 720/h, four
	// or five arrivals in eight hours, and the band is two-sided on purpose.
	//
	// Below it is mass action: multiply the start side by C(beans,15) *
	// C(milk,50) * cups and a queued order is picked up before anyone can lose
	// patience, so nobody ever walks out. The old engine reported exactly zero
	// walkouts from four baristas up. A floor of zero is not a well-run shop,
	// it is a shop where a queue drains faster than any rate in the model.
	//
	// Above it is the pantry binding, which is what the shipped restock rates
	// used to do — 44 walkouts at eight baristas against the 4 the patience
	// accounts for, ten times the floor, with idle baristas and no diagnostic.
	res := runGate(t, h, staffing(8, nil))
	floor := d.abandonFloor()
	walked := res.Final["counter/walked_out"]
	t.Logf("eight baristas lost %.1f of %.0f arrivals; the declared patience alone accounts for %.1f",
		walked, d.arrivals*gateHours, floor)
	switch {
	case walked < floor/2:
		t.Errorf("eight baristas lost %.1f customers against the %.1f the declared 12/h patience accounts for — "+
			"something is starting queued orders faster than any rate this model declares", walked, floor)
	case walked > 2.5*floor:
		t.Errorf("eight baristas at 31%% utilization lost %.1f customers against the %.1f their patience "+
			"accounts for — with the pool this idle, something else is holding orders in the queue; "+
			"read Contended", walked, floor)
	}

	// Logged rather than asserted, and the distinction is the point. The split
	// by which a queued order leaves — started against abandoned — is 720 to 12,
	// or 0.984. Mass action reads 1.000. Those are a point and a half apart and
	// the run-to-run spread over four seeds was a point, so a band that admitted
	// the honest number would admit the broken one too. The walkout floor above
	// is the same claim measured where it is worth several standard errors
	// instead of a fraction of one.
	for _, drink := range gateDrinks {
		started := res.Metrics.Throughput[drink+"_started"]
		abandoned := res.Metrics.Throughput["counter/abandon_"+drink]
		if started+abandoned == 0 {
			t.Fatalf("no %s order ever left the queue", drink)
		}
		t.Logf("%-11s left the queue started %.3f of the time (declared %.0f/h against %.0f/h patience = %.3f)",
			drink, started/(started+abandoned), d.start[drink], d.abandon[drink],
			d.start[drink]/(d.start[drink]+d.abandon[drink]))
	}
}

// GATE 6 — the shop is limited by its baristas, and the run says which.
//
// This gate exists because its absence was load-bearing. Every headcount sweep
// above used to override the three restock rates to 20, on the reasoning that a
// staffing gate should not be measuring the pantry. The reasoning was right and
// the fix was wrong: the shipped café really was milk-bound — 990 units an hour
// of demand against deliveries worth 1000, with a capacity that blocked the
// delivery whenever stock was above half — so the gates certified a shop that
// was never shipped, and every assertion in them was blind to the constraint
// that actually decided the answer. modelDemand never read a restock rate, so
// pantry/restock_milk could have been set to 0.5 and all five gates would still
// have been green.
//
// Two claims, then. The shipped pantry is not the constraint at any headcount
// an operator would consider — asserted against the model as shipped, with
// nothing overridden. And when a resource does bind, the result names it:
// Contended is the diagnostic that was missing when a planner read "eight idle
// baristas, half the customers lost" and had no way to reach "buy more milk".
func TestGateTheShopIsStaffBound(t *testing.T) {
	h := testApp(t)

	pantry := []string{"pantry/coffee_beans", "pantry/milk", "pantry/cups"}
	for n := 1; n <= 8; n++ {
		res := runGate(t, h, staffing(n, nil))
		held := map[string]sim.Contention{}
		for _, c := range res.Contended {
			held[c.Place] = c
		}
		for _, p := range pantry {
			// 2% of the horizon is a delivery arriving a little late, not a
			// shop that is short of anything. The shipped café measured 29% on
			// milk at four baristas and 40% at eight.
			if got := held[p].Fraction; got > 0.02 {
				t.Errorf("n=%d: %s was the only thing standing between a waiting order and a barista for "+
					"%.0f%% of the day — this sweep is measuring the pantry, not the staffing",
					n, p, got*100)
			}
		}
		staff := held["staff/available"]
		t.Logf("n=%d: staff short %.0f%% of the day, blocking %d transitions; pantry milk %.1f%% beans %.1f%%",
			n, staff.Fraction*100, len(staff.Blocking),
			held["pantry/milk"].Fraction*100, held["pantry/coffee_beans"].Fraction*100)

		// And below the offered load the pool had better be the thing that is
		// short, or "hire" is not the answer to anything.
		if n <= 2 && staff.Fraction < 0.25 {
			t.Errorf("n=%d is under an offered load of 2.55 erlangs but the pool was short for only %.0f%% "+
				"of the day; something else is deciding this shop's throughput", n, staff.Fraction*100)
		}
	}
}

// heaviestDraw is the largest weight any transition consumes from place p.
func heaviestDraw(t *testing.T, p string) float64 {
	t.Helper()
	m := cafe.FlatModel()
	var heaviest float64
	for _, tr := range m.Transitions {
		for _, in := range m.Inputs(tr.ID) {
			if in.Place == p && float64(in.Weight) > heaviest {
				heaviest = float64(in.Weight)
			}
		}
	}
	if heaviest == 0 {
		t.Fatalf("nothing draws from %s", p)
	}
	return heaviest
}

func sortedIDs(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Deterministic failure order; a map range would report the same defect in a
	// different order every run.
	sort.Strings(out)
	return out
}
