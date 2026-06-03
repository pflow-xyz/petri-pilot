package mcp

import (
	"math"

	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// Force-directed (Fruchterman-Reingold) layout for position-less models.
// Replaces the old horizontal-row fallback, which produced unreadable
// diagrams for anything beyond ~6 nodes.
//
// The algorithm:
//   - Initialize nodes on a circle (deterministic — same model produces the
//     same layout across runs).
//   - Each iteration accumulates displacement from repulsive forces between
//     all node pairs (k^2/d) and attractive forces along arcs (d^2/k).
//   - Apply displacements with a bounded max step that cools linearly to
//     zero over N iterations.
//   - Translate the final result into the positive quadrant with margin.

// computeAutoLayout returns explicit (x, y) positions for every place and
// transition. Used only when a model lacks positions; explicit X/Y values
// in the model take precedence wherever present.
func computeAutoLayout(model *goflowmetamodel.Model) (placePos, transPos map[string][2]int) {
	type node struct {
		id        string
		x, y, dx, dy float64
		isPlace   bool
	}

	nodes := make([]*node, 0, len(model.Places)+len(model.Transitions))
	idIdx := map[string]int{}
	for _, p := range model.Places {
		idIdx[p.ID] = len(nodes)
		nodes = append(nodes, &node{id: p.ID, isPlace: true})
	}
	for _, t := range model.Transitions {
		idIdx[t.ID] = len(nodes)
		nodes = append(nodes, &node{id: t.ID})
	}
	n := len(nodes)
	if n == 0 {
		return map[string][2]int{}, map[string][2]int{}
	}

	// Initial circular layout — deterministic so test output is stable.
	const radius = 200.0
	for i, nd := range nodes {
		theta := 2 * math.Pi * float64(i) / float64(n)
		nd.x = radius * math.Cos(theta)
		nd.y = radius * math.Sin(theta)
	}

	// Optimal edge length k = sqrt(area / n). area chosen to roughly match
	// the historic 120px node spacing for n ~= 6.
	area := math.Max(1, 800.0*600.0)
	k := math.Sqrt(area / float64(n))

	// Build adjacency from arcs. Place→Transition and Transition→Place.
	type edge struct{ a, b int }
	edges := make([]edge, 0, len(model.Arcs))
	for _, arc := range model.Arcs {
		ai, aok := idIdx[arc.From]
		bi, bok := idIdx[arc.To]
		if aok && bok {
			edges = append(edges, edge{ai, bi})
		}
	}

	const iterations = 80
	temperature := radius / 4 // initial max step ≈ 1/4 the initial circle

	for iter := 0; iter < iterations; iter++ {
		// Repulsion: all-pairs O(n²) — fine for n up to a few hundred.
		for i := 0; i < n; i++ {
			nodes[i].dx, nodes[i].dy = 0, 0
		}
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				ddx := nodes[i].x - nodes[j].x
				ddy := nodes[i].y - nodes[j].y
				d := math.Hypot(ddx, ddy)
				if d < 0.001 {
					d = 0.001
					ddx = 0.001
				}
				force := k * k / d
				nodes[i].dx += (ddx / d) * force
				nodes[i].dy += (ddy / d) * force
			}
		}

		// Attraction along edges.
		for _, e := range edges {
			a, b := nodes[e.a], nodes[e.b]
			ddx := a.x - b.x
			ddy := a.y - b.y
			d := math.Hypot(ddx, ddy)
			if d < 0.001 {
				continue
			}
			force := d * d / k
			fx := (ddx / d) * force
			fy := (ddy / d) * force
			a.dx -= fx
			a.dy -= fy
			b.dx += fx
			b.dy += fy
		}

		// Apply displacement, bounded by temperature, then cool.
		for _, nd := range nodes {
			d := math.Hypot(nd.dx, nd.dy)
			if d < 0.001 {
				continue
			}
			step := math.Min(d, temperature)
			nd.x += (nd.dx / d) * step
			nd.y += (nd.dy / d) * step
		}
		temperature *= 1 - 1.0/float64(iterations)
	}

	// Translate into positive coordinates with a 60px margin.
	minX, minY := math.Inf(1), math.Inf(1)
	for _, nd := range nodes {
		if nd.x < minX {
			minX = nd.x
		}
		if nd.y < minY {
			minY = nd.y
		}
	}
	const margin = 60.0
	placePos = make(map[string][2]int, len(model.Places))
	transPos = make(map[string][2]int, len(model.Transitions))
	for _, nd := range nodes {
		x := int(nd.x - minX + margin)
		y := int(nd.y - minY + margin)
		if nd.isPlace {
			placePos[nd.id] = [2]int{x, y}
		} else {
			transPos[nd.id] = [2]int{x, y}
		}
	}
	return placePos, transPos
}

// shouldAutoLayout decides whether to run the force-directed layout. It only
// runs when ALL nodes lack positions — partial positions are honored as
// authoritative hints and the renderer keeps its old behavior.
func shouldAutoLayout(model *goflowmetamodel.Model) bool {
	for _, p := range model.Places {
		if p.X != 0 || p.Y != 0 {
			return false
		}
	}
	for _, t := range model.Transitions {
		if t.X != 0 || t.Y != 0 {
			return false
		}
	}
	return len(model.Places)+len(model.Transitions) > 0
}
