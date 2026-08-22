//go:build amd64 && !purego

package main

import (
	"crypto/sha256"
	"math/rand"
	"strconv"
	"testing"
)

// TestKernelLaneIndependence checks that the two lanes of sum256x2 stay fully
// separate: each lane's digest must equal a standalone SHA-256 of its own input
// and must not change when the other lane's input changes. Cross-lane leakage
// is the most direct way an interleaved two-lane kernel can be subtly wrong
// while still "looking hashed". Single-call, so it runs a large sample cheaply.
func TestKernelLaneIndependence(t *testing.T) {
	forceKernel(t)

	rng := rand.New(rand.NewSource(20260822))
	rounds := 100_000
	if testing.Short() {
		rounds = 5_000
	}

	var a, b, c [sha256.Size * 2]byte
	for i := 0; i < rounds; i++ {
		rng.Read(a[:])
		rng.Read(b[:])
		rng.Read(c[:])

		var abA, abB, acA, acB [sha256.Size]byte
		sum256x2(&a, &b, &abA, &abB) // lane A paired with b
		sum256x2(&a, &c, &acA, &acB) // same lane A paired with c

		if abA != sha256.Sum256(a[:]) {
			t.Fatalf("round %d: lane A digest wrong", i)
		}
		if abA != acA {
			t.Fatalf("round %d: lane A result depends on the other lane's input", i)
		}
		if abB != sha256.Sum256(b[:]) || acB != sha256.Sum256(c[:]) {
			t.Fatalf("round %d: lane B digest wrong", i)
		}
	}

	// Identical inputs in both lanes must both yield the same correct digest.
	var x [sha256.Size * 2]byte
	rng.Read(x[:])
	var x0, x1 [sha256.Size]byte
	sum256x2(&x, &x, &x0, &x1)
	if x0 != sha256.Sum256(x[:]) || x1 != sha256.Sum256(x[:]) {
		t.Fatal("identical-lane inputs produced a wrong digest")
	}
}

// TestChainPositionIndependence checks that a seed's full 50,000-round result
// is identical no matter which lane or batch position it occupies and no matter
// what its neighbors are. Interleaved batching is exactly where a position- or
// neighbor-dependent bug would hide, and this ties the kernel-driven pair and
// quad paths back to the canonical single-lane reference for every position.
func TestChainPositionIndependence(t *testing.T) {
	forceKernel(t)

	rng := rand.New(rand.NewSource(7))
	seedCount := 12
	if testing.Short() {
		seedCount = 3
	}

	randSeed := func() string { return "nb-" + strconv.Itoa(rng.Intn(1_000_000)) }

	for i := 0; i < seedCount; i++ {
		seed := "pos-" + strconv.Itoa(i)
		want := referenceRisk(seed)

		// Pair, seed in each of the two lanes with a random neighbor.
		if got, _ := calculateRiskPair(seed, randSeed()); string(got[:]) != want {
			t.Fatalf("seed %q wrong in pair lane 0", seed)
		}
		if _, got := calculateRiskPair(randSeed(), seed); string(got[:]) != want {
			t.Fatalf("seed %q wrong in pair lane 1", seed)
		}

		// Quad, seed in each of the four positions with random neighbors.
		for pos := 0; pos < maxRiskLanes; pos++ {
			seeds := [maxRiskLanes]string{randSeed(), randSeed(), randSeed(), randSeed()}
			seeds[pos] = seed
			got := calculateRiskQuad(seeds)
			if string(got[pos][:]) != want {
				t.Fatalf("seed %q wrong in quad position %d", seed, pos)
			}
		}
	}
}
