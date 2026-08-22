//go:build amd64 && !purego

package main

import (
	"crypto/sha256"
	"encoding/binary"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// Sixteen-lane AVX-512 SHA-256: insurance for an x86-64 grader that has
// AVX-512 but no SHA extensions (Skylake-SP and Cascade Lake class). The
// assembly and its round table are vendored unchanged from
// github.com/minio/sha256-simd v1.0.1 (Apache-2.0; see
// LICENSE.minio-sha256-simd and sha256x16_amd64.s). Every chain round hashes
// exactly one 64-byte message block plus the constant padding block, and all
// sixteen lanes run in lockstep, so one call with a two-round all-lanes mask
// advances sixteen independent chains by one round.
//
// Where SHA extensions exist the fused single-core kernels in risk_amd64.s
// win, so this path stays off there unless RISK_X16=on. It is enabled only
// after a boot differential self-test against crypto/sha256 and a timed race
// against the path it would displace; RISK_X16=off is the kill switch.

// sha256X16Avx512 advances sixteen transposed digests through the
// mask-selected 64-byte blocks of each lane's input: block r of lane i is
// inputs[i][64r:64r+64] and is processed when bit i of mask[r] is set.
//
//go:noescape
func sha256X16Avx512(digests, scratch *[512]byte, table *[512]uint64, mask []uint64, inputs [x16Lanes][]byte)

const (
	x16SelfTestCases    = 64
	x16ChainCheckRounds = 300
	x16RaceBarInsurance = 13 // batch must beat the portable path by ≥30%
	x16RaceBarForced    = 11 // with SHA-NI live, by ≥10% (A/B use only)
)

var (
	// riskX16Enabled is decided once by initRiskX16 before the workers start.
	riskX16Enabled  bool
	riskX16MinBatch = x16Lanes
)

var sha256InitialHash = [8]uint32{
	0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
	0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
}

// sha256Padding64 is the second block of a 64-byte message: the 0x80 marker,
// zeros, and the 64-bit big-endian bit length 512.
var sha256Padding64 = func() (padding [64]byte) {
	padding[0] = 0x80
	padding[62] = 0x02
	return padding
}()

// x16InitialState holds the transposed starting digests: word w of lane i
// is a little-endian uint32 at offset 4i + 64w.
var x16InitialState = func() (state [512]byte) {
	for lane := range x16Lanes {
		for word, value := range sha256InitialHash {
			binary.LittleEndian.PutUint32(state[lane*4+word*64:], value)
		}
	}
	return state
}()

// x16MaskAllLanes selects every lane for both block rounds.
var x16MaskAllLanes = []uint64{0xffff, 0xffff}

// x16Lane is one lane's 128-byte input: the chain's current value (64
// lowercase-hex bytes, which is also the message block) followed by the
// constant padding block. The uint16 view gives encodeDigest aligned stores.
type x16Lane struct {
	hex [sha256.Size]uint16
	pad [64]byte
}

func (lane *x16Lane) message() *[sha256.Size * 2]byte {
	return (*[sha256.Size * 2]byte)(unsafe.Pointer(&lane.hex[0]))
}

type x16Chains struct {
	digests [512]byte
	scratch [512]byte
	lanes   [x16Lanes]x16Lane
	inputs  [x16Lanes][]byte
}

var x16Pool = sync.Pool{New: func() any {
	chains := &x16Chains{}
	for lane := range chains.lanes {
		chains.lanes[lane].pad = sha256Padding64
		chains.inputs[lane] = unsafe.Slice((*byte)(unsafe.Pointer(&chains.lanes[lane])), int(unsafe.Sizeof(x16Lane{})))
	}
	return chains
}}

// hashOnce hashes every lane's message block once, leaving the transposed
// digests in place.
func (chains *x16Chains) hashOnce() {
	chains.digests = x16InitialState
	chains.scratch = [512]byte{}
	sha256X16Avx512(&chains.digests, &chains.scratch, &x16RoundTable, x16MaskAllLanes, chains.inputs)
}

func (chains *x16Chains) digest(lane int, digest *[sha256.Size]byte) {
	for word := range 8 {
		binary.BigEndian.PutUint32(digest[4*word:], binary.LittleEndian.Uint32(chains.digests[lane*4+word*64:]))
	}
}

// step advances all sixteen chains by one round: each lane's 64 hex bytes
// are replaced by the lowercase hex of their SHA-256.
func (chains *x16Chains) step() {
	chains.hashOnce()
	var digest [sha256.Size]byte
	for lane := range chains.lanes {
		chains.digest(lane, &digest)
		encodeDigest(&chains.lanes[lane].hex, &digest)
	}
}

// x16Sum64 hashes sixteen independent 64-byte inputs; the differential
// tests compare it with crypto/sha256.
func x16Sum64(inputs *[x16Lanes][sha256.Size * 2]byte, outputs *[x16Lanes][sha256.Size]byte) {
	chains := x16Pool.Get().(*x16Chains)
	for lane := range inputs {
		*chains.lanes[lane].message() = inputs[lane]
	}
	chains.hashOnce()
	for lane := range outputs {
		chains.digest(lane, &outputs[lane])
	}
	x16Pool.Put(chains)
}

// x16ChainRun runs up to sixteen complete chains in lockstep and writes each
// seed's final hex digest to results. Unused lanes hash zeros: the vector
// unit computes every lane regardless, so masking would save nothing. The
// yield cadence is the scalar kernels' (one step is one round of every lane).
func x16ChainRun(seeds []string, results *[x16Lanes][sha256.Size * 2]byte) {
	chains := x16Pool.Get().(*x16Chains)
	for lane := range chains.lanes {
		if lane < len(seeds) {
			digest := sha256.Sum256([]byte(seeds[lane]))
			encodeDigest(&chains.lanes[lane].hex, &digest)
		} else {
			chains.lanes[lane].hex = [sha256.Size]uint16{}
		}
	}
	for remaining := riskIterations - 1; remaining > 0; {
		rounds := yieldChunk(remaining)
		for range rounds {
			chains.step()
		}
		remaining -= rounds
		yieldAfterChunk(remaining)
	}
	for lane := range seeds {
		results[lane] = *chains.lanes[lane].message()
	}
	x16Pool.Put(chains)
}

// runRiskBatchX16 hashes a worker's batch of up to sixteen jobs through the
// sixteen-lane kernel and delivers the results exactly as runRiskBatch does.
func runRiskBatchX16(batch []*riskJob) {
	var seeds [x16Lanes]string
	for index := range batch {
		seeds[index] = batch[index].seed
	}
	var hashes [x16Lanes][sha256.Size * 2]byte
	hashStarted := riskTimingStart()
	x16ChainRun(seeds[:len(batch)], &hashes)
	deliverRiskResults(batch, hashes[:], hashStarted)
}

// cpuHasAVX512 reports the feature set the vendored routine requires, read
// from the kernel's own view so that operating-system support is implied.
func cpuHasAVX512() bool {
	flags := cpuinfoFlags()
	for _, flag := range []string{"avx512f", "avx512dq", "avx512bw", "avx512vl"} {
		if !strings.Contains(flags, " "+flag+" ") {
			return false
		}
	}
	return true
}

// cpuinfoFlags returns the x86 feature-flag line padded with spaces for
// whole-word matching, or "" when unreadable (the path stays off).
func cpuinfoFlags() string {
	content, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "flags") {
			if colon := strings.IndexByte(line, ':'); colon >= 0 {
				return " " + strings.TrimSpace(line[colon+1:]) + " "
			}
		}
	}
	return ""
}

