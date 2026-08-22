package main

import (
	"crypto/sha256"
	"math/rand"
	"testing"
)

// TestKernelSum64MatchesStdlib: the direct 2-block kernel must be
// bit-identical to crypto/sha256 for every 64-byte input we can throw at it.
// A wrong digest scores zero, so this runs both in CI here and inside the
// grader's docker build (RUN go test gate).
func TestKernelSum64MatchesStdlib(t *testing.T) {
	if !kernelUseSHANI && !kernelUseAVX2 {
		t.Skip("no eligible ISA on this machine; kernel inactive (stdlib fallback ships)")
	}
	var in [64]byte
	var got [32]byte

	// Structured patterns: every byte value in every repeated-fill form.
	for v := 0; v < 256; v++ {
		for j := range in {
			in[j] = byte(v)
		}
		kernelSum64(&in, &got)
		if want := sha256.Sum256(in[:]); got != want {
			t.Fatalf("fill 0x%02x: kernel %x != stdlib %x", v, got, want)
		}
	}
	// 50k random inputs (deterministic seed).
	rnd := rand.New(rand.NewSource(42))
	for i := 0; i < 50000; i++ {
		rnd.Read(in[:])
		kernelSum64(&in, &got)
		if want := sha256.Sum256(in[:]); got != want {
			t.Fatalf("random case %d (in=%x): kernel %x != stdlib %x", i, in, got, want)
		}
	}
	// Chain composition: kernel output fed back through hex, 1000 deep,
	// against the naive reference.
	if got, want := riskChainN("kernel-diff", 1000), naiveRiskChainN("kernel-diff", 1000); got != want {
		t.Fatalf("1000-deep chain: %s != %s", got, want)
	}
}

// TestKernelPairMatchesStdlib: both lanes of the interleaved routine must be
// bit-identical to crypto/sha256, including when lanes carry unequal data.
func TestKernelPairMatchesStdlib(t *testing.T) {
	if !kernelPairOK {
		t.Skip("2-lane pair path not active on this machine")
	}
	rnd := rand.New(rand.NewSource(7))
	var inA, inB [64]byte
	var gotA, gotB [32]byte
	for i := 0; i < 50000; i++ {
		rnd.Read(inA[:])
		rnd.Read(inB[:])
		kernelSum64Pair(&inA, &inB, &gotA, &gotB)
		if want := sha256.Sum256(inA[:]); gotA != want {
			t.Fatalf("case %d lane A: %x != %x", i, gotA, want)
		}
		if want := sha256.Sum256(inB[:]); gotB != want {
			t.Fatalf("case %d lane B: %x != %x", i, gotB, want)
		}
	}
	// Lanes must not leak into each other: same input on both lanes.
	for i := 0; i < 1000; i++ {
		rnd.Read(inA[:])
		inB = inA
		kernelSum64Pair(&inA, &inB, &gotA, &gotB)
		if gotA != gotB || gotA != sha256.Sum256(inA[:]) {
			t.Fatalf("case %d equal-lane mismatch", i)
		}
	}
}

// BenchmarkPairVsSerial measures the interleave ratio on this silicon:
// compare ns/op of Serial2 (two sequential kernelSum64) against Pair.
func BenchmarkKernelSerial2(b *testing.B) {
	if !kernelPairOK {
		b.Skip("pair path inactive")
	}
	var inA, inB [64]byte
	var outA, outB [32]byte
	inA[0], inB[0] = 1, 2
	for i := 0; i < b.N; i++ {
		kernelSum64(&inA, &outA)
		kernelSum64(&inB, &outB)
	}
}

func BenchmarkKernelPair(b *testing.B) {
	if !kernelPairOK {
		b.Skip("pair path inactive")
	}
	var inA, inB [64]byte
	var outA, outB [32]byte
	inA[0], inB[0] = 1, 2
	for i := 0; i < b.N; i++ {
		kernelSum64Pair(&inA, &inB, &outA, &outB)
	}
}

// TestRiskChainPairMatchesSingle: full 50k-iteration paired chains must equal
// the single-lane chains digest-for-digest (this is the shipped combination).
func TestRiskChainPairMatchesSingle(t *testing.T) {
	if riskSumPair == nil {
		t.Skip("pair path inactive on this machine")
	}
	ha, hb := riskChainPair("0.4823905", "none")
	if want := riskChain("0.4823905"); ha != want {
		t.Fatalf("lane A full chain: %s != %s", ha, want)
	}
	if want := riskChain("none"); hb != want {
		t.Fatalf("lane B full chain: %s != %s", hb, want)
	}
}

func riskChainN(seed string, n int) string {
	var buf [64]byte
	sum := sha256.Sum256([]byte(seed))
	hexEncode64(&buf, &sum)
	for i := 1; i < n; i++ {
		riskSum64(&buf, &sum)
		hexEncode64(&buf, &sum)
	}
	return string(buf[:])
}

func naiveRiskChainN(seed string, n int) string {
	h := seed
	for i := 0; i < n; i++ {
		sum := sha256.Sum256([]byte(h))
		h = string(hexAppend(sum[:]))
	}
	return h
}

func hexAppend(b []byte) []byte {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, v := range b {
		out = append(out, digits[v>>4], digits[v&15])
	}
	return out
}
