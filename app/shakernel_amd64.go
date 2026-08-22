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
	if os.Getenv("RISK_KERNEL") == "off" {
		log.Printf("risk kernel: disabled by RISK_KERNEL=off (stdlib path)")
		return
	}
	flags := cpuinfoFlags()
	has := func(f string) bool { return strings.Contains(flags, " "+f+" ") }
	// Exactly the stdlib's own dispatch gates for these routines.
	kernelUseSHANI = has("sha_ni") && has("avx") && has("sse4_1") && has("ssse3")
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
	}
	log.Printf("risk kernel: direct 2-block %s kernel enabled (self-test passed; 2-lane pair available=%v)", path, kernelPairOK)
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
