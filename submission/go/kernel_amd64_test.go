package main

import (
	"crypto/sha256"
	"math/rand"
	"testing"
)

func TestSum256x2MatchesStandardLibrary(t *testing.T) {
	if !useSHANIPair {
		t.Skip("two-lane SHA-NI kernel not selected on this processor")
	}
	random := rand.New(rand.NewSource(7))
	for round := 0; round < 10000; round++ {
		var inA, inB [sha256.Size * 2]byte
		random.Read(inA[:])
		random.Read(inB[:])
		var outA, outB [sha256.Size]byte
		sum256x2(&inA, &inB, &outA, &outB)
		if outA != sha256.Sum256(inA[:]) || outB != sha256.Sum256(inB[:]) {
			t.Fatalf("round %d: two-lane digest mismatch", round)
		}
	}
}

func TestPairKernelPathsAgree(t *testing.T) {
	if !useSHANIPair {
		t.Skip("two-lane SHA-NI kernel not selected on this processor")
	}
	a, b := calculateRiskPair("kernel-a", "kernel-b")
	if string(a[:]) != referenceRisk("kernel-a") || string(b[:]) != referenceRisk("kernel-b") {
		t.Fatal("two-lane kernel chain diverged from reference")
	}
}

var benchmarkDigestSink [sha256.Size]byte

func BenchmarkSum256x2(b *testing.B) {
	if !useSHANIPair {
		b.Skip("two-lane SHA-NI kernel not selected on this processor")
	}
	var inA, inB [sha256.Size * 2]byte
	var outA, outB [sha256.Size]byte
	for b.Loop() {
		sum256x2(&inA, &inB, &outA, &outB)
		inA = [sha256.Size * 2]byte(append(outA[:], outB[:]...))
	}
	benchmarkDigestSink = outB
}

func BenchmarkSum256Standard2x(b *testing.B) {
	var inA, inB [sha256.Size * 2]byte
	var outA, outB [sha256.Size]byte
	for b.Loop() {
		outA = sha256.Sum256(inA[:])
		outB = sha256.Sum256(inB[:])
		inA = [sha256.Size * 2]byte(append(outA[:], outB[:]...))
	}
	benchmarkDigestSink = outB
}
