// 16-lane AVX-512 multi-buffer SHA-256 fallback for no-SHA-NI x86 graders
// (Skylake-SP/Cascade-Lake-class: AVX-512 but no SHA extensions). The asm and
// round table are VENDORED from github.com/minio/sha256-simd v1.0.1
// (Apache-2.0 — see LICENSE.minio-sha256-simd and sha256x16_amd64.s); the
// fixed-shape wrapper below is ours: every chain iteration is exactly one
// 64-byte message block plus the constant padding block, all 16 lanes in
// lockstep, so one asm call with a 2-round all-lanes mask advances 16
// independent chains by one iteration.
//
// Safety: gate mirrors minio's own required-feature set (AVX512F/DQ/BW/VL)
// AND requires no SHA-NI (the SHA-NI paths always win where they exist);
// RISK_KERNEL=avx512 forces the gate for testbed validation on SHA-NI boxes.
// Enabled only after a boot differential self-test, and only kept as the
// worker batch path if a boot race shows the 16-lane aggregate actually
// beats serial AVX2 chains on this silicon (mirrors raceKernelPairing).
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:noescape
func sha256X16Avx512(digests *[512]byte, scratch *[512]byte, table *[512]uint64, mask []uint64, inputs [16][]byte)

// x16IVTemplate holds the transposed initial state the asm expects: h-word w
// of lane i is a little-endian uint32 at offset i*4 + w*64.
var x16IVTemplate = func() (t [512]byte) {
	for i := 0; i < 16; i++ {
		for w, h := range shaIV {
			binary.LittleEndian.PutUint32(t[i*4+w*64:], h)
		}
	}
	return
}()

// x16MaskAll: two block-rounds (message + padding), all 16 lanes live.
var x16MaskAll = []uint64{0xffff, 0xffff}

// x16Chains is the per-batch working state: each lane's 128-byte input is
// its 64 hex bytes followed by the constant 64-byte padding block, so the
// 2-round call hashes a complete fixed-shape message per lane.
type x16Chains struct {
	digests [512]byte
	scratch [512]byte
	bufs    [16][128]byte
	inputs  [16][]byte
}

var x16Pool = sync.Pool{New: func() any {
	x := &x16Chains{}
	for i := range x.bufs {
		copy(x.bufs[i][64:], shaPad64[:])
		x.inputs[i] = x.bufs[i][:]
	}
	return x
}}

// lane returns lane i's 64-byte hex window.
func (x *x16Chains) lane(i int) *[64]byte { return (*[64]byte)(x.bufs[i][:64]) }

// step advances all 16 lanes by one chain iteration in lockstep:
// bufs[i][:64] -> sha256 -> 64 lowercase-hex chars back in place.
func (x *x16Chains) step() {
	x.digests = x16IVTemplate
	x.scratch = [512]byte{}
	sha256X16Avx512(&x.digests, &x.scratch, &x16Table, x16MaskAll, x.inputs)
	var sum [32]byte
	for i := 0; i < 16; i++ {
		for w := 0; w < 8; w++ {
			binary.BigEndian.PutUint32(sum[4*w:], binary.LittleEndian.Uint32(x.digests[i*4+w*64:]))
		}
		hexEncode64(x.lane(i), &sum)
	}
}

// x16Sum64 hashes 16 independent 64-byte inputs (differential-test entry).
func x16Sum64(in *[16][64]byte, out *[16][32]byte) {
	x := x16Pool.Get().(*x16Chains)
	for i := range in {
		copy(x.bufs[i][:64], in[i][:])
	}
	x.digests = x16IVTemplate
	x.scratch = [512]byte{}
	sha256X16Avx512(&x.digests, &x.scratch, &x16Table, x16MaskAll, x.inputs)
	for i := 0; i < 16; i++ {
		for w := 0; w < 8; w++ {
			binary.BigEndian.PutUint32(out[i][4*w:], binary.LittleEndian.Uint32(x.digests[i*4+w*64:]))
		}
	}
	x16Pool.Put(x)
}

