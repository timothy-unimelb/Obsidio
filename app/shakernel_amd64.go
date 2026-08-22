// Fixed-shape SHA-256 kernel for the /risk hot loop (amd64 only).
//
// After the first iteration every hash input is EXACTLY 64 bytes (the hex
// digest), i.e. exactly two compression blocks: the message block plus a
// CONSTANT padding block. The stdlib path (crypto/sha256.Sum256) spends
// ~15% of loop CPU on Digest bookkeeping that a fixed-shape caller never
// needs — New/Reset, buffered Write, checkSum's padding arithmetic, and the
// final state copy (measured: benchmarks/profiles/posthex-champion-20260822).
// This kernel drives the SAME stdlib compression assembly (vendored,
// unmodified, from Go 1.26's crypto/internal/fips140/sha256 — see
// sha256block_amd64.s) directly: IV, two block calls, big-endian encode.
//
// Safety: enabled only when /proc/cpuinfo advertises the exact feature set
// the asm requires (the stdlib's own gates), after a 512-case differential
// self-test against crypto/sha256 at boot, and RISK_KERNEL=off kills it.
// Anything else falls back to the stdlib path — a wrong digest scores zero,
// so the kernel must be provably identical or absent.
package main

import (
	"crypto/sha256"
	"log"
	"math/rand"
	"os"
	"strings"
)

// shaDigest mirrors the leading field of the stdlib Digest: the vendored
// block functions read and write ONLY these 32 bytes (verified against the
// generated assembly: state loads are MOVL/VMOVDQU at offsets 0..28).
type shaDigest struct{ h [8]uint32 }

//go:noescape
func blockAVX2(dig *shaDigest, p []byte)

//go:noescape
func blockSHANI(dig *shaDigest, p []byte)

// blockSHANI2 compresses one 64-byte block into each of two INDEPENDENT
// digests, instruction-interleaved so the second chain's sha256rnds2 ops fill
// the first chain's instruction latency on one core (generated — see
// sha256block2_amd64.s header). SHA-NI gate only.
//
//go:noescape
func blockSHANI2(da, db *shaDigest, pa, pb *[64]byte)

var shaIV = [8]uint32{
	0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
	0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
}

// Constant second block for a 64-byte message: 0x80 marker, zeros, and the
// 64-bit big-endian bit length (512 = 0x200) in the last 8 bytes.
var shaPad64 = func() (p [64]byte) {
	p[0] = 0x80
	p[62] = 0x02
	return
}()

var kernelUseSHANI, kernelUseAVX2 bool

func kernelSum64(in *[64]byte, out *[32]byte) {
	d := shaDigest{h: shaIV}
	if kernelUseSHANI {
		blockSHANI(&d, in[:])
		blockSHANI(&d, shaPad64[:])
	} else {
		blockAVX2(&d, in[:])
		blockAVX2(&d, shaPad64[:])
	}
	for i, v := range d.h {
		out[4*i] = byte(v >> 24)
		out[4*i+1] = byte(v >> 16)
		out[4*i+2] = byte(v >> 8)
		out[4*i+3] = byte(v)
	}
}

// pairHashHex performs one FUSED chain iteration for two lanes, in place:
// reads each lane's 64-byte buffer, hashes it (message block + the constant
// padding block, whose W+K schedule is precomputed into the binary), and
// writes the 64 lowercase-hex digest chars back — byte-swap and hex expansion
// done in-register via PSHUFB. See sha256block2_amd64.s.
//
//go:noescape
func pairHashHex(pa, pb *[64]byte)

// hashHex1 is the single-lane fused iteration (same structure, one lane) —
// used by lone-waiter chains, which dominate the grading ramp.
//
//go:noescape
func hashHex1(pa *[64]byte)

