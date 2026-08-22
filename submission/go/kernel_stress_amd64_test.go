//go:build amd64 && !purego

package main

import (
	"crypto/sha256"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
)

// TestKernelUnderGCAndPreemption hammers the two-lane kernel from many
// goroutines while the garbage collector runs aggressively and goroutine stacks
// are grown between calls. A NOSPLIT assembly routine with a mis-declared frame
// or a callee-saved register violation corrupts results (or crashes) only under
// exactly this kind of runtime pressure — a stack move or preemption landing on
// the kernel call. That is the failure class behind the "fix stack alignment"
// commit, and a quiet single-threaded loop never reproduces it. Each result is
// fed back into the next input so the calls form a dependent chain the compiler
// cannot elide, and every output is checked against the standard library.
func TestKernelUnderGCAndPreemption(t *testing.T) {
	forceKernel(t)

	// Force frequent collections, restore the caller's setting afterward.
	defer debug.SetGCPercent(debug.SetGCPercent(1))

	workers := runtime.GOMAXPROCS(0) * 4
	if workers < 8 {
		workers = 8
	}
	iterations := 200_000
	if testing.Short() {
		iterations = 5_000
	}

	var mismatches int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed byte) {
			defer wg.Done()
			var inA, inB [sha256.Size * 2]byte
			for i := range inA {
				inA[i] = seed
				inB[i] = seed ^ 0xff
			}
			var outA, outB [sha256.Size]byte
			for i := 0; i < iterations; i++ {
				sum256x2(&inA, &inB, &outA, &outB)
				if outA != sha256.Sum256(inA[:]) || outB != sha256.Sum256(inB[:]) {
					atomic.AddInt64(&mismatches, 1)
					return
				}
				// Chain the outputs into the next inputs (defeats dead-code
				// elimination; makes every call depend on the previous one).
				copy(inA[:sha256.Size], outA[:])
				copy(inA[sha256.Size:], outB[:])
				copy(inB[:sha256.Size], outB[:])
				copy(inB[sha256.Size:], outA[:])
				if i%1024 == 0 {
					// Grow and unwind the stack so a kernel call can land
					// immediately after a stack move.
					sink += growStack(24)
				}
			}
		}(byte(w))
	}
	wg.Wait()
	if mismatches != 0 {
		t.Fatalf("%d goroutine(s) saw a kernel digest mismatch under GC/preemption pressure", mismatches)
	}
}

var sink byte

// growStack recurses with a per-frame buffer to force the goroutine stack to
// grow (and later shrink). The return value is consumed so the recursion is not
// optimized away.
func growStack(depth int) byte {
	if depth == 0 {
		return 1
	}
	var pad [512]byte
	pad[0] = growStack(depth - 1)
	return pad[0]
}