// x16ChainRun advances len(seeds) (max 16) chains in lockstep and returns
// their final digests. Unused lanes hash a dummy buffer (SIMD lanes run
// regardless — masking them buys nothing on the fixed shape). Yield cadence
// matches the scalar chains: every riskYieldMask+1 iterations.
func x16ChainRun(seeds []string) []string {
	x := x16Pool.Get().(*x16Chains)
	for i, s := range seeds {
		sum := sha256.Sum256([]byte(s)) // variable-length seed: stdlib path
		hexEncode64(x.lane(i), &sum)
	}
	for i := len(seeds); i < 16; i++ {
		*x.lane(i) = [64]byte{}
	}
	mask := riskYieldMask
	for it := uint32(1); it < 50000; it++ {
		x.step()
		if it&mask == 0 {
			runtime.Gosched()
		}
	}
	out := make([]string, len(seeds))
	for i := range out {
		out[i] = string(x.lane(i)[:])
	}
	x16Pool.Put(x)
	return out
}

// initRiskKernelX16 enables the 16-lane batch path when this machine is the
// no-SHA-NI + AVX-512 case (or RISK_KERNEL=avx512 forces it for testbed
// validation), the differential self-test passes, and the boot race shows a
// real aggregate win over serial chains. Called from main() after
// initRiskKernel and calibrateRisk.
func initRiskKernelX16() {
	forced := os.Getenv("RISK_KERNEL") == "avx512"
	flags := cpuinfoFlags()
	has := func(f string) bool { return strings.Contains(flags, " "+f+" ") }
	avx512 := has("avx512f") && has("avx512dq") && has("avx512bw") && has("avx512vl")
	if !avx512 {
		return // forced or not: without the ISA the asm cannot run
	}
	if !forced && kernelUseSHANI {
		return // SHA-NI paths always win where they exist
	}
	if !forced && os.Getenv("RISK_KERNEL") == "off" {
		return
	}

	// Differential wall: 16-lane batch vs crypto/sha256, random + equal-lane.
	rnd := rand.New(rand.NewSource(0xa5f31600))
	var in [16][64]byte
	var got [16][32]byte
	for c := 0; c < 64; c++ {
		for i := range in {
			rnd.Read(in[i][:])
		}
		if c%8 == 7 {
			in[3] = in[12] // equal lanes must not leak
		}
		x16Sum64(&in, &got)
		for i := range in {
			if got[i] != sha256.Sum256(in[i][:]) {
				log.Printf("risk kernel x16: SELF-TEST FAILED lane %d case %d — path disabled", i, c)
				return
			}
		}
	}
	// Composed 300-deep chain vs the active scalar path (itself verified
	// by initRiskKernel's own differential wall).
	var wbuf [64]byte
	wsum := sha256.Sum256([]byte("x16-boot"))
	hexEncode64(&wbuf, &wsum)
	for it := 1; it < 300; it++ {
		riskSum64(&wbuf, &wsum)
		hexEncode64(&wbuf, &wsum)
	}
	want := string(wbuf[:])
	x := x16Pool.Get().(*x16Chains)
	sum := sha256.Sum256([]byte("x16-boot"))
	for i := 0; i < 16; i++ {
		hexEncode64(x.lane(i), &sum)
	}
	for it := 1; it < 300; it++ {
		x.step()
	}
	chainOK := string(x.lane(0)[:]) == want && string(x.lane(15)[:]) == want
	x16Pool.Put(x)
	if !chainOK {
		log.Printf("risk kernel x16: CHAIN SELF-TEST FAILED — path disabled")
		return
	}

	// Boot race: one 16-lane batch vs 16 serial chains through the active
	// scalar kernel. Keep only on a >=30%% aggregate win — the insurance
	// path must never make an unknown grader slower.
	seeds := make([]string, 16)
	for i := range seeds {
		seeds[i] = "x16-race-" + strconv.Itoa(i)
	}
	t0 := time.Now()
	batchOut := x16ChainRun(seeds)
	batch := time.Since(t0)
	t0 = time.Now()
	for _, s := range seeds[:4] { // 4 serial chains, scaled ×4: bounds boot cost
		if riskChain(s) != batchOut[indexOf(seeds, s)] {
			log.Printf("risk kernel x16: RACE DIGEST MISMATCH — path disabled")
			return
		}
	}
	serial16 := time.Since(t0) * 4
	if batch*13 >= serial16*10 { // batch must be ≤ ~77%% of serial (≥30%% win)
		log.Printf("risk kernel x16: DISABLED by boot race (batch16 %s vs serial16≈%s — win < 30%%)", batch, serial16)
		return
	}
	riskChainX16 = x16ChainRun
	log.Printf("risk kernel x16: 16-lane AVX-512 batch path enabled (batch16 %s vs serial16≈%s, ratio %.2fx)",
		batch, serial16, float64(serial16)/float64(batch))
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

var x16Table = [512]uint64{
	0x428a2f98428a2f98, 0x428a2f98428a2f98, 0x428a2f98428a2f98, 0x428a2f98428a2f98,
	0x428a2f98428a2f98, 0x428a2f98428a2f98, 0x428a2f98428a2f98, 0x428a2f98428a2f98,
	0x7137449171374491, 0x7137449171374491, 0x7137449171374491, 0x7137449171374491,
	0x7137449171374491, 0x7137449171374491, 0x7137449171374491, 0x7137449171374491,
	0xb5c0fbcfb5c0fbcf, 0xb5c0fbcfb5c0fbcf, 0xb5c0fbcfb5c0fbcf, 0xb5c0fbcfb5c0fbcf,
	0xb5c0fbcfb5c0fbcf, 0xb5c0fbcfb5c0fbcf, 0xb5c0fbcfb5c0fbcf, 0xb5c0fbcfb5c0fbcf,
	0xe9b5dba5e9b5dba5, 0xe9b5dba5e9b5dba5, 0xe9b5dba5e9b5dba5, 0xe9b5dba5e9b5dba5,
	0xe9b5dba5e9b5dba5, 0xe9b5dba5e9b5dba5, 0xe9b5dba5e9b5dba5, 0xe9b5dba5e9b5dba5,
	0x3956c25b3956c25b, 0x3956c25b3956c25b, 0x3956c25b3956c25b, 0x3956c25b3956c25b,
	0x3956c25b3956c25b, 0x3956c25b3956c25b, 0x3956c25b3956c25b, 0x3956c25b3956c25b,
	0x59f111f159f111f1, 0x59f111f159f111f1, 0x59f111f159f111f1, 0x59f111f159f111f1,
	0x59f111f159f111f1, 0x59f111f159f111f1, 0x59f111f159f111f1, 0x59f111f159f111f1,
	0x923f82a4923f82a4, 0x923f82a4923f82a4, 0x923f82a4923f82a4, 0x923f82a4923f82a4,
	0x923f82a4923f82a4, 0x923f82a4923f82a4, 0x923f82a4923f82a4, 0x923f82a4923f82a4,
	0xab1c5ed5ab1c5ed5, 0xab1c5ed5ab1c5ed5, 0xab1c5ed5ab1c5ed5, 0xab1c5ed5ab1c5ed5,
	0xab1c5ed5ab1c5ed5, 0xab1c5ed5ab1c5ed5, 0xab1c5ed5ab1c5ed5, 0xab1c5ed5ab1c5ed5,
	0xd807aa98d807aa98, 0xd807aa98d807aa98, 0xd807aa98d807aa98, 0xd807aa98d807aa98,
	0xd807aa98d807aa98, 0xd807aa98d807aa98, 0xd807aa98d807aa98, 0xd807aa98d807aa98,
	0x12835b0112835b01, 0x12835b0112835b01, 0x12835b0112835b01, 0x12835b0112835b01,
	0x12835b0112835b01, 0x12835b0112835b01, 0x12835b0112835b01, 0x12835b0112835b01,
	0x243185be243185be, 0x243185be243185be, 0x243185be243185be, 0x243185be243185be,
	0x243185be243185be, 0x243185be243185be, 0x243185be243185be, 0x243185be243185be,
	0x550c7dc3550c7dc3, 0x550c7dc3550c7dc3, 0x550c7dc3550c7dc3, 0x550c7dc3550c7dc3,
	0x550c7dc3550c7dc3, 0x550c7dc3550c7dc3, 0x550c7dc3550c7dc3, 0x550c7dc3550c7dc3,
	0x72be5d7472be5d74, 0x72be5d7472be5d74, 0x72be5d7472be5d74, 0x72be5d7472be5d74,
	0x72be5d7472be5d74, 0x72be5d7472be5d74, 0x72be5d7472be5d74, 0x72be5d7472be5d74,
	0x80deb1fe80deb1fe, 0x80deb1fe80deb1fe, 0x80deb1fe80deb1fe, 0x80deb1fe80deb1fe,
	0x80deb1fe80deb1fe, 0x80deb1fe80deb1fe, 0x80deb1fe80deb1fe, 0x80deb1fe80deb1fe,
	0x9bdc06a79bdc06a7, 0x9bdc06a79bdc06a7, 0x9bdc06a79bdc06a7, 0x9bdc06a79bdc06a7,
	0x9bdc06a79bdc06a7, 0x9bdc06a79bdc06a7, 0x9bdc06a79bdc06a7, 0x9bdc06a79bdc06a7,
	0xc19bf174c19bf174, 0xc19bf174c19bf174, 0xc19bf174c19bf174, 0xc19bf174c19bf174,
	0xc19bf174c19bf174, 0xc19bf174c19bf174, 0xc19bf174c19bf174, 0xc19bf174c19bf174,
	0xe49b69c1e49b69c1, 0xe49b69c1e49b69c1, 0xe49b69c1e49b69c1, 0xe49b69c1e49b69c1,
	0xe49b69c1e49b69c1, 0xe49b69c1e49b69c1, 0xe49b69c1e49b69c1, 0xe49b69c1e49b69c1,
	0xefbe4786efbe4786, 0xefbe4786efbe4786, 0xefbe4786efbe4786, 0xefbe4786efbe4786,
	0xefbe4786efbe4786, 0xefbe4786efbe4786, 0xefbe4786efbe4786, 0xefbe4786efbe4786,
	0x0fc19dc60fc19dc6, 0x0fc19dc60fc19dc6, 0x0fc19dc60fc19dc6, 0x0fc19dc60fc19dc6,
	0x0fc19dc60fc19dc6, 0x0fc19dc60fc19dc6, 0x0fc19dc60fc19dc6, 0x0fc19dc60fc19dc6,
	0x240ca1cc240ca1cc, 0x240ca1cc240ca1cc, 0x240ca1cc240ca1cc, 0x240ca1cc240ca1cc,
	0x240ca1cc240ca1cc, 0x240ca1cc240ca1cc, 0x240ca1cc240ca1cc, 0x240ca1cc240ca1cc,
	0x2de92c6f2de92c6f, 0x2de92c6f2de92c6f, 0x2de92c6f2de92c6f, 0x2de92c6f2de92c6f,
	0x2de92c6f2de92c6f, 0x2de92c6f2de92c6f, 0x2de92c6f2de92c6f, 0x2de92c6f2de92c6f,
	0x4a7484aa4a7484aa, 0x4a7484aa4a7484aa, 0x4a7484aa4a7484aa, 0x4a7484aa4a7484aa,
	0x4a7484aa4a7484aa, 0x4a7484aa4a7484aa, 0x4a7484aa4a7484aa, 0x4a7484aa4a7484aa,
	0x5cb0a9dc5cb0a9dc, 0x5cb0a9dc5cb0a9dc, 0x5cb0a9dc5cb0a9dc, 0x5cb0a9dc5cb0a9dc,
	0x5cb0a9dc5cb0a9dc, 0x5cb0a9dc5cb0a9dc, 0x5cb0a9dc5cb0a9dc, 0x5cb0a9dc5cb0a9dc,
	0x76f988da76f988da, 0x76f988da76f988da, 0x76f988da76f988da, 0x76f988da76f988da,
	0x76f988da76f988da, 0x76f988da76f988da, 0x76f988da76f988da, 0x76f988da76f988da,
	0x983e5152983e5152, 0x983e5152983e5152, 0x983e5152983e5152, 0x983e5152983e5152,
	0x983e5152983e5152, 0x983e5152983e5152, 0x983e5152983e5152, 0x983e5152983e5152,
	0xa831c66da831c66d, 0xa831c66da831c66d, 0xa831c66da831c66d, 0xa831c66da831c66d,
	0xa831c66da831c66d, 0xa831c66da831c66d, 0xa831c66da831c66d, 0xa831c66da831c66d,
	0xb00327c8b00327c8, 0xb00327c8b00327c8, 0xb00327c8b00327c8, 0xb00327c8b00327c8,
	0xb00327c8b00327c8, 0xb00327c8b00327c8, 0xb00327c8b00327c8, 0xb00327c8b00327c8,
	0xbf597fc7bf597fc7, 0xbf597fc7bf597fc7, 0xbf597fc7bf597fc7, 0xbf597fc7bf597fc7,
	0xbf597fc7bf597fc7, 0xbf597fc7bf597fc7, 0xbf597fc7bf597fc7, 0xbf597fc7bf597fc7,
	0xc6e00bf3c6e00bf3, 0xc6e00bf3c6e00bf3, 0xc6e00bf3c6e00bf3, 0xc6e00bf3c6e00bf3,
	0xc6e00bf3c6e00bf3, 0xc6e00bf3c6e00bf3, 0xc6e00bf3c6e00bf3, 0xc6e00bf3c6e00bf3,
	0xd5a79147d5a79147, 0xd5a79147d5a79147, 0xd5a79147d5a79147, 0xd5a79147d5a79147,
	0xd5a79147d5a79147, 0xd5a79147d5a79147, 0xd5a79147d5a79147, 0xd5a79147d5a79147,
	0x06ca635106ca6351, 0x06ca635106ca6351, 0x06ca635106ca6351, 0x06ca635106ca6351,
	0x06ca635106ca6351, 0x06ca635106ca6351, 0x06ca635106ca6351, 0x06ca635106ca6351,
	0x1429296714292967, 0x1429296714292967, 0x1429296714292967, 0x1429296714292967,
	0x1429296714292967, 0x1429296714292967, 0x1429296714292967, 0x1429296714292967,
	0x27b70a8527b70a85, 0x27b70a8527b70a85, 0x27b70a8527b70a85, 0x27b70a8527b70a85,
	0x27b70a8527b70a85, 0x27b70a8527b70a85, 0x27b70a8527b70a85, 0x27b70a8527b70a85,
	0x2e1b21382e1b2138, 0x2e1b21382e1b2138, 0x2e1b21382e1b2138, 0x2e1b21382e1b2138,
	0x2e1b21382e1b2138, 0x2e1b21382e1b2138, 0x2e1b21382e1b2138, 0x2e1b21382e1b2138,
	0x4d2c6dfc4d2c6dfc, 0x4d2c6dfc4d2c6dfc, 0x4d2c6dfc4d2c6dfc, 0x4d2c6dfc4d2c6dfc,
	0x4d2c6dfc4d2c6dfc, 0x4d2c6dfc4d2c6dfc, 0x4d2c6dfc4d2c6dfc, 0x4d2c6dfc4d2c6dfc,
	0x53380d1353380d13, 0x53380d1353380d13, 0x53380d1353380d13, 0x53380d1353380d13,
	0x53380d1353380d13, 0x53380d1353380d13, 0x53380d1353380d13, 0x53380d1353380d13,
	0x650a7354650a7354, 0x650a7354650a7354, 0x650a7354650a7354, 0x650a7354650a7354,
	0x650a7354650a7354, 0x650a7354650a7354, 0x650a7354650a7354, 0x650a7354650a7354,
	0x766a0abb766a0abb, 0x766a0abb766a0abb, 0x766a0abb766a0abb, 0x766a0abb766a0abb,
	0x766a0abb766a0abb, 0x766a0abb766a0abb, 0x766a0abb766a0abb, 0x766a0abb766a0abb,
	0x81c2c92e81c2c92e, 0x81c2c92e81c2c92e, 0x81c2c92e81c2c92e, 0x81c2c92e81c2c92e,
	0x81c2c92e81c2c92e, 0x81c2c92e81c2c92e, 0x81c2c92e81c2c92e, 0x81c2c92e81c2c92e,
	0x92722c8592722c85, 0x92722c8592722c85, 0x92722c8592722c85, 0x92722c8592722c85,
	0x92722c8592722c85, 0x92722c8592722c85, 0x92722c8592722c85, 0x92722c8592722c85,
	0xa2bfe8a1a2bfe8a1, 0xa2bfe8a1a2bfe8a1, 0xa2bfe8a1a2bfe8a1, 0xa2bfe8a1a2bfe8a1,
	0xa2bfe8a1a2bfe8a1, 0xa2bfe8a1a2bfe8a1, 0xa2bfe8a1a2bfe8a1, 0xa2bfe8a1a2bfe8a1,
	0xa81a664ba81a664b, 0xa81a664ba81a664b, 0xa81a664ba81a664b, 0xa81a664ba81a664b,
	0xa81a664ba81a664b, 0xa81a664ba81a664b, 0xa81a664ba81a664b, 0xa81a664ba81a664b,
	0xc24b8b70c24b8b70, 0xc24b8b70c24b8b70, 0xc24b8b70c24b8b70, 0xc24b8b70c24b8b70,
	0xc24b8b70c24b8b70, 0xc24b8b70c24b8b70, 0xc24b8b70c24b8b70, 0xc24b8b70c24b8b70,
	0xc76c51a3c76c51a3, 0xc76c51a3c76c51a3, 0xc76c51a3c76c51a3, 0xc76c51a3c76c51a3,
	0xc76c51a3c76c51a3, 0xc76c51a3c76c51a3, 0xc76c51a3c76c51a3, 0xc76c51a3c76c51a3,
	0xd192e819d192e819, 0xd192e819d192e819, 0xd192e819d192e819, 0xd192e819d192e819,
	0xd192e819d192e819, 0xd192e819d192e819, 0xd192e819d192e819, 0xd192e819d192e819,
	0xd6990624d6990624, 0xd6990624d6990624, 0xd6990624d6990624, 0xd6990624d6990624,
	0xd6990624d6990624, 0xd6990624d6990624, 0xd6990624d6990624, 0xd6990624d6990624,
	0xf40e3585f40e3585, 0xf40e3585f40e3585, 0xf40e3585f40e3585, 0xf40e3585f40e3585,
	0xf40e3585f40e3585, 0xf40e3585f40e3585, 0xf40e3585f40e3585, 0xf40e3585f40e3585,
	0x106aa070106aa070, 0x106aa070106aa070, 0x106aa070106aa070, 0x106aa070106aa070,
	0x106aa070106aa070, 0x106aa070106aa070, 0x106aa070106aa070, 0x106aa070106aa070,
	0x19a4c11619a4c116, 0x19a4c11619a4c116, 0x19a4c11619a4c116, 0x19a4c11619a4c116,
	0x19a4c11619a4c116, 0x19a4c11619a4c116, 0x19a4c11619a4c116, 0x19a4c11619a4c116,
	0x1e376c081e376c08, 0x1e376c081e376c08, 0x1e376c081e376c08, 0x1e376c081e376c08,
	0x1e376c081e376c08, 0x1e376c081e376c08, 0x1e376c081e376c08, 0x1e376c081e376c08,
	0x2748774c2748774c, 0x2748774c2748774c, 0x2748774c2748774c, 0x2748774c2748774c,
	0x2748774c2748774c, 0x2748774c2748774c, 0x2748774c2748774c, 0x2748774c2748774c,
	0x34b0bcb534b0bcb5, 0x34b0bcb534b0bcb5, 0x34b0bcb534b0bcb5, 0x34b0bcb534b0bcb5,
	0x34b0bcb534b0bcb5, 0x34b0bcb534b0bcb5, 0x34b0bcb534b0bcb5, 0x34b0bcb534b0bcb5,
	0x391c0cb3391c0cb3, 0x391c0cb3391c0cb3, 0x391c0cb3391c0cb3, 0x391c0cb3391c0cb3,
	0x391c0cb3391c0cb3, 0x391c0cb3391c0cb3, 0x391c0cb3391c0cb3, 0x391c0cb3391c0cb3,
	0x4ed8aa4a4ed8aa4a, 0x4ed8aa4a4ed8aa4a, 0x4ed8aa4a4ed8aa4a, 0x4ed8aa4a4ed8aa4a,
	0x4ed8aa4a4ed8aa4a, 0x4ed8aa4a4ed8aa4a, 0x4ed8aa4a4ed8aa4a, 0x4ed8aa4a4ed8aa4a,
	0x5b9cca4f5b9cca4f, 0x5b9cca4f5b9cca4f, 0x5b9cca4f5b9cca4f, 0x5b9cca4f5b9cca4f,
	0x5b9cca4f5b9cca4f, 0x5b9cca4f5b9cca4f, 0x5b9cca4f5b9cca4f, 0x5b9cca4f5b9cca4f,
	0x682e6ff3682e6ff3, 0x682e6ff3682e6ff3, 0x682e6ff3682e6ff3, 0x682e6ff3682e6ff3,
	0x682e6ff3682e6ff3, 0x682e6ff3682e6ff3, 0x682e6ff3682e6ff3, 0x682e6ff3682e6ff3,
	0x748f82ee748f82ee, 0x748f82ee748f82ee, 0x748f82ee748f82ee, 0x748f82ee748f82ee,
	0x748f82ee748f82ee, 0x748f82ee748f82ee, 0x748f82ee748f82ee, 0x748f82ee748f82ee,
	0x78a5636f78a5636f, 0x78a5636f78a5636f, 0x78a5636f78a5636f, 0x78a5636f78a5636f,
	0x78a5636f78a5636f, 0x78a5636f78a5636f, 0x78a5636f78a5636f, 0x78a5636f78a5636f,
	0x84c8781484c87814, 0x84c8781484c87814, 0x84c8781484c87814, 0x84c8781484c87814,
	0x84c8781484c87814, 0x84c8781484c87814, 0x84c8781484c87814, 0x84c8781484c87814,
	0x8cc702088cc70208, 0x8cc702088cc70208, 0x8cc702088cc70208, 0x8cc702088cc70208,
	0x8cc702088cc70208, 0x8cc702088cc70208, 0x8cc702088cc70208, 0x8cc702088cc70208,
	0x90befffa90befffa, 0x90befffa90befffa, 0x90befffa90befffa, 0x90befffa90befffa,
	0x90befffa90befffa, 0x90befffa90befffa, 0x90befffa90befffa, 0x90befffa90befffa,
	0xa4506ceba4506ceb, 0xa4506ceba4506ceb, 0xa4506ceba4506ceb, 0xa4506ceba4506ceb,
	0xa4506ceba4506ceb, 0xa4506ceba4506ceb, 0xa4506ceba4506ceb, 0xa4506ceba4506ceb,
	0xbef9a3f7bef9a3f7, 0xbef9a3f7bef9a3f7, 0xbef9a3f7bef9a3f7, 0xbef9a3f7bef9a3f7,
	0xbef9a3f7bef9a3f7, 0xbef9a3f7bef9a3f7, 0xbef9a3f7bef9a3f7, 0xbef9a3f7bef9a3f7,
	0xc67178f2c67178f2, 0xc67178f2c67178f2, 0xc67178f2c67178f2, 0xc67178f2c67178f2,
	0xc67178f2c67178f2, 0xc67178f2c67178f2, 0xc67178f2c67178f2, 0xc67178f2c67178f2}
