package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

func TestHealth(t *testing.T) {
	response := request(t, "/health")
	if response.Code != http.StatusOK || response.Body.String() != `{"status":"ok"}` {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestPrice(t *testing.T) {
	response := request(t, "/price?symbol=AAPL")
	if response.Code != http.StatusOK || response.Body.String() != `{"symbol":"AAPL","price":187.42}` {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestUnknownSymbol(t *testing.T) {
	for _, path := range []string{"/price?symbol=NOPE", "/stats?symbol=NOPE"} {
		response := request(t, path)
		if response.Code != http.StatusNotFound || response.Body.String() != `{"error":"unknown symbol"}` {
			t.Fatalf("unexpected response for %s: %d %s", path, response.Code, response.Body.String())
		}
	}
}

func TestStats(t *testing.T) {
	response := request(t, "/stats?symbol=MSFT")
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}

	var body struct {
		Symbol string  `json:"symbol"`
		Mean   float64 `json:"mean"`
		Min    float64 `json:"min"`
		Max    float64 `json:"max"`
		Stddev float64 `json:"stddev"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Symbol != "MSFT" {
		t.Fatalf("unexpected symbol: %s", body.Symbol)
	}

	expected := referenceStats(412.30)
	assertNear(t, "mean", body.Mean, expected[0])
	assertNear(t, "min", body.Min, expected[1])
	assertNear(t, "max", body.Max, expected[2])
	assertNear(t, "stddev", body.Stddev, expected[3])
}

func TestRisk(t *testing.T) {
	for _, seed := range []string{"0.48", "hello world", "symbols: +/%"} {
		got := calculateRisk(seed)
		want := referenceRisk(seed)
		if string(got[:]) != want {
			t.Fatalf("wrong risk hash for %q:\n got %s\nwant %s", seed, got, want)
		}
	}
}

func TestPackedHexEncoding(t *testing.T) {
	for value := range 256 {
		var digest [sha256.Size]byte
		for index := range digest {
			digest[index] = byte(value)
		}
		var words [sha256.Size]uint16
		encodeDigest(&words, &digest)
		got := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), sha256.Size*2)
		want := make([]byte, sha256.Size*2)
		hex.Encode(want, digest[:])
		if !bytes.Equal(got, want) {
			t.Fatalf("wrong encoding for byte %#x: got %q, want %q", value, got, want)
		}
	}
}

func TestRiskEndpointEscapesSeed(t *testing.T) {
	seed := `quote"and\\slash`
	response := request(t, "/risk?seed="+urlQueryEscape(seed))
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}

	var body struct {
		Seed     string `json:"seed"`
		RiskHash string `json:"risk_hash"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Seed != seed || body.RiskHash != referenceRisk(seed) {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestRiskTimingIsOptional(t *testing.T) {
	previous := emitRiskTiming
	emitRiskTiming = true
	defer func() { emitRiskTiming = previous }()

	response := request(t, "/risk?seed=timed")
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	timing := response.Header().Get("Server-Timing")
	if !strings.Contains(timing, "risk_queue;dur=") || !strings.Contains(timing, "risk_hash;dur=") {
		t.Fatalf("unexpected Server-Timing header: %q", timing)
	}
}

func TestConcurrentRiskRequests(t *testing.T) {
	type completedRequest struct {
		index  int
		status int
		body   string
	}

	const requestCount = 8
	expected := make([]string, requestCount)
	completed := make(chan completedRequest, requestCount)
	for index := range requestCount {
		seed := "concurrent-" + strconv.Itoa(index)
		expected[index] = referenceRisk(seed)
		go func() {
			response := httptest.NewRecorder()
			route(response, httptest.NewRequest(http.MethodGet, "/risk?seed="+seed, nil))
			completed <- completedRequest{index: index, status: response.Code, body: response.Body.String()}
		}()
	}

	for range requestCount {
		result := <-completed
		if result.status != http.StatusOK {
			t.Fatalf("request %d returned status %d", result.index, result.status)
		}
		var body struct {
			RiskHash string `json:"risk_hash"`
		}
		if err := json.Unmarshal([]byte(result.body), &body); err != nil {
			t.Fatalf("request %d returned invalid JSON: %v", result.index, err)
		}
		if body.RiskHash != expected[result.index] {
			t.Fatalf("request %d returned the wrong hash", result.index)
		}
	}
}

func BenchmarkPrice(b *testing.B) {
	request := httptest.NewRequest(http.MethodGet, "/price?symbol=AAPL", nil)
	for index := 0; index < b.N; index++ {
		response := httptest.NewRecorder()
		route(response, request)
	}
}

func BenchmarkStats(b *testing.B) {
	request := httptest.NewRequest(http.MethodGet, "/stats?symbol=AAPL", nil)
	for index := 0; index < b.N; index++ {
		response := httptest.NewRecorder()
		route(response, request)
	}
}

func BenchmarkRisk(b *testing.B) {
	for index := 0; index < b.N; index++ {
		_ = calculateRisk(strconv.Itoa(index))
	}
}

func BenchmarkRiskStandardHex(b *testing.B) {
	for index := 0; index < b.N; index++ {
		_ = calculateRiskStandardHex(strconv.Itoa(index))
	}
}

func BenchmarkRiskStarter(b *testing.B) {
	for index := 0; index < b.N; index++ {
		_ = referenceRisk(strconv.Itoa(index))
	}
}

var benchmarkHexSink uint16

func BenchmarkHexStandard(b *testing.B) {
	digest := sha256.Sum256([]byte("benchmark"))
	var encoded [sha256.Size * 2]byte
	for b.Loop() {
		hex.Encode(encoded[:], digest[:])
	}
	benchmarkHexSink = uint16(encoded[0])
}

func BenchmarkHexPacked(b *testing.B) {
	digest := sha256.Sum256([]byte("benchmark"))
	var encoded [sha256.Size]uint16
	for b.Loop() {
		encodeDigest(&encoded, &digest)
	}
	benchmarkHexSink = encoded[0]
}

func request(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	route(response, httptest.NewRequest(http.MethodGet, path, nil))
	return response
}

func referenceRisk(seed string) string {
	value := seed
	for range riskIterations {
		digest := sha256.Sum256([]byte(value))
		value = hex.EncodeToString(digest[:])
	}
	return value
}

func calculateRiskStandardHex(seed string) [sha256.Size * 2]byte {
	digest := sha256.Sum256([]byte(seed))
	var encoded [sha256.Size * 2]byte
	hex.Encode(encoded[:], digest[:])
	for iteration := 1; iteration < riskIterations; iteration++ {
		digest = sha256.Sum256(encoded[:])
		hex.Encode(encoded[:], digest[:])
	}
	return encoded
}

func referenceStats(price float64) [4]float64 {
	values := make([]float64, 500)
	for index := range values {
		values[index] = price * (1 + math.Sin(float64(index))/50)
	}
	sum, minimum, maximum := 0.0, values[0], values[0]
	for _, value := range values {
		sum += value
		minimum = math.Min(minimum, value)
		maximum = math.Max(maximum, value)
	}
	mean := sum / float64(len(values))
	variance := 0.0
	for _, value := range values {
		variance += (value - mean) * (value - mean)
	}
	return [4]float64{mean, minimum, maximum, math.Sqrt(variance / float64(len(values)))}
}

func assertNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("%s: got %.15f, want %.15f", name, got, want)
	}
}

func urlQueryEscape(value string) string {
	result := ""
	for _, character := range []byte(value) {
		switch {
		case character >= 'a' && character <= 'z', character >= 'A' && character <= 'Z':
			result += string(character)
		default:
			result += "%" + strconv.FormatInt(int64(character), 16)
		}
	}
	return result
}
