//go:build amd64 && !purego

package main

import (
	"crypto/sha256"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"unsafe"
)

func requireAVX512(tb testing.TB) {
	tb.Helper()
	if !cpuHasAVX512() {
		tb.Skip("AVX-512 F/DQ/BW/VL not present on this processor")
	}
}

// TestX16LaneLayout pins the memory layout the vendored routine reads: a
// 128-byte lane whose second block is the standard padding for a 64-byte
// message, and round constants broadcast sixteen lanes wide.
func TestX16LaneLayout(t *testing.T) {
	if size := unsafe.Sizeof(x16Lane{}); size != 128 {
		t.Fatalf("lane size %d, want 128", size)
	}
	var want [64]byte
	want[0] = 0x80
	want[63] = 0
	want[62] = 0x02 // 512 bits, big-endian
	if sha256Padding64 != want {
		t.Fatalf("padding block %x", sha256Padding64)
	}
	for round := range sha256RoundConstants {
		for lane := range 8 {
			got := x16RoundTable[round*8+lane]
			if uint32(got) != sha256RoundConstants[round] || uint32(got>>32) != sha256RoundConstants[round] {
				t.Fatalf("round %d lane %d: table %#x", round, lane, got)
			}
		}
	}
	chains := x16Pool.Get().(*x16Chains)
	defer x16Pool.Put(chains)
	for lane := range chains.lanes {
		if len(chains.inputs[lane]) != 128 || &chains.inputs[lane][0] != (*byte)(unsafe.Pointer(&chains.lanes[lane].hex[0])) {
			t.Fatalf("lane %d input slice does not cover the lane", lane)
		}
		if chains.lanes[lane].pad != sha256Padding64 {
			t.Fatalf("lane %d padding block not initialized", lane)
		}
	}
}

// TestX16Sum64MatchesStandardLibrary: every lane must be bit-identical to
// crypto/sha256, including equal lanes (no cross-lane leaks through the
// transposed state) and structured fills.
func TestX16Sum64MatchesStandardLibrary(t *testing.T) {
	requireAVX512(t)
	random := rand.New(rand.NewSource(1616))
	var inputs [x16Lanes][sha256.Size * 2]byte
	var outputs [x16Lanes][sha256.Size]byte
	cases := 5000
	if testing.Short() {
		cases = 500
	}
	for testCase := range cases {
		for lane := range inputs {
			random.Read(inputs[lane][:])
		}
		if testCase%10 == 9 {
			inputs[0] = inputs[15]
			inputs[7] = inputs[8]
		}
		x16Sum64(&inputs, &outputs)
		for lane := range inputs {
			if want := sha256.Sum256(inputs[lane][:]); outputs[lane] != want {
				t.Fatalf("case %d lane %d: %x != %x", testCase, lane, outputs[lane], want)
			}
		}
	}
	for value := 0; value < 256; value += 15 {
		for lane := range inputs {
			for index := range inputs[lane] {
				inputs[lane][index] = byte(value + lane)
			}
		}
		x16Sum64(&inputs, &outputs)
		for lane := range inputs {
			if want := sha256.Sum256(inputs[lane][:]); outputs[lane] != want {
				t.Fatalf("fill %d lane %d mismatch", value, lane)
			}
		}
	}
}

// TestX16ChainMatchesReference: complete 50,000-round lockstep chains must
// equal the independent reference digest for digest, for full and partial
// batches, with the unused lanes' zeros not disturbing the live ones.
func TestX16ChainMatchesReference(t *testing.T) {
	requireAVX512(t)
	seeds := make([]string, x16Lanes)
	for lane := range seeds {
		seeds[lane] = "x16-chain-" + strconv.Itoa(lane)
	}
	var results [x16Lanes][sha256.Size * 2]byte
	x16ChainRun(seeds, &results)
	for lane, seed := range seeds {
		if got := string(results[lane][:]); got != referenceRisk(seed) {
			t.Fatalf("lane %d (%s): %s", lane, seed, got)
		}
	}
	partial := []string{"0.48", "hello world", "symbols: +/%", "none", "x", "y", "z", "w", "v"}
	results = [x16Lanes][sha256.Size * 2]byte{}
	x16ChainRun(partial, &results)
	for lane, seed := range partial {
		if got := string(results[lane][:]); got != referenceRisk(seed) {
			t.Fatalf("partial lane %d (%s): %s", lane, seed, got)
		}
	}
}

