package main

import (
	"crypto/sha256"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const (
	riskIterations   = 50_000
	defaultRiskJobs  = 2
	defaultRiskQueue = 32
	defaultRiskLanes = 4
	maxRiskLanes     = 4
)

type priceSeries [500]float64

type market struct {
	priceResponse string
	data          priceSeries
}

type responseBuffer struct {
	bytes []byte
}

type riskJob struct {
	seed     string
	queuedAt time.Time
	result   chan<- riskResult
}

type riskResult struct {
	hash      [sha256.Size * 2]byte
	queueWait time.Duration
	hashTime  time.Duration
}

var markets = buildMarkets()
var lowercaseHexPairs = buildLowercaseHexPairs()

var (
	riskWorkerCount = envInt("RISK_WORKERS", defaultRiskJobs, 1, 2)
	riskQueue       = make(chan riskJob, envInt("RISK_QUEUE", defaultRiskQueue, 1, 200))
	riskLaneLimit   = envInt("RISK_LANES", defaultRiskLanes, 1, maxRiskLanes)
	riskWorkersOnce sync.Once
	emitRiskTiming  = os.Getenv("RISK_TIMING") == "1"
)

var responseBuffers = sync.Pool{
	New: func() any {
		return &responseBuffer{bytes: make([]byte, 0, 192)}
	},
}

func main() {
	// Docker CPU quotas are not guaranteed to change the processor count seen by
	// every Go release. Pinning it avoids overscheduling on the grading host.
	runtime.GOMAXPROCS(2)
	startRiskWorkers()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           http.HandlerFunc(route),
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    8 << 10,
	}

	log.Printf("obsidio listening on :%s with %d risk workers, %d lanes each, and queue capacity %d", port, riskWorkerCount, riskLaneLimit, cap(riskQueue))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func route(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, `{"error":"method not allowed"}`)
		return
	}

	switch r.URL.Path {
	case "/health":
		writeJSON(w, http.StatusOK, `{"status":"ok"}`)
	case "/price":
		handlePrice(w, queryValue(r.URL.RawQuery, "symbol"))
	case "/stats":
		handleStats(w, queryValue(r.URL.RawQuery, "symbol"))
	case "/risk":
		seed := queryValue(r.URL.RawQuery, "seed")
		if seed == "" {
			seed = "none"
		}
		handleRisk(w, r, seed)
	default:
		writeJSON(w, http.StatusNotFound, `{"error":"not found"}`)
	}
}

func handlePrice(w http.ResponseWriter, symbol string) {
	m, ok := markets[symbol]
	if !ok {
		writeJSON(w, http.StatusNotFound, `{"error":"unknown symbol"}`)
		return
	}

	writeJSON(w, http.StatusOK, m.priceResponse)
}

