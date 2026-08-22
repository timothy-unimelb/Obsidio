// Four-lane interleaved SHA-NI chain kernel (amd64 only) — Go framing for
// the vendored sha4lane_amd64.s (see its header for provenance).
//
// Why four lanes: the fused pair path runs ~2.85ms/chain on SPR; the 4-lane
// interleave measured 2.66ms/chain on the same silicon (Tim's Level-0
// medians, quad 10.65ms). Two workers × 4 lanes also drains a deeper queue
// per wakeup. Unlike the x16 batch (needs ~15 parked waiters to beat pair
// displacement), a quad only needs 4 — under the governor's measured queue
// equilibrium that fires constantly.
//
// Safety discipline is identical to every other kernel path: SHA-NI cpuinfo
// gate, boot differential self-test against crypto/sha256, boot race with a
// ≥5% win requirement against the pair path it displaces, RISK_X4=off kill
// switch. The asm has no alignment requirements (all MOVOU).
package main

import (
	"crypto/sha256"
	"log"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"time"
)

// riskChain1x/2x/4x replace each 64-byte lowercase-hex buffer with the
// lowercase-hex SHA-256 of its contents, `rounds` times, for one, two, or
// four independent chains interleaved on one core. NOSPLIT asm is not
// async-preemptible — callers must chunk rounds at the yield stride.
//
//go:noescape
func riskChain1x(buf0 *[64]byte, rounds int)

//go:noescape
func riskChain2x(buf0, buf1 *[64]byte, rounds int)

//go:noescape
func riskChain4x(buf0, buf1, buf2, buf3 *[64]byte, rounds int)

// cpuidSHA ships inside the vendored asm; our gating reads /proc/cpuinfo
// (kernelUseSHANI) instead, but the declaration must exist for the linker.
//
//go:noescape
func cpuidSHA() bool

// riskChainQuadKernel advances four chains in lockstep through the 4-lane
// kernel. Identical math to four riskChain calls (differentially tested at
// boot and in the suite); chunked at the same yield cadence as the pair path.
func riskChainQuadKernel(s0, s1, s2, s3 string) (string, string, string, string) {
	var b0, b1, b2, b3 [64]byte
	d0 := sha256.Sum256([]byte(s0))
	d1 := sha256.Sum256([]byte(s1))
	d2 := sha256.Sum256([]byte(s2))
	d3 := sha256.Sum256([]byte(s3))
	hexEncode64(&b0, &d0)
	hexEncode64(&b1, &d1)
	hexEncode64(&b2, &d2)
	hexEncode64(&b3, &d3)
	stride := riskYieldMask + 1 // 0 (mask==^0): yielding disabled, one call
	for done := uint32(1); done < 50000; {
		n := 50000 - done
		if stride != 0 && n > stride {
			n = stride
		}
		riskChain4x(&b0, &b1, &b2, &b3, int(n))
		done += n
		if done < 50000 {
			runtime.Gosched()
		}
	}
	return string(b0[:]), string(b1[:]), string(b2[:]), string(b3[:])
}

// initRiskKernelX4 installs the quad path if (a) the SHA-NI gate is up,
// (b) the kernel is bit-identical to the crypto/sha256+hex reference on this
// machine right now, and (c) it beats the pair path it displaces by ≥5% in a
// boot race. Call after initRiskKernel and raceKernelPairing.
func initRiskKernelX4() {
	if os.Getenv("RISK_X4") == "off" {
		log.Printf("risk x4: disabled by RISK_X4=off")
		return
	}
	if !kernelUseSHANI || riskSumPair == nil {
		return // needs SHA-NI silicon and a live pair path to displace
	}
	// Differential self-test: every lane must match an independent
	// crypto/sha256+hexEncode64 chain for assorted round counts. Random raw
	// bytes (not just hex) so the message schedule sees all byte values.
	rnd := rand.New(rand.NewSource(0x4a4e5f))
	var bufs, want [4][64]byte
	for _, n := range []int{1, 2, 3, 17, 256} {
		for c := 0; c < 128; c++ {
			for l := 0; l < 4; l++ {
				rnd.Read(bufs[l][:])
				want[l] = bufs[l]
				for k := 0; k < n; k++ {
					s := sha256.Sum256(want[l][:])
					hexEncode64(&want[l], &s)
				}
			}
			riskChain4x(&bufs[0], &bufs[1], &bufs[2], &bufs[3], n)
			for l := 0; l < 4; l++ {
				if bufs[l] != want[l] {
					log.Printf("risk x4: SELF-TEST FAILED (n=%d case %d lane %d) — quad path disabled", n, c, l)
					return
				}
			}
		}
	}
	// Boot race: a quad must beat the two pair calls it displaces by ≥5%
	// (median of 3). The digests are already verified, so losing the race
	// only costs the boot time spent here.
	pair2 := make([]time.Duration, 3)
	quad := make([]time.Duration, 3)
	for i := range quad {
		t0 := time.Now()
		riskChainPair("obsidio-x4-race-a", "obsidio-x4-race-b")
		riskChainPair("obsidio-x4-race-c", "obsidio-x4-race-d")
		pair2[i] = time.Since(t0)
		t0 = time.Now()
		riskChainQuadKernel("obsidio-x4-race-a", "obsidio-x4-race-b", "obsidio-x4-race-c", "obsidio-x4-race-d")
		quad[i] = time.Since(t0)
	}
	sort.Slice(pair2, func(i, j int) bool { return pair2[i] < pair2[j] })
	sort.Slice(quad, func(i, j int) bool { return quad[i] < quad[j] })
	p, q := pair2[1], quad[1]
	if os.Getenv("RISK_X4") != "on" && q*100 >= p*95 {
		log.Printf("risk x4: DISABLED by boot race (quad %s vs pair-2 %s — win < 5%%)", q, p)
		return
	}
	riskChainQuad = riskChainQuadKernel
	log.Printf("risk x4: enabled (quad %s vs pair-2 %s, ratio %.2fx)", q, p, float64(p)/float64(q))
}