// TestX16BatchMatchesReference runs the worker-side batch entry point for
// every batch size the worker can hand it.
func TestX16BatchMatchesReference(t *testing.T) {
	requireAVX512(t)
	for _, size := range []int{maxRiskLanes, 9, x16Lanes} {
		batch := make([]*riskJob, size)
		for index := range batch {
			batch[index] = &riskJob{seed: "x16-batch-" + strconv.Itoa(size) + "-" + strconv.Itoa(index), result: make(chan riskResult, 1)}
		}
		runRiskBatchX16(batch)
		for index := range batch {
			got := <-batch[index].result
			if string(got.hash[:]) != referenceRisk(batch[index].seed) {
				t.Fatalf("batch size %d lane %d diverged from reference", size, index)
			}
		}
	}
}

// TestX16SelfTestAndRace exercises the boot gate: the differential self-test
// must pass, and the race must cross-check digests and report a ratio.
func TestX16SelfTestAndRace(t *testing.T) {
	requireAVX512(t)
	if !x16SelfTest() {
		t.Fatal("self-test failed")
	}
	ok, ratio, minBatch := x16BootRace()
	if ratio <= 0 {
		t.Fatalf("race returned no measurement (ok=%v ratio=%v)", ok, ratio)
	}
	if ok && (minBatch < maxRiskLanes || minBatch > x16Lanes) {
		t.Fatalf("minimum batch %d out of range", minBatch)
	}
	t.Logf("sixteen-lane batch vs displaced path: %.2fx (enabled=%v, minimum batch %d, SHA-NI=%v)", ratio, ok, minBatch, useSHANIPair)
}

// TestX16RaceHammer: concurrent batches, as two workers run them, must not
// share state through the pool or the transposed buffers.
func TestX16RaceHammer(t *testing.T) {
	requireAVX512(t)
	var group sync.WaitGroup
	for goroutine := range 4 {
		group.Add(1)
		go func() {
			defer group.Done()
			seeds := make([]string, x16Lanes)
			for lane := range seeds {
				seeds[lane] = "hammer-" + strconv.Itoa(goroutine) + "-" + strconv.Itoa(lane)
			}
			var results [x16Lanes][sha256.Size * 2]byte
			x16ChainRun(seeds, &results)
			for lane, seed := range seeds {
				if string(results[lane][:]) != referenceRisk(seed) {
					t.Errorf("goroutine %d lane %d: digest mismatch", goroutine, lane)
					return
				}
			}
		}()
	}
	group.Wait()
}

// BenchmarkX16Step measures one round of all sixteen lanes; per-chain cost
// is ns/op ÷ 16. Compare with BenchmarkRisk ÷ 50,000 on the same machine.
func BenchmarkX16Step(b *testing.B) {
	requireAVX512(b)
	chains := x16Pool.Get().(*x16Chains)
	defer x16Pool.Put(chains)
	for lane := range chains.lanes {
		chains.lanes[lane].hex[0] = uint16(lane)
	}
	for b.Loop() {
		chains.step()
	}
}

func BenchmarkX16Chain(b *testing.B) {
	requireAVX512(b)
	seeds := make([]string, x16Lanes)
	var results [x16Lanes][sha256.Size * 2]byte
	for index := 0; index < b.N; index++ {
		for lane := range seeds {
			seeds[lane] = strconv.Itoa(index*x16Lanes + lane)
		}
		x16ChainRun(seeds, &results)
	}
}
