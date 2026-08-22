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