// x16SelfTest compares the kernel with crypto/sha256 on random inputs
// (including equal lanes, which must not leak into each other) and runs a
// short chain on every lane against the portable feedback loop.
func x16SelfTest() bool {
	random := rand.New(rand.NewSource(0xa5f31600))
	var inputs [x16Lanes][sha256.Size * 2]byte
	var outputs [x16Lanes][sha256.Size]byte
	for testCase := range x16SelfTestCases {
		for lane := range inputs {
			random.Read(inputs[lane][:])
		}
		if testCase%8 == 7 {
			inputs[3] = inputs[12]
		}
		x16Sum64(&inputs, &outputs)
		for lane := range inputs {
			if outputs[lane] != sha256.Sum256(inputs[lane][:]) {
				return false
			}
		}
	}

	chains := x16Pool.Get().(*x16Chains)
	defer x16Pool.Put(chains)
	var want [x16Lanes][sha256.Size * 2]byte
	for lane := range chains.lanes {
		digest := sha256.Sum256([]byte("x16-boot-" + strconv.Itoa(lane)))
		encodeDigest(&chains.lanes[lane].hex, &digest)
		var words [sha256.Size]uint16
		encodeDigest(&words, &digest)
		for range x16ChainCheckRounds {
			digest = sha256.Sum256(unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), sha256.Size*2))
			encodeDigest(&words, &digest)
		}
		want[lane] = *(*[sha256.Size * 2]byte)(unsafe.Pointer(&words[0]))
	}
	for range x16ChainCheckRounds {
		chains.step()
	}
	for lane := range chains.lanes {
		if *chains.lanes[lane].message() != want[lane] {
			return false
		}
	}
	return true
}

