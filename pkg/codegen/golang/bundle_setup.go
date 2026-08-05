package golang

import (
	"sort"

	"github.com/pflow-xyz/go-pflow/metamodel"
)

// SetupStep is one entity-local transition the generated test fires before a
// command, to reach a marking where the command is enabled.
type SetupStep struct {
	SubnetID    string
	PackageName string
	ConstName   string
	AggVar      string // the participant variable this entity's id comes from
}

// enablingSequence finds a short run of entity-local transitions that leaves
// target enabled in the flattened net.
//
// The generated bundle test used to fire its command straight from the initial
// marking. That happens to work when every member starts with a token in place
// — warehouse's order begins in `draft`, shop's likewise — and it fails on any
// model whose work has to arrive first. The café's counter starts with
// `orders_pending` empty, because an order is something a customer places, so
// `make_espresso` cannot possibly fire until `order_espresso` has. The test was
// asserting a property of two example models rather than of composition.
//
// Breadth-first over markings, so the sequence found is the shortest one, and
// bounded so a large or unbounded net degrades to "no setup found" rather than
// hanging the generator. Only non-command transitions are candidates: a command
// is exactly what the entity API refuses, so the test could not fire one as
// setup even if it wanted to.
func enablingSequence(flat *metamodel.Model, target string, isCommand map[string]bool,
	entityOf func(flatTransition string) (subnetID, pkg, constName, aggVar string, ok bool)) []SetupStep {

	const (
		maxStates = 20000
		maxDepth  = 12
	)

	start := markingOf(flat)
	if enabled(flat, target, start) {
		return nil // already enabled: warehouse and shop take this path
	}

	type node struct {
		marking map[string]int
		path    []string
	}
	seen := map[string]bool{markingKey(flat, start): true}
	queue := []node{{marking: start}}

	// Deterministic candidate order, so the same model always yields the same
	// setup and the freeze tests stay meaningful.
	candidates := make([]string, 0, len(flat.Transitions))
	for _, t := range flat.Transitions {
		if !isCommand[t.ID] {
			candidates = append(candidates, t.ID)
		}
	}
	sort.Strings(candidates)

	for len(queue) > 0 && len(seen) < maxStates {
		cur := queue[0]
		queue = queue[1:]
		if len(cur.path) >= maxDepth {
			continue
		}

		for _, id := range candidates {
			if !enabled(flat, id, cur.marking) {
				continue
			}
			next := fire(flat, id, cur.marking)
			if enabled(flat, target, next) {
				return toSteps(append(append([]string{}, cur.path...), id), entityOf)
			}
			key := markingKey(flat, next)
			if seen[key] {
				continue
			}
			seen[key] = true
			queue = append(queue, node{
				marking: next,
				path:    append(append([]string{}, cur.path...), id),
			})
		}
	}
	return nil
}

// toSteps resolves flat transition IDs back to the entity packages that own
// them, dropping any that no single entity can fire.
func toSteps(path []string, entityOf func(string) (string, string, string, string, bool)) []SetupStep {
	out := make([]SetupStep, 0, len(path))
	for _, id := range path {
		subnet, pkg, constName, aggVar, ok := entityOf(id)
		if !ok {
			return nil // unresolvable: emit no setup rather than a broken test
		}
		out = append(out, SetupStep{SubnetID: subnet, PackageName: pkg, ConstName: constName, AggVar: aggVar})
	}
	return out
}

func markingOf(m *metamodel.Model) map[string]int {
	out := make(map[string]int, len(m.Places))
	for _, p := range m.Places {
		if p.IsToken() {
			out[p.ID] = p.Initial
		}
	}
	return out
}

func markingKey(m *metamodel.Model, marking map[string]int) string {
	var b []byte
	for _, p := range m.Places {
		if !p.IsToken() {
			continue
		}
		b = append(b, p.ID...)
		b = append(b, ':')
		b = appendInt(b, marking[p.ID])
		b = append(b, ';')
	}
	return string(b)
}

func appendInt(b []byte, n int) []byte {
	if n == 0 {
		return append(b, '0')
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return append(b, digits[i:]...)
}

// enabled applies the firing rule: enough tokens on every consuming arc, the
// threshold met on every read arc, and no inhibitor satisfied.
func enabled(m *metamodel.Model, transition string, marking map[string]int) bool {
	for _, a := range m.Arcs {
		if a.To != transition {
			continue
		}
		if m.PlaceByID(a.From) == nil {
			continue
		}
		w := a.Weight
		if w == 0 {
			w = 1
		}
		switch {
		case a.IsInhibitor():
			if marking[a.From] >= w {
				return false
			}
		default:
			// A read arc gates identically to a consuming one; only the effect
			// of firing differs, which is handled in fire.
			if marking[a.From] < w {
				return false
			}
		}
	}
	return true
}

// fire returns the marking after transition fires. Read and inhibitor arcs
// move nothing — treating a read arc as consuming here would make the search
// explore states the net cannot reach.
func fire(m *metamodel.Model, transition string, marking map[string]int) map[string]int {
	next := make(map[string]int, len(marking))
	for k, v := range marking {
		next[k] = v
	}
	for _, a := range m.Arcs {
		w := a.Weight
		if w == 0 {
			w = 1
		}
		if a.IsReadOnly() {
			continue
		}
		if a.To == transition && m.PlaceByID(a.From) != nil {
			next[a.From] -= w
		}
		if a.From == transition && m.PlaceByID(a.To) != nil {
			next[a.To] += w
		}
	}
	return next
}
