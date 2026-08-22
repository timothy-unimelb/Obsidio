//go:build amd64 && !purego

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"testing"
)

// sum256x2 is a test shim over one kernel round: it hashes two arbitrary
// 64-byte inputs and decodes the kernel's hex output back into raw digests so
// the fuzz, lane-independence, and stress tests can compare with crypto/sha256.
func sum256x2(inA, inB *[64]byte, outA, outB *[32]byte) {
	bufA, bufB := *inA, *inB
	riskChain2x(&bufA, &bufB, 1)
	if _, err := hex.Decode(outA[:], bufA[:]); err != nil {
		panic(err)
	}
	if _, err := hex.Decode(outB[:], bufB[:]); err != nil {
		panic(err)
	}
}

// referenceRounds applies the hex-feedback round with the standard library.
func referenceRounds(buffer *[64]byte, rounds int) {
	for range rounds {
		digest := sha256.Sum256(buffer[:])
		hex.Encode(buffer[:], digest[:])
	}
}

func randomHexBuffer(random *rand.Rand) (buffer [64]byte) {
	var digest [32]byte
	random.Read(digest[:])
	hex.Encode(buffer[:], digest[:])
	return buffer
}

func TestRiskChainKernelsMatchStandardLibrary(t *testing.T) {
	if !useSHANIPair {
		t.Skip("SHA-NI chain kernel not selected on this processor")
	}
	random := rand.New(rand.NewSource(7))
	for round := 0; round < 2000; round++ {
		rounds := 1 + random.Intn(5)
		var bufs, want [4][64]byte
		for index := range bufs {
			bufs[index] = randomHexBuffer(random)
			want[index] = bufs[index]
			referenceRounds(&want[index], rounds)
		}
		pair := bufs
		riskChain2x(&pair[0], &pair[1], rounds)
		if pair[0] != want[0] || pair[1] != want[1] {
			t.Fatalf("round %d: two-lane chain mismatch", round)
		}
		quad := bufs
		riskChain4x(&quad[0], &quad[1], &quad[2], &quad[3], rounds)
		if quad != want {
			t.Fatalf("round %d: four-lane chain mismatch", round)
		}
	}
}

func TestKernelZeroRoundsIsNoop(t *testing.T) {
	if !useSHANIPair {
		t.Skip("SHA-NI chain kernel not selected on this processor")
	}
	random := rand.New(rand.NewSource(9))
	a, b := randomHexBuffer(random), randomHexBuffer(random)
	wantA, wantB := a, b
	riskChain2x(&a, &b, 0)
	if a != wantA || b != wantB {
		t.Fatal("zero rounds modified the buffers")
	}
}

func TestPairKernelPathsAgree(t *testing.T) {
	if !useSHANIPair {
		t.Skip("SHA-NI chain kernel not selected on this processor")
	}
	a, b := calculateRiskPair("kernel-a", "kernel-b")
	if string(a[:]) != referenceRisk("kernel-a") || string(b[:]) != referenceRisk("kernel-b") {
		t.Fatal("two-lane kernel chain diverged from reference")
	}
}

func BenchmarkRiskChain2x(b *testing.B) {
	if !useSHANIPair {
		b.Skip("SHA-NI chain kernel not selected on this processor")
	}
	random := rand.New(rand.NewSource(1))
	bufA, bufB := randomHexBuffer(random), randomHexBuffer(random)
	for b.Loop() {
		riskChain2x(&bufA, &bufB, 1000)
	}
}
