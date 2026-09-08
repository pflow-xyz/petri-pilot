package prng

import (
	"math"
	"testing"
)

// The vectors below were produced by an INDEPENDENT Python transcription of
// xoshiro128++ + splitmix64 + the 53-bit double construction — two separate
// implementations of the written spec agreeing, not one implementation
// echoing itself. Any conforming implementation (the JS engine included)
// must reproduce them exactly; a change to this file is a change to the
// portable contract and invalidates every parity fixture.
var goldenUint32 = map[int64][4]uint32{
	0:        {1179900579, 1938959192, 3089844957, 3657088315},
	1:        {2146930148, 2585199205, 3670091704, 2556029204},
	42:       {2643743425, 1762251840, 1632151183, 1417845339},
	-1:       {2650238882, 1278636629, 2532285648, 588492107},
	20260809: {3066698231, 3233994224, 3477068307, 1864505539},
}

var goldenFloat64 = map[int64][3]float64{
	0:        {0.2747170139212095, 0.7194105897209213, 0.23642878317643257},
	1:        {0.4998711317920207, 0.8545098125571156, 0.3019295761526998},
	42:       {0.6155444861226248, 0.38001480944542465, 0.7745493794146345},
	-1:       {0.6170568264849915, 0.5895936946348771, 0.31683869669720166},
	20260809: {0.7140213232708942, 0.8095680507579001, 0.8923036601211438},
}

func TestGoldenUint32(t *testing.T) {
	for seed, want := range goldenUint32 {
		r := New(seed)
		for i, w := range want {
			if got := r.Uint32(); got != w {
				t.Errorf("seed %d draw %d: got %d, want %d", seed, i, got, w)
			}
		}
	}
}

func TestGoldenFloat64(t *testing.T) {
	for seed, want := range goldenFloat64 {
		r := New(seed)
		for i, w := range want {
			got := r.Float64()
			// Exact equality is the contract: both halves of the 53-bit
			// construction are exactly representable, so there is no
			// rounding for a tolerance to forgive.
			if got != w {
				t.Errorf("seed %d draw %d: got %v, want %v (diff %g)", seed, i, got, w, got-w)
			}
		}
	}
}

func TestFloat64Range(t *testing.T) {
	r := New(7)
	for i := 0; i < 100000; i++ {
		v := r.Float64()
		if v < 0 || v >= 1 {
			t.Fatalf("draw %d out of [0,1): %v", i, v)
		}
		if math.IsNaN(v) {
			t.Fatalf("draw %d is NaN", i)
		}
	}
}

func TestSeedsDiverge(t *testing.T) {
	a, b := New(1), New(2)
	same := 0
	for i := 0; i < 64; i++ {
		if a.Uint32() == b.Uint32() {
			same++
		}
	}
	if same > 2 {
		t.Errorf("seeds 1 and 2 agree on %d of 64 draws", same)
	}
}

func TestStreamIsDeterministic(t *testing.T) {
	a, b := New(99), New(99)
	for i := 0; i < 1000; i++ {
		if a.Float64() != b.Float64() {
			t.Fatalf("same seed diverged at draw %d", i)
		}
	}
}
