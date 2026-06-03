package mcp

import (
	"fmt"
	"testing"

	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

// Synthetic large-model factory. Builds a chain of N places connected by N-1
// transitions; gives a place at index i an explicit (x, y) only when
// withPositions is true, otherwise leaves them blank to exercise the
// auto-layout.
func chainModel(n int, withPositions bool) *goflowmetamodel.Model {
	model := &goflowmetamodel.Model{}
	for i := 0; i < n; i++ {
		p := goflowmetamodel.Place{ID: fmt.Sprintf("p%03d", i)}
		if withPositions {
			p.X = 80 + (i%10)*120
			p.Y = 80 + (i/10)*200
		}
		model.Places = append(model.Places, p)
	}
	for i := 0; i < n-1; i++ {
		tid := fmt.Sprintf("t%03d", i)
		tr := goflowmetamodel.Transition{ID: tid}
		if withPositions {
			tr.X = 140 + (i%10)*120
			tr.Y = 180 + (i/10)*200
		}
		model.Transitions = append(model.Transitions, tr)
		model.Arcs = append(model.Arcs,
			goflowmetamodel.Arc{From: fmt.Sprintf("p%03d", i), To: tid},
			goflowmetamodel.Arc{From: tid, To: fmt.Sprintf("p%03d", i+1)},
		)
	}
	return model
}

func BenchmarkRenderPNG_Positioned(b *testing.B) {
	for _, n := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := chainModel(n, true)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := renderPNG(m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRenderPNG_AutoLayout(b *testing.B) {
	// Auto-layout is O(N²) per iteration × 80 iterations, so the 500 case
	// will be the dominant cost. Worth measuring honestly so callers know
	// what they're paying.
	for _, n := range []int{10, 50, 100, 500} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := chainModel(n, false)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := renderPNG(m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkHeatmap(b *testing.B) {
	for _, n := range []int{9, 36, 100, 400} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			m := chainModel(n, true)
			opts := &HeatmapOpts{Labels: true}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := renderHeatmapPNG(m, opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