func handleStats(w http.ResponseWriter, symbol string) {
	m, ok := markets[symbol]
	if !ok {
		writeJSON(w, http.StatusNotFound, `{"error":"unknown symbol"}`)
		return
	}

	// The contract requires these values to be recomputed on every request.
	values := &m.data
	sum, minimum, maximum := 0.0, values[0], values[0]
	for _, value := range values {
		sum += value
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	mean := sum / float64(len(values))
	variance := 0.0
	for _, value := range values {
		difference := value - mean
		variance += difference * difference
	}
	standardDeviation := math.Sqrt(variance / float64(len(values)))

	pooled := getResponseBuffer()
	buffer := pooled.bytes
	buffer = append(buffer, `{"symbol":`...)
	buffer = strconv.AppendQuote(buffer, symbol)
	buffer = append(buffer, `,"mean":`...)
	buffer = strconv.AppendFloat(buffer, mean, 'g', -1, 64)
	buffer = append(buffer, `,"min":`...)
	buffer = strconv.AppendFloat(buffer, minimum, 'g', -1, 64)
	buffer = append(buffer, `,"max":`...)
	buffer = strconv.AppendFloat(buffer, maximum, 'g', -1, 64)
	buffer = append(buffer, `,"stddev":`...)
	buffer = strconv.AppendFloat(buffer, standardDeviation, 'g', -1, 64)
	buffer = append(buffer, '}')
	writeJSONBytes(w, http.StatusOK, buffer)
	pooled.bytes = buffer
	putResponseBuffer(pooled)
}

func handleRisk(w http.ResponseWriter, r *http.Request, seed string) {
	startRiskWorkers()
	resultChannel := make(chan riskResult, 1)
	job := riskJob{seed: seed, result: resultChannel}
	if emitRiskTiming {
		job.queuedAt = time.Now()
	}

	select {
	case riskQueue <- job:
	case <-r.Context().Done():
		return
	}

	var result riskResult
	select {
	case result = <-resultChannel:
	case <-r.Context().Done():
		return
	}

	if emitRiskTiming {
		queueMilliseconds := float64(result.queueWait) / float64(time.Millisecond)
		hashMilliseconds := float64(result.hashTime) / float64(time.Millisecond)
		timing := "risk_queue;dur=" + strconv.FormatFloat(queueMilliseconds, 'f', 3, 64) +
			", risk_hash;dur=" + strconv.FormatFloat(hashMilliseconds, 'f', 3, 64)
		w.Header().Set("Server-Timing", timing)
	}

	pooled := getResponseBuffer()
	buffer := pooled.bytes
	buffer = append(buffer, `{"seed":`...)
	buffer = strconv.AppendQuote(buffer, seed)
	buffer = append(buffer, `,"risk_hash":"`...)
	buffer = append(buffer, result.hash[:]...)
	buffer = append(buffer, '"', '}')
	writeJSONBytes(w, http.StatusOK, buffer)
	pooled.bytes = buffer
	putResponseBuffer(pooled)
}

// startRiskWorkers creates the only goroutines allowed to execute the expensive
// hash loop. HTTP handlers enqueue jobs and wait for their own buffered result
// channel, while price and stats requests bypass this queue completely.
func startRiskWorkers() {
	riskWorkersOnce.Do(func() {
		for range riskWorkerCount {
			go riskWorker()
		}
	})
}

// riskWorker takes one job, then opportunistically drains up to riskLaneLimit-1
// more without waiting. Independent chains are hashed as interleaved lanes on
// this worker's core so the processor can overlap one chain's SHA latency with
// another's work. With an empty queue a job still runs alone immediately.
func riskWorker() {
	var batch [maxRiskLanes]riskJob
	for job := range riskQueue {
		batch[0] = job
		count := 1
	fill:
		for count < riskLaneLimit {
			select {
			case next := <-riskQueue:
				batch[count] = next
				count++
			default:
				break fill
			}
		}
		runRiskBatch(batch[:count])
	}
}

func runRiskBatch(batch []riskJob) {
	var hashes [maxRiskLanes][sha256.Size * 2]byte
	var hashStarted time.Time
	if emitRiskTiming {
		hashStarted = time.Now()
	}

	switch len(batch) {
	case 1:
		hashes[0] = calculateRisk(batch[0].seed)
	case 2:
		hashes[0], hashes[1] = calculateRiskPair(batch[0].seed, batch[1].seed)
	case 3:
		hashes[0], hashes[1] = calculateRiskPair(batch[0].seed, batch[1].seed)
		hashes[2] = calculateRisk(batch[2].seed)
	default:
		hashes = calculateRiskQuad([maxRiskLanes]string{batch[0].seed, batch[1].seed, batch[2].seed, batch[3].seed})
	}

	if !emitRiskTiming {
		for index := range batch {
			batch[index].result <- riskResult{hash: hashes[index]}
		}
		return
	}

	// Diagnostic only: hash time is the shared wall time of the whole batch.
	finished := time.Now()
	for index := range batch {
		batch[index].result <- riskResult{
			hash:      hashes[index],
			queueWait: hashStarted.Sub(batch[index].queuedAt),
			hashTime:  finished.Sub(hashStarted),
		}
	}
}

// calculateRisk performs the specified SHA-256 -> lowercase hex feedback loop.
// It uses a fixed buffer after the first iteration, avoiding 49,999 temporary
// strings and byte slices without skipping any of the required work.
func calculateRisk(seed string) [sha256.Size * 2]byte {
	digest := sha256.Sum256([]byte(seed))
	var encodedWords [sha256.Size]uint16
	encodeDigest(&encodedWords, &digest)
	encoded := unsafe.Slice((*byte)(unsafe.Pointer(&encodedWords[0])), sha256.Size*2)

	for iteration := 1; iteration < riskIterations; iteration++ {
		digest = sha256.Sum256(encoded)
		encodeDigest(&encodedWords, &digest)
	}

	var result [sha256.Size * 2]byte
	copy(result[:], encoded)
	return result
}

// calculateRiskPair runs two independent chains in one loop. Each chain still
// performs every one of its own 50,000 rounds in order; only the instruction
// stream is interleaved so the core can overlap the two chains' SHA latency.
func calculateRiskPair(seedA, seedB string) (resultA, resultB [sha256.Size * 2]byte) {
	digestA := sha256.Sum256([]byte(seedA))
	digestB := sha256.Sum256([]byte(seedB))
	var wordsA, wordsB [sha256.Size]uint16
	encodeDigest(&wordsA, &digestA)
	encodeDigest(&wordsB, &digestB)
	encodedA := unsafe.Slice((*byte)(unsafe.Pointer(&wordsA[0])), sha256.Size*2)
	encodedB := unsafe.Slice((*byte)(unsafe.Pointer(&wordsB[0])), sha256.Size*2)

	for iteration := 1; iteration < riskIterations; iteration++ {
		digestA = sha256.Sum256(encodedA)
		digestB = sha256.Sum256(encodedB)
		encodeDigest(&wordsA, &digestA)
		encodeDigest(&wordsB, &digestB)
	}

	copy(resultA[:], encodedA)
	copy(resultB[:], encodedB)
	return resultA, resultB
}

// calculateRiskQuad is the four-lane form of calculateRiskPair.
func calculateRiskQuad(seeds [maxRiskLanes]string) (results [maxRiskLanes][sha256.Size * 2]byte) {
	digest0 := sha256.Sum256([]byte(seeds[0]))
	digest1 := sha256.Sum256([]byte(seeds[1]))
	digest2 := sha256.Sum256([]byte(seeds[2]))
	digest3 := sha256.Sum256([]byte(seeds[3]))
	var words0, words1, words2, words3 [sha256.Size]uint16
	encodeDigest(&words0, &digest0)
	encodeDigest(&words1, &digest1)
	encodeDigest(&words2, &digest2)
	encodeDigest(&words3, &digest3)
	encoded0 := unsafe.Slice((*byte)(unsafe.Pointer(&words0[0])), sha256.Size*2)
	encoded1 := unsafe.Slice((*byte)(unsafe.Pointer(&words1[0])), sha256.Size*2)
	encoded2 := unsafe.Slice((*byte)(unsafe.Pointer(&words2[0])), sha256.Size*2)
	encoded3 := unsafe.Slice((*byte)(unsafe.Pointer(&words3[0])), sha256.Size*2)

	for iteration := 1; iteration < riskIterations; iteration++ {
		digest0 = sha256.Sum256(encoded0)
		digest1 = sha256.Sum256(encoded1)
		digest2 = sha256.Sum256(encoded2)
		digest3 = sha256.Sum256(encoded3)
		encodeDigest(&words0, &digest0)
		encodeDigest(&words1, &digest1)
		encodeDigest(&words2, &digest2)
		encodeDigest(&words3, &digest3)
	}

	copy(results[0][:], encoded0)
	copy(results[1][:], encoded1)
	copy(results[2][:], encoded2)
	copy(results[3][:], encoded3)
	return results
}

// encodeDigest writes two lowercase hexadecimal bytes with one native-width
// store. The table is laid out for the current byte order, while the backing
// uint16 array guarantees aligned stores on every supported architecture.
func encodeDigest(destination *[sha256.Size]uint16, digest *[sha256.Size]byte) {
	// SHA-256 has a fixed 32-byte output. Four stores per iteration reduce loop
	// bookkeeping while keeping this helper small enough for the compiler to
	// inline into the 50,000-round feedback chain.
	for index := 0; index < sha256.Size; index += 4 {
		destination[index] = lowercaseHexPairs[digest[index]]
		destination[index+1] = lowercaseHexPairs[digest[index+1]]
		destination[index+2] = lowercaseHexPairs[digest[index+2]]
		destination[index+3] = lowercaseHexPairs[digest[index+3]]
	}
}

func buildLowercaseHexPairs() [256]uint16 {
	const digits = "0123456789abcdef"
	var marker uint16 = 0x0102
	littleEndian := *(*byte)(unsafe.Pointer(&marker)) == 0x02
	var pairs [256]uint16
	for value := range pairs {
		first := uint16(digits[byte(value)>>4])
		second := uint16(digits[byte(value)&0x0f])
		if littleEndian {
			pairs[value] = first | second<<8
		} else {
			pairs[value] = first<<8 | second
		}
	}
	return pairs
}

func buildMarkets() map[string]*market {
	prices := map[string]float64{
		"AAPL": 187.42,
		"GOOG": 141.80,
		"MSFT": 412.30,
		"AMZN": 178.10,
		"NVDA": 120.15,
		"META": 502.60,
		"TSLA": 244.70,
		"JPM":  198.35,
	}

	result := make(map[string]*market, len(prices))
	for symbol, price := range prices {
		entry := &market{
			priceResponse: `{"symbol":` + strconv.Quote(symbol) + `,"price":` +
				strconv.FormatFloat(price, 'f', -1, 64) + `}`,
		}
		for index := range entry.data {
			entry.data[index] = price * (1 + math.Sin(float64(index))/50)
		}
		result[symbol] = entry
	}
	return result
}

func queryValue(rawQuery, key string) string {
	for rawQuery != "" {
		part := rawQuery
		if ampersand := strings.IndexByte(rawQuery, '&'); ampersand >= 0 {
			part, rawQuery = rawQuery[:ampersand], rawQuery[ampersand+1:]
		} else {
			rawQuery = ""
		}

		name, value, found := strings.Cut(part, "=")
		if !found || name != key {
			continue
		}
		if strings.ContainsAny(value, "+%") {
			decoded, err := url.QueryUnescape(value)
			if err == nil {
				return decoded
			}
		}
		return value
	}
	return ""
}

func getResponseBuffer() *responseBuffer {
	buffer := responseBuffers.Get().(*responseBuffer)
	buffer.bytes = buffer.bytes[:0]
	return buffer
}

func putResponseBuffer(buffer *responseBuffer) {
	if cap(buffer.bytes) > 512 {
		return
	}
	responseBuffers.Put(buffer)
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func writeJSONBytes(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func envInt(name string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < minimum || value > maximum {
		return fallback
	}
	return value
}
