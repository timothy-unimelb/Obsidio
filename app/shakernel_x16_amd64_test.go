package main

import (
	"crypto/sha256"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

func x16Available() bool {
	f := cpuinfoFlags()
	for _, want := range []string{"avx512f", "avx512dq", "avx512bw", "avx512vl"} {
		if !strings.Contains(f, " "+want+" ") {
			return false
		}
	}
	return true
}

// TestX16Sum64MatchesStdlib: every lane of the vendored 16-lane kernel must
// be bit-identical to crypto/sha256, including equal-lane inputs (no
// cross-lane leaks in the transposed state).
func TestX16Sum64MatchesStdlib(t *testing.T) {
	if !x16Available() {
		t.Skip("no AVX-512 on this machine")
	}
	rnd := rand.New(rand.NewSource(1616))
	var in [16][64]byte
	var got [16][32]byte
	for c := 0; c < 5000; c++ {
		for i := range in {
			rnd.Read(in[i][:])
		}
		if c%10 == 9 {
			in[0] = in[15] // equal lanes must not leak
			in[7] = in[8]
		}
		x16Sum64(&in, &got)
		for i := range in {
			if want := sha256.Sum256(in[i][:]); got[i] != want {
				t.Fatalf("case %d lane %d: %x != %x", c, i, got[i], want)
			}
		}
	}
	// Structured fills.
	for v := 0; v < 256; v += 15 {
		for i := range in {
			for j := range in[i] {
				in[i][j] = byte(v + i)
			}
		}
		x16Sum64(&in, &got)
		for i := range in {
			if want := sha256.Sum256(in[i][:]); got[i] != want {
				t.Fatalf("fill %d lane %d mismatch", v, i)
			}
		}
	}
}

// TestX16ChainMatchesReference: full 50k-iteration lockstep chains must equal
// the scalar riskChain digest-for-digest, for full and partial batches.
func TestX16ChainMatchesReference(t *testing.T) {
	if !x16Available() {
		t.Skip("no AVX-512 on this machine")
	}
	if testing.Short() {
		t.Skip("full-chain test")
	}
	seeds := make([]string, 16)
	for i := range seeds {
		seeds[i] = "x16-chain-" + string(rune('a'+i))
	}
	outs := x16ChainRun(seeds)
	for i, s := range seeds {
		if want := riskChain(s); outs[i] != want {
			t.Fatalf("lane %d (%s): %s != %s", i, s, outs[i], want)
		}
	}
	// Partial batch (9 live lanes, dummy tail).
	part := seeds[:9]
	outs = x16ChainRun(part)
	for i, s := range part {
		if want := riskChain(s); outs[i] != want {
			t.Fatalf("partial lane %d: %s != %s", i, s, outs[i])
		}
	}
}

// TestX16RaceHammer: concurrent batches (as two workers would run) must not
// share state through the pool or the transposed buffers.
func TestX16RaceHammer(t *testing.T) {
	if !x16Available() {
		t.Skip("no AVX-512 on this machine")
	}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			seeds := make([]string, 16)
			for i := range seeds {
				seeds[i] = "hammer-" + string(rune('a'+g)) + "-" + string(rune('a'+i))
			}
			// Shortened chains would need a second entry point; full chains
			// keep the test honest and still run in ~1s on 4 goroutines.
			outs := x16ChainRun(seeds)
			for i, s := range seeds {
				if want := riskChain(s); outs[i] != want {
					t.Errorf("goroutine %d lane %d: digest mismatch", g, i)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

// BenchmarkX16Step: aggregate 16-lane iteration cost. Per-chain cost is
// ns/op ÷ 16 — compare against BenchmarkSingleComposedAVX2 (the no-SHA-NI
// scalar path it would replace).
func BenchmarkX16Step(b *testing.B) {
	if !x16Available() {
		b.Skip("no AVX-512")
	}
	x := x16Pool.Get().(*x16Chains)
	defer x16Pool.Put(x)
	for i := 0; i < 16; i++ {
		x.bufs[i][0] = byte(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x.step()
	}
}
