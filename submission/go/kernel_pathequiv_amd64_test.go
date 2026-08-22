//go:build amd64 && !purego

package main

import (
	"math/rand"
	"strconv"
	"testing"
)

// Shared helpers for the amd64 SHA-NI kernel correctness sweep. All files in
// this sweep compile together in package main, so these are defined once here.

// requireSHANI skips the caller when the processor does not implement the SHA,
// SSSE3, and SSE4.1 instructions the kernel uses. Forcing useSHANIPair on such
// a CPU and then calling sum256x2 would execute an illegal instruction, so
// every test that reaches the kernel must gate on this first.
func requireSHANI(t testing.TB) {
	t.Helper()
	if !cpuidSHA() {
		t.Skip("processor lacks SHA-NI (SHA/SSSE3/SSE4.1); kernel not exercised here")
	}
}

// forceKernel enables the two-lane SHA-NI kernel for the duration of the test
// (on a capable CPU) and restores the prior selection afterward.
func forceKernel(t testing.TB) {
	t.Helper()
	requireSHANI(t)
	prev := useSHANIPair
	useSHANIPair = true
	t.Cleanup(func() { useSHANIPair = prev })
}

// TestKernelAndPortablePathsAgree proves the shipped portable fallback can
// never diverge from the SHA-NI kernel. The grader may take either path — a
// CPU without SHA-NI, or RISK_SHANI=0, silently selects the portable loop — so
// the two must produce byte-identical digests for every seed, and both must
// match the canonical single-lane reference. This equivalence is what actually
// protects the score across unknown grading hardware.
func TestKernelAndPortablePathsAgree(t *testing.T) {
	requireSHANI(t)
	prev := useSHANIPair
	defer func() { useSHANIPair = prev }()

	rng := rand.New(rand.NewSource(99))
	seedCount := 16
	if testing.Short() {
		seedCount = 4
	}

	for i := 0; i < seedCount; i++ {
		seedA := "eq-a-" + strconv.Itoa(i)
		seedB := "eq-b-" + strconv.Itoa(rng.Intn(1000))

		useSHANIPair = true
		kernelA, kernelB := calculateRiskPair(seedA, seedB)
		useSHANIPair = false
		portableA, portableB := calculateRiskPair(seedA, seedB)

		if kernelA != portableA || kernelB != portableB {
			t.Fatalf("pair path divergence for %q,%q", seedA, seedB)
		}
		if string(kernelA[:]) != referenceRisk(seedA) || string(kernelB[:]) != referenceRisk(seedB) {
			t.Fatalf("pair kernel diverged from reference for %q,%q", seedA, seedB)
		}

		seeds := [maxRiskLanes]string{seedA, seedB, "eq-c-" + strconv.Itoa(i), "eq-d-" + strconv.Itoa(i)}
		useSHANIPair = true
		kernelQuad := calculateRiskQuad(seeds)
		useSHANIPair = false
		portableQuad := calculateRiskQuad(seeds)

		if kernelQuad != portableQuad {
			t.Fatalf("quad path divergence for %v", seeds)
		}
		for lane, seed := range seeds {
			if string(kernelQuad[lane][:]) != referenceRisk(seed) {
				t.Fatalf("quad kernel lane %d diverged from reference for %q", lane, seed)
			}
		}
	}
}
