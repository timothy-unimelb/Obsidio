package main

import (
	"crypto/sha256"
	"math/rand"
	"strings"
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

// TestKernelAVX2BranchMatchesStdlib: force the AVX2 branch of kernelSum64 —
// the path a non-SHA-NI grader box would take — and verify it against
// stdlib. Every SHA-NI machine also has AVX2, so this runs everywhere the
// SHA-NI tests run.
func TestKernelAVX2BranchMatchesStdlib(t *testing.T) {
	if !strings.Contains(cpuinfoFlags(), " avx2 ") {
		t.Skip("no AVX2 on this machine")
	}
	savedS, savedA := kernelUseSHANI, kernelUseAVX2
	kernelUseSHANI, kernelUseAVX2 = false, true
	defer func() { kernelUseSHANI, kernelUseAVX2 = savedS, savedA }()
	rnd := rand.New(rand.NewSource(555))
	var in [64]byte
	var got [32]byte
	for i := 0; i < 50000; i++ {
		rnd.Read(in[:])
		kernelSum64(&in, &got)
		if want := sha256.Sum256(in[:]); got != want {
			t.Fatalf("AVX2 branch case %d: %x != %x", i, got, want)
		}
	}
}

// BenchmarkSingleComposedAVX2: the per-iteration cost a non-SHA-NI x86
// grader would see on the direct-kernel path (vs stdlib's own AVX2 path in
// BenchmarkRiskChain with GODEBUG=cpu.sha=off).
func BenchmarkSingleComposedAVX2(b *testing.B) {
	if !strings.Contains(cpuinfoFlags(), " avx2 ") {
		b.Skip("no AVX2")
	}
	savedS, savedA := kernelUseSHANI, kernelUseAVX2
	kernelUseSHANI, kernelUseAVX2 = false, true
	defer func() { kernelUseSHANI, kernelUseAVX2 = savedS, savedA }()
	var in [64]byte
	var s [32]byte
	in[0] = 1
	for i := 0; i < b.N; i++ {
		kernelSum64(&in, &s)
		hexEncode64(&in, &s)
	}
}

// TestPairHashHexMatchesComposed: the fused iteration must equal
// hash-then-hex composed from already-verified pieces, for arbitrary 64-byte
// inputs (not just hex strings) and for equal-lane inputs.
func TestPairHashHexMatchesComposed(t *testing.T) {
	if riskPairIter == nil {
		t.Skip("fused pair path inactive on this machine")
	}
	rnd := rand.New(rand.NewSource(99))
	var a, b [64]byte
	var sa, sb [32]byte
	var ea, eb [64]byte
	for i := 0; i < 50000; i++ {
		rnd.Read(a[:])
		rnd.Read(b[:])
		ca, cb := a, b
		kernelSum64Pair(&ca, &cb, &sa, &sb)
		hexEncode64(&ea, &sa)
		hexEncode64(&eb, &sb)
		pairHashHex(&a, &b)
		if a != ea {
			t.Fatalf("case %d lane A: fused %s != composed %s", i, a, ea)
		}
		if b != eb {
			t.Fatalf("case %d lane B: fused %s != composed %s", i, b, eb)
		}
	}
	for i := 0; i < 1000; i++ {
		rnd.Read(a[:])
		b = a
		pairHashHex(&a, &b)
		if a != b {
			t.Fatalf("case %d: equal-lane divergence", i)
		}
	}
}

func BenchmarkPairComposed(b *testing.B) {
	if !kernelPairOK {
		b.Skip("pair path inactive")
	}
	var inA, inB [64]byte
	var sa, sb [32]byte
	inA[0], inB[0] = 1, 2
	for i := 0; i < b.N; i++ {
		kernelSum64Pair(&inA, &inB, &sa, &sb)
		hexEncode64(&inA, &sa)
		hexEncode64(&inB, &sb)
	}
}

func BenchmarkPairFused(b *testing.B) {
	if riskPairIter == nil {
		b.Skip("fused path inactive")
	}
	var inA, inB [64]byte
	inA[0], inB[0] = 1, 2
	for i := 0; i < b.N; i++ {
		pairHashHex(&inA, &inB)
	}
}

// TestHashHex1MatchesComposed: single-lane fused iteration vs the composed
// reference for arbitrary 64-byte inputs.
func TestHashHex1MatchesComposed(t *testing.T) {
	if riskIter1 == nil {
		t.Skip("single-lane fused path inactive on this machine")
	}
	rnd := rand.New(rand.NewSource(123))
	var x, e [64]byte
	var s [32]byte
	for i := 0; i < 50000; i++ {
		rnd.Read(x[:])
		c := x
		kernelSum64(&c, &s)
		hexEncode64(&e, &s)
		hashHex1(&x)
		if x != e {
			t.Fatalf("case %d: fused %s != composed %s", i, x, e)
		}
	}
}

func BenchmarkSingleComposed(b *testing.B) {
	if !kernelUseSHANI && !kernelUseAVX2 {
		b.Skip("kernel inactive")
	}
	var in [64]byte
	var s [32]byte
	in[0] = 1
	for i := 0; i < b.N; i++ {
		kernelSum64(&in, &s)
		hexEncode64(&in, &s)
	}
}

func BenchmarkSingleFused(b *testing.B) {
	if riskIter1 == nil {
		b.Skip("single fused inactive")
	}
	var in [64]byte
	in[0] = 1
	for i := 0; i < b.N; i++ {
		hashHex1(&in)
	}
}

// TestPairHashHexNMatchesComposed: the v3 loop-in-asm routine must equal the
// already-verified single-shot fused routine composed n times, for arbitrary
// 64-byte starts and every chunk shape the shipped loop can produce.
func TestPairHashHexNMatchesComposed(t *testing.T) {
	if riskPairIter == nil {
		t.Skip("fused pair path inactive on this machine")
	}
	rnd := rand.New(rand.NewSource(2026))
	var a, b [64]byte
	for _, n := range []int{1, 2, 3, 4, 5, 17, 256, 4096, 4335} {
		cases := 64
		if n >= 256 {
			cases = 8
		}
		for c := 0; c < cases; c++ {
			rnd.Read(a[:])
			rnd.Read(b[:])
			wa, wb := a, b
			for k := 0; k < n; k++ {
				pairHashHex(&wa, &wb)
			}
			ga, gb := a, b
			pairHashHexN(&ga, &gb, n)
			if ga != wa || gb != wb {
				t.Fatalf("n=%d case %d: v3 (%s,%s) != composed (%s,%s)", n, c, ga, gb, wa, wb)
			}
		}
	}
	// Equal-lane inputs must not leak across lanes.
	for c := 0; c < 100; c++ {
		rnd.Read(a[:])
		b = a
		pairHashHexN(&a, &b, 17)
		if a != b {
			t.Fatalf("case %d: equal-lane divergence", c)
		}
	}
	// n=0 must be a no-op that leaves the buffers untouched.
	rnd.Read(a[:])
	rnd.Read(b[:])
	ca, cb := a, b
	pairHashHexN(&a, &b, 0)
	if a != ca || b != cb {
		t.Fatal("n=0 modified the buffers")
	}
}

// TestHashHex1NMatchesComposed: single-lane v3 vs composed hashHex1.
func TestHashHex1NMatchesComposed(t *testing.T) {
	if riskIter1 == nil {
		t.Skip("fused single path inactive on this machine")
	}
	rnd := rand.New(rand.NewSource(2027))
	var x [64]byte
	for _, n := range []int{1, 2, 3, 4, 5, 17, 256, 4096, 4335} {
		cases := 64
		if n >= 256 {
			cases = 8
		}
		for c := 0; c < cases; c++ {
			rnd.Read(x[:])
			w := x
			for k := 0; k < n; k++ {
				hashHex1(&w)
			}
			g := x
			hashHex1N(&g, n)
			if g != w {
				t.Fatalf("n=%d case %d: v3 %s != composed %s", n, c, g, w)
			}
		}
	}
	rnd.Read(x[:])
	c := x
	hashHex1N(&x, 0)
	if x != c {
		t.Fatal("n=0 modified the buffer")
	}
}

// BenchmarkPairFusedN reports per-iteration cost of the v3 loop at the
// shipped chunk size — compare ns/op directly against BenchmarkPairFused.
func BenchmarkPairFusedN(b *testing.B) {
	if riskPairIter == nil {
		b.Skip("fused pair path inactive")
	}
	var inA, inB [64]byte
	inA[0], inB[0] = 1, 2
	const chunk = 4096
	for i := 0; i < b.N; i += chunk {
		n := chunk
		if rem := b.N - i; rem < chunk {
			n = rem
		}
		pairHashHexN(&inA, &inB, n)
	}
}

func BenchmarkSingleFusedN(b *testing.B) {
	if riskIter1 == nil {
		b.Skip("fused single path inactive")
	}
	var in [64]byte
	in[0] = 1
	const chunk = 4096
	for i := 0; i < b.N; i += chunk {
		n := chunk
		if rem := b.N - i; rem < chunk {
			n = rem
		}
		hashHex1N(&in, n)
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
