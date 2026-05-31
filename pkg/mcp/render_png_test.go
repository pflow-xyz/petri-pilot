package mcp

import (
	"image/png"
	"os"
	"strings"
	"testing"

	goflowmetamodel "github.com/pflow-xyz/go-pflow/metamodel"
)

func coffeeShopModel() *goflowmetamodel.Model {
	return &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "order_pending", Initial: 2, X: 80, Y: 80},
			{ID: "barista_idle", Initial: 1, X: 80, Y: 220},
			{ID: "brewing", X: 260, Y: 150},
			{ID: "ready", X: 440, Y: 150},
			{ID: "delivered", X: 620, Y: 150},
		},
		Transitions: []goflowmetamodel.Transition{
			{ID: "start_brew", X: 170, Y: 150},
			{ID: "finish_brew", X: 350, Y: 150},
			{ID: "deliver", X: 530, Y: 150},
		},
		Arcs: []goflowmetamodel.Arc{
			{From: "order_pending", To: "start_brew", Weight: 1},
			{From: "barista_idle", To: "start_brew", Weight: 1},
			{From: "start_brew", To: "brewing", Weight: 1},
			{From: "brewing", To: "finish_brew", Weight: 1},
			{From: "finish_brew", To: "ready", Weight: 1},
			{From: "finish_brew", To: "barista_idle", Weight: 1},
			{From: "ready", To: "deliver", Weight: 1},
			{From: "deliver", To: "delivered", Weight: 1},
		},
	}
}

func TestRenderPNG_CoffeeShop(t *testing.T) {
	pngBytes, err := renderPNG(coffeeShopModel())
	if err != nil {
		t.Fatalf("renderPNG: %v", err)
	}
	if len(pngBytes) < 200 {
		t.Fatalf("PNG too small: %d bytes", len(pngBytes))
	}
	// Confirm it decodes as a real PNG with sensible dimensions.
	img, err := png.Decode(strings.NewReader(string(pngBytes)))
	if err != nil {
		t.Fatalf("PNG decode: %v", err)
	}
	b := img.Bounds()
	if b.Dx() < 600 || b.Dy() < 200 {
		t.Fatalf("unexpected dimensions: %dx%d", b.Dx(), b.Dy())
	}
	// Side-effect: write to /tmp so a human can eyeball it.
	if path := os.Getenv("RENDER_PNG_OUT"); path != "" {
		if err := os.WriteFile(path, pngBytes, 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote %d bytes to %s", len(pngBytes), path)
	}
}

func TestRenderPNG_AutoLayout(t *testing.T) {
	// Model with no positions — exercises the auto-layout path.
	m := &goflowmetamodel.Model{
		Places: []goflowmetamodel.Place{
			{ID: "a", Initial: 1},
			{ID: "b"},
		},
		Transitions: []goflowmetamodel.Transition{
			{ID: "t"},
		},
		Arcs: []goflowmetamodel.Arc{
			{From: "a", To: "t"},
			{From: "t", To: "b"},
		},
	}
	pngBytes, err := renderPNG(m)
	if err != nil {
		t.Fatalf("renderPNG: %v", err)
	}
	if _, err := png.Decode(strings.NewReader(string(pngBytes))); err != nil {
		t.Fatalf("PNG decode: %v", err)
	}
}
