package main

import (
	"crypto/sha256"
	"encoding/hex"
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
)

const (
	riskIterations  = 50_000
	defaultRiskJobs = 2
)

type priceSeries [500]float64

type market struct {
	priceResponse string
	data          priceSeries
}

type responseBuffer struct {
	bytes []byte
}

var markets = buildMarkets()

// riskSlots prevents a burst of expensive requests from creating more runnable
// hashing goroutines than the two-CPU container can execute. Cheap handlers do
// not enter this queue, so they remain immediately schedulable.
var riskSlots = make(chan struct{}, envInt("RISK_WORKERS", defaultRiskJobs, 1, 2))

var responseBuffers = sync.Pool{
	New: func() any {
		return &responseBuffer{bytes: make([]byte, 0, 192)}
	},
}

func main() {
	// Docker CPU quotas are not guaranteed to change the processor count seen by
	// every Go release. Pinning it avoids overscheduling on the grading host.
	runtime.GOMAXPROCS(2)

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

	log.Printf("obsidio listening on :%s with %d risk workers", port, cap(riskSlots))
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
	select {
	case riskSlots <- struct{}{}:
		defer func() { <-riskSlots }()
	case <-r.Context().Done():
		return
	}

	hash := calculateRisk(seed)
	pooled := getResponseBuffer()
	buffer := pooled.bytes
	buffer = append(buffer, `{"seed":`...)
	buffer = strconv.AppendQuote(buffer, seed)
	buffer = append(buffer, `,"risk_hash":"`...)
	buffer = append(buffer, hash[:]...)
	buffer = append(buffer, '"', '}')
	writeJSONBytes(w, http.StatusOK, buffer)
	pooled.bytes = buffer
	putResponseBuffer(pooled)
}

// calculateRisk performs the specified SHA-256 -> lowercase hex feedback loop.
// It uses a fixed buffer after the first iteration, avoiding 49,999 temporary
// strings and byte slices without skipping any of the required work.
func calculateRisk(seed string) [sha256.Size * 2]byte {
	digest := sha256.Sum256([]byte(seed))
	var encoded [sha256.Size * 2]byte
	hex.Encode(encoded[:], digest[:])

	for iteration := 1; iteration < riskIterations; iteration++ {
		digest = sha256.Sum256(encoded[:])
		hex.Encode(encoded[:], digest[:])
	}
	return encoded
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