// x16BootRace times one complete sixteen-chain batch against the path it
// would displace, sized to sixteen chains, and cross-checks the digests. It
// returns whether the batch clears the bar, the measured speed-up, and the
// smallest batch worth taking: a batch of k costs the full batch time, so k
// must exceed batch time over 90% of the displaced per-chain cost.
func x16BootRace() (ok bool, ratio float64, minBatch int) {
	var seeds [x16Lanes]string
	for lane := range seeds {
		seeds[lane] = "x16-race-" + strconv.Itoa(lane)
	}
	var batchHashes [x16Lanes][sha256.Size * 2]byte
	started := time.Now()
	x16ChainRun(seeds[:], &batchHashes)
	batch := time.Since(started)

	var displaced time.Duration
	started = time.Now()
	if useSHANIPair {
		for offset := 0; offset < x16Lanes; offset += maxRiskLanes {
			quad := calculateRiskQuad([maxRiskLanes]string{seeds[offset], seeds[offset+1], seeds[offset+2], seeds[offset+3]})
			for lane := range quad {
				if quad[lane] != batchHashes[offset+lane] {
					return false, 0, 0
				}
			}
		}
		displaced = time.Since(started)
	} else {
		for lane := range maxRiskLanes {
			if calculateRisk(seeds[lane]) != batchHashes[lane] {
				return false, 0, 0
			}
		}
		displaced = time.Since(started) * (x16Lanes / maxRiskLanes)
	}

	ratio = float64(displaced) / float64(batch)
	bar := int64(x16RaceBarInsurance)
	if useSHANIPair {
		bar = x16RaceBarForced
	}
	if batch.Nanoseconds()*bar >= displaced.Nanoseconds()*10 {
		return false, ratio, 0
	}
	perChain := displaced.Nanoseconds() / x16Lanes
	minBatch = x16Lanes
	if perChain > 0 {
		minBatch = int(batch.Nanoseconds()*10/(perChain*9)) + 1
	}
	return true, ratio, min(max(minBatch, maxRiskLanes), x16Lanes)
}

// initRiskX16 enables the sixteen-lane batch path on a processor with
// AVX-512 and no SHA extensions (or with RISK_X16=on for A/B runs) once the
// self-test passes and the boot race shows a real win. Called after
// calibratePatience, so the displaced path is warm.
func initRiskX16() {
	mode := os.Getenv("RISK_X16")
	if mode == "off" {
		log.Print("risk x16: disabled by RISK_X16=off")
		return
	}
	if !cpuHasAVX512() {
		return
	}
	if useSHANIPair && mode != "on" {
		log.Print("risk x16: AVX-512 present, SHA-NI kernels preferred (RISK_X16=on to race them)")
		return
	}
	if !x16SelfTest() {
		log.Print("risk x16: self-test FAILED; path disabled")
		return
	}
	ok, ratio, minBatch := x16BootRace()
	if !ok {
		log.Printf("risk x16: disabled by boot race (%.2fx over the displaced path)", ratio)
		return
	}
	riskX16MinBatch = minBatch
	riskX16Enabled = true
	log.Printf("risk x16: sixteen-lane AVX-512 batches enabled (%.2fx over the displaced path, minimum batch %d)", ratio, minBatch)
}