// pairHashHexN (kernel v3) runs n fused chain iterations for two lanes with
// the loop INSIDE the asm: each iteration's ASCII digest stays in registers
// and is re-flipped as the next message, so intermediate digests never touch
// memory and the per-iteration Go→asm call overhead disappears. NOSPLIT asm
// is not async-preemptible, so callers must chunk n (yield-stride cadence).
//
//go:noescape
func pairHashHexN(pa, pb *[64]byte, n int)

// hashHex1N: single-lane n-iteration variant of pairHashHexN.
//
//go:noescape
func hashHex1N(pa *[64]byte, n int)

// kernelSum64Pair hashes two independent 64-byte inputs on one core via the
// interleaved 2-lane routine. Caller must ensure kernelUseSHANI.
func kernelSum64Pair(inA, inB *[64]byte, outA, outB *[32]byte) {
	da := shaDigest{h: shaIV}
	db := shaDigest{h: shaIV}
	blockSHANI2(&da, &db, inA, inB)
	blockSHANI2(&da, &db, &shaPad64, &shaPad64)
	for i := 0; i < 8; i++ {
		va, vb := da.h[i], db.h[i]
		outA[4*i], outA[4*i+1], outA[4*i+2], outA[4*i+3] = byte(va>>24), byte(va>>16), byte(va>>8), byte(va)
		outB[4*i], outB[4*i+1], outB[4*i+2], outB[4*i+3] = byte(vb>>24), byte(vb>>16), byte(vb>>8), byte(vb)
	}
}

