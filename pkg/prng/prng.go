// Package prng is the portable seeded random stream shared by every engine
// that claims parity (ROADMAP Phase 2). "Same seed, same trajectory" across
// Go and JS is only true if both sides draw identical numbers, so the
// algorithm is pinned here and reimplemented — never approximated — anywhere
// else.
//
// The generator is xoshiro128++ (Blackman & Vigna), chosen over PCG32
// because it needs nothing beyond 32-bit unsigned arithmetic: every
// operation (xor, shift, rotate, add mod 2^32) is exact in JavaScript with
// >>> and Math.imul-free addition, so a JS implementation can match this one
// bit for bit without BigInt in the hot path. Seeding expands a 64-bit seed
// through splitmix64 (the reference recommendation), which JS needs BigInt
// for — once, at construction, off the hot path.
//
// Float64 builds the canonical 53-bit double from two 32-bit draws:
// (top 27 bits << 26 | top 26 bits) * 2^-53. Both halves are exact in a
// float64, so Go and JS produce identical doubles, and every downstream
// difference is arithmetic, not randomness.
package prng

// Rand is a deterministic xoshiro128++ stream.
type Rand struct {
	s0, s1, s2, s3 uint32
}

// splitmix64 is the reference seed expander. It cannot return the all-zero
// xoshiro state for any input, but the guard in New keeps that invariant
// explicit rather than inherited.
func splitmix64(x uint64) (uint64, uint64) {
	x += 0x9E3779B97F4A7C15
	z := x
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return x, z ^ (z >> 31)
}

// New seeds a stream from a 64-bit seed. Equal seeds give equal streams in
// every conforming implementation; that is the whole contract.
func New(seed int64) *Rand {
	x := uint64(seed)
	x, a := splitmix64(x)
	_, b := splitmix64(x)
	r := &Rand{
		s0: uint32(a), s1: uint32(a >> 32),
		s2: uint32(b), s3: uint32(b >> 32),
	}
	if r.s0|r.s1|r.s2|r.s3 == 0 {
		r.s3 = 1 // unreachable via splitmix64, but the invariant is load-bearing
	}
	return r
}

func rotl(x uint32, k uint) uint32 {
	return (x << k) | (x >> (32 - k))
}

// Uint32 advances the stream one step.
func (r *Rand) Uint32() uint32 {
	result := rotl(r.s0+r.s3, 7) + r.s0
	t := r.s1 << 9
	r.s2 ^= r.s0
	r.s3 ^= r.s1
	r.s1 ^= r.s2
	r.s0 ^= r.s3
	r.s2 ^= t
	r.s3 = rotl(r.s3, 11)
	return result
}

// Float64 returns a double in [0, 1) built from exactly two Uint32 draws.
// The draw order (high word first) is part of the portable contract.
func (r *Rand) Float64() float64 {
	hi := r.Uint32() >> 5 // top 27 bits
	lo := r.Uint32() >> 6 // top 26 bits
	return float64(uint64(hi)<<26|uint64(lo)) / (1 << 53)
}
