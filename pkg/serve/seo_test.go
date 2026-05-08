package serve

import (
	"os"
	"strings"
	"testing"
)

func TestInjectSEO_CuratedRoute(t *testing.T) {
	in := []byte("<html><head><title>Tic-Tac-Toe Simulator</title></head><body></body></html>")
	out := injectSEO(in, "tic-tac-toe")

	wants := []string{
		`<meta name="description" content="Nine board cells as places`,
		`<link rel="canonical" href="https://pilot.pflow.xyz/tic-tac-toe/">`,
		`<meta property="og:title" content="Tic-Tac-Toe Petri Net Simulator">`,
		`<meta property="og:image" content="https://pilot.pflow.xyz/logo-square.png">`,
		`<meta name="twitter:card" content="summary">`,
		`<title>Tic-Tac-Toe Petri Net Simulator</title>`,
	}
	for _, w := range wants {
		if !strings.Contains(string(out), w) {
			t.Errorf("expected output to contain %q, got:\n%s", w, out)
		}
	}
}

func TestInjectSEO_UnknownRouteIsNoOp(t *testing.T) {
	in := []byte("<html><head><title>Anything</title></head></html>")
	out := injectSEO(in, "not-a-real-app")
	if string(out) != string(in) {
		t.Errorf("expected unknown route to be left unchanged, got:\n%s", out)
	}
}

func TestInjectSEO_EmptyNameIsNoOp(t *testing.T) {
	in := []byte("<html><head><title>Anything</title></head></html>")
	out := injectSEO(in, "")
	if string(out) != string(in) {
		t.Errorf("expected empty name to be left unchanged")
	}
}

func TestInjectSEO_PreservesExistingDescription(t *testing.T) {
	// Pages that already declare their own description (like the landing page)
	// must not be touched.
	in := []byte(`<html><head><title>X</title><meta name="description" content="hand-curated"></head></html>`)
	out := injectSEO(in, "tic-tac-toe")
	if string(out) != string(in) {
		t.Errorf("expected existing description to short-circuit injection")
	}
}

func TestInjectSEO_EscapesAttributes(t *testing.T) {
	// Sanity: the curated descriptions don't contain quotes today, but make sure
	// the escaper would handle them if a future entry did.
	if got := htmlAttrEscape(`he said "hi" & <stuff>`); got != `he said &quot;hi&quot; &amp; &lt;stuff&gt;` {
		t.Errorf("htmlAttrEscape wrong: %q", got)
	}
}

func TestFrontendNameFromPath(t *testing.T) {
	cases := map[string]string{
		"frontends/tic-tac-toe":         "tic-tac-toe",
		"frontends/coffeeshop/":         "coffeeshop",
		"landing":                       "",
		"generated/erc20token/frontend": "",
	}
	for in, want := range cases {
		if got := frontendNameFromPath(in); got != want {
			t.Errorf("frontendNameFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenerateSitemap(t *testing.T) {
	xml := string(generateSitemap())

	wants := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`,
		`<loc>https://pilot.pflow.xyz/</loc>`,
		`<loc>https://pilot.pflow.xyz/pflow</loc>`,
		`<loc>https://pilot.pflow.xyz/tic-tac-toe/</loc>`,
		`<loc>https://pilot.pflow.xyz/code-to-flow/</loc>`,
		`<priority>1.0</priority>`,
		`<priority>0.8</priority>`,
		`<priority>0.7</priority>`,
		`</urlset>`,
	}
	for _, w := range wants {
		if !strings.Contains(xml, w) {
			t.Errorf("expected sitemap to contain %q", w)
		}
	}

	// Every curated frontend must appear in the sitemap — that's the whole
	// point of generating it from frontendMeta.
	for name := range frontendMeta {
		want := "<loc>https://pilot.pflow.xyz/" + name + "/</loc>"
		if !strings.Contains(xml, want) {
			t.Errorf("sitemap missing entry for %q", name)
		}
	}
}

func TestEveryCuratedRouteHasFrontendDir(t *testing.T) {
	// Catch typos: every key in frontendMeta must correspond to a real directory
	// under frontends/. Skip if frontends/ isn't reachable from the test cwd.
	if _, err := os.Stat("../../frontends"); err != nil {
		t.Skip("frontends/ not reachable from test cwd; skipping")
	}
	for name := range frontendMeta {
		if _, err := os.Stat("../../frontends/" + name); err != nil {
			t.Errorf("frontendMeta key %q has no matching frontends/%s directory", name, name)
		}
	}
}