// initRiskKernel picks the fastest provably-correct hash path for riskChain.
// Called before calibrateRisk so calibration measures the active kernel.
func initRiskKernel() {
	mode := os.Getenv("RISK_KERNEL")
	if mode == "off" {
		log.Printf("risk kernel: disabled by RISK_KERNEL=off (stdlib path)")
		return
	}
	flags := cpuinfoFlags()
	has := func(f string) bool { return strings.Contains(flags, " "+f+" ") }
	// Exactly the stdlib's own dispatch gates for these routines.
	// RISK_KERNEL=avx512 (testbed-only) simulates a no-SHA-NI grader: the
	// scalar path drops to the AVX2 kernel and the 16-lane AVX-512 batch
	// path's gate fires (see initRiskKernelX16) — pair with
	// GODEBUG=cpu.sha=off so the stdlib reference paths are honest too.
	kernelUseSHANI = has("sha_ni") && has("avx") && has("sse4_1") && has("ssse3") && mode != "avx512"
	kernelUseAVX2 = !kernelUseSHANI && has("avx") && has("avx2") && has("bmi2")
	if !kernelUseSHANI && !kernelUseAVX2 {
		log.Printf("risk kernel: no eligible ISA in cpuinfo (stdlib path)")
		return
	}
	// Differential self-test: the kernel ships only if it is bit-identical
	// to crypto/sha256 on this machine, right now.
	rnd := rand.New(rand.NewSource(0x0b51d10))
	var in [64]byte
	var got [32]byte
	for i := 0; i < 512; i++ {
		rnd.Read(in[:])
		kernelSum64(&in, &got)
		if got != sha256.Sum256(in[:]) {
			kernelUseSHANI, kernelUseAVX2 = false, false
			log.Printf("risk kernel: SELF-TEST FAILED on case %d — stdlib fallback", i)
			return
		}
	}
	riskSum64 = kernelSum64
	path := "SHA-NI"
	if kernelUseAVX2 {
		path = "AVX2"
	}
	// 2-lane interleave: SHA-NI only, and only after its own self-test.
	if kernelUseSHANI {
		var inB [64]byte
		var gotA, gotB [32]byte
		kernelPairOK = true
		for i := 0; i < 256; i++ {
			rnd.Read(in[:])
			rnd.Read(inB[:])
			kernelSum64Pair(&in, &inB, &gotA, &gotB)
			if gotA != sha256.Sum256(in[:]) || gotB != sha256.Sum256(inB[:]) {
				kernelPairOK = false
				log.Printf("risk kernel: 2-lane SELF-TEST FAILED on case %d — pair path disabled", i)
				break
			}
		}
	}
	if kernelPairOK {
		riskSumPair = kernelSum64Pair // final say: raceKernelPairing in main()
		// Fused iteration (hash + in-asm hex, precomputed pad schedule):
		// enable only if IT TOO is bit-identical to the composed reference.
		fusedOK := true
		var a, b, wantA, wantB [64]byte
		var sa, sb [32]byte
		for i := 0; i < 512; i++ {
			rnd.Read(a[:])
			rnd.Read(b[:])
			wantA, wantB = a, b
			kernelSum64Pair(&wantA, &wantB, &sa, &sb)
			var ea, eb [64]byte
			hexEncode64(&ea, &sa)
			hexEncode64(&eb, &sb)
			pairHashHex(&a, &b)
			if a != ea || b != eb {
				fusedOK = false
				log.Printf("risk kernel: FUSED-ITER SELF-TEST FAILED on case %d — unfused pair path kept", i)
				break
			}
		}
		if fusedOK {
			riskPairIter = pairHashHex
			// Single-lane fused variant: same self-test discipline.
			singleOK := true
			var s1 [32]byte
			var e1, x1 [64]byte
			for i := 0; i < 512; i++ {
				rnd.Read(x1[:])
				c := x1
				kernelSum64(&c, &s1)
				hexEncode64(&e1, &s1)
				hashHex1(&x1)
				if x1 != e1 {
					singleOK = false
					log.Printf("risk kernel: SINGLE-FUSED SELF-TEST FAILED on case %d — unfused single path kept", i)
					break
				}
			}
			if singleOK {
				riskIter1 = hashHex1
			}
			// Kernel v3 (loop-in-asm N variants): OFF by default — Tier-0
			// on c7i (Xeon 8488C) measured 114.9ns/pair-iter vs v2's 114.1:
			// the per-iteration call + store/load/flip overhead the loop
			// deletes was already fully hidden behind the second lane's SHA
			// dependency chain. Kept behind RISK_KERNEL_V3=on for A/B on
			// other silicon; differential wall still runs in the test suite.
			if singleOK && os.Getenv("RISK_KERNEL_V3") == "on" {
				v3OK := true
				for _, n := range []int{1, 2, 3, 17, 256} {
					for c := 0; c < 128 && v3OK; c++ {
						rnd.Read(a[:])
						rnd.Read(b[:])
						wantA, wantB = a, b
						for k := 0; k < n; k++ {
							pairHashHex(&wantA, &wantB)
						}
						ga, gb := a, b
						pairHashHexN(&ga, &gb, n)
						if ga != wantA || gb != wantB {
							v3OK = false
							log.Printf("risk kernel: V3 PAIR SELF-TEST FAILED (n=%d case %d) — v2 per-iteration path kept", n, c)
						}
						w1 := a
						for k := 0; k < n; k++ {
							hashHex1(&w1)
						}
						g1 := a
						hashHex1N(&g1, n)
						if g1 != w1 {
							v3OK = false
							log.Printf("risk kernel: V3 SINGLE SELF-TEST FAILED (n=%d case %d) — v2 per-iteration path kept", n, c)
						}
					}
				}
				if v3OK {
					riskPairIterN = pairHashHexN
					riskIter1N = hashHex1N
				}
			}
		}
	}
	log.Printf("risk kernel: direct 2-block %s kernel enabled (self-test passed; 2-lane pair=%v fused=%v v3loop=%v)",
		path, kernelPairOK, riskPairIter != nil, riskPairIterN != nil)
}

// kernelPairOK: the interleaved 2-lane path passed its boot self-test.
var kernelPairOK bool

// cpuinfoFlags returns the x86 feature-flag line padded with spaces for
// whole-word matching, or "" if unreadable (kernel stays off — safe).
func cpuinfoFlags() string {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "flags") {
			if i := strings.IndexByte(line, ':'); i >= 0 {
				return " " + strings.TrimSpace(line[i+1:]) + " "
			}
		}
	}
	return ""
}
