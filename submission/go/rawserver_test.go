package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The raw server's wire output is validated with http.ReadResponse: the
// standard library parser is the independent reference for framing.

func startRawServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go rawServe(listener)
	t.Cleanup(func() { listener.Close() })
	return listener.Addr().String()
}

func dialRaw(t *testing.T, address string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, bufio.NewReader(conn)
}

func readRawResponse(t *testing.T, reader *bufio.Reader) (int, string, *http.Response) {
	t.Helper()
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("standard library could not parse the raw response: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("body read: %v", err)
	}
	response.Body.Close()
	return response.StatusCode, string(body), response
}

func writeRaw(t *testing.T, conn net.Conn, request string) {
	t.Helper()
	if _, err := conn.Write([]byte(request)); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestRawHealthKeepAlive(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	for index := range 3 {
		writeRaw(t, conn, "GET /health HTTP/1.1\r\nHost: x\r\n\r\n")
		code, body, response := readRawResponse(t, reader)
		if code != http.StatusOK || body != `{"status":"ok"}` {
			t.Fatalf("request %d: %d %q", index, code, body)
		}
		if contentType := response.Header.Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("content-type %q", contentType)
		}
		if response.Close {
			t.Fatalf("request %d: keep-alive connection was marked close", index)
		}
	}
}

func TestRawPriceAndStats(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	writeRaw(t, conn, "GET /price?symbol=AAPL HTTP/1.1\r\nHost: x\r\n\r\n")
	code, body, _ := readRawResponse(t, reader)
	if code != http.StatusOK || body != `{"symbol":"AAPL","price":187.42}` {
		t.Fatalf("price: %d %q", code, body)
	}
	writeRaw(t, conn, "GET /stats?symbol=NVDA HTTP/1.1\r\nHost: x\r\n\r\n")
	code, body, _ = readRawResponse(t, reader)
	if code != http.StatusOK || !strings.Contains(body, `"symbol":"NVDA"`) || !strings.Contains(body, `"stddev":`) {
		t.Fatalf("stats: %d %q", code, body)
	}
	// Every 404 keeps the same shape the net/http handlers produce.
	cases := map[string]string{
		"/price?symbol=NOPE": `{"error":"unknown symbol"}`,
		"/stats?symbol=":     `{"error":"unknown symbol"}`,
		"/price":             `{"error":"unknown symbol"}`,
		"/nope":              `{"error":"not found"}`,
		"/":                  `{"error":"not found"}`,
	}
	for target, want := range cases {
		writeRaw(t, conn, fmt.Sprintf("GET %s HTTP/1.1\r\nHost: x\r\n\r\n", target))
		code, body, _ = readRawResponse(t, reader)
		if code != http.StatusNotFound || body != want {
			t.Fatalf("%s: %d %q, want 404 %q", target, code, body, want)
		}
	}
}

// TestRawMethodNotAllowed covers the generic status-line branch, which the
// four contract statuses never exercise.
func TestRawMethodNotAllowed(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	writeRaw(t, conn, "PUT /price HTTP/1.1\r\nHost: x\r\nContent-Length: 2\r\n\r\n{}")
	code, body, response := readRawResponse(t, reader)
	if code != http.StatusMethodNotAllowed || body != `{"error":"method not allowed"}` {
		t.Fatalf("put: %d %q", code, body)
	}
	if response.Status != "405 Method Not Allowed" {
		t.Fatalf("status line %q", response.Status)
	}
	// The connection is still usable: the body was consumed by framing.
	writeRaw(t, conn, "GET /health HTTP/1.1\r\nHost: x\r\n\r\n")
	if code, body, _ = readRawResponse(t, reader); code != http.StatusOK || body != `{"status":"ok"}` {
		t.Fatalf("health after put: %d %q", code, body)
	}
}

// TestRawSplitReads: a head arriving in arbitrary chunks must parse
// identically (the buffer refill path).
func TestRawSplitReads(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	full := "GET /price?symbol=GOOG HTTP/1.1\r\nHost: x\r\nAccept: */*\r\n\r\n"
	for _, chunk := range []string{full[:3], full[3:20], full[20 : len(full)-2], full[len(full)-2:]} {
		writeRaw(t, conn, chunk)
		time.Sleep(10 * time.Millisecond)
	}
	code, body, _ := readRawResponse(t, reader)
	if code != http.StatusOK || body != `{"symbol":"GOOG","price":141.8}` {
		t.Fatalf("split read: %d %q", code, body)
	}
}

// TestRawPipelining: several requests in one segment are answered in order.
func TestRawPipelining(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	writeRaw(t, conn, "GET /price?symbol=MSFT HTTP/1.1\r\nHost: x\r\n\r\n"+
		"GET /health HTTP/1.1\r\nHost: x\r\n\r\n"+
		"GET /price?symbol=TSLA HTTP/1.1\r\nHost: x\r\n\r\n")
	for index, want := range []string{`{"symbol":"MSFT","price":412.3}`, `{"status":"ok"}`, `{"symbol":"TSLA","price":244.7}`} {
		code, body, _ := readRawResponse(t, reader)
		if code != http.StatusOK || body != want {
			t.Fatalf("pipelined %d: %d %q", index, code, body)
		}
	}
}

// TestRawOversizedHeadCloses: a head that cannot fit the buffer closes the
// connection without any response bytes.
func TestRawOversizedHeadCloses(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	writeRaw(t, conn, "GET /health HTTP/1.1\r\nX-Pad: "+strings.Repeat("a", 5000)+"\r\n\r\n")
	// Unread bytes at close may surface as a reset rather than a clean EOF;
	// the requirement is only: no response byte, connection dead.
	if value, err := reader.ReadByte(); err == nil {
		t.Fatalf("want closed connection, got response byte %q", value)
	}
}

// TestRawOversizedBodyCloses: a declared body larger than the bound closes
// the connection before any of it is read.
func TestRawOversizedBodyCloses(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	writeRaw(t, conn, "POST /price HTTP/1.1\r\nHost: x\r\nContent-Length: 100000\r\n\r\n")
	if value, err := reader.ReadByte(); err == nil {
		t.Fatalf("want closed connection, got response byte %q", value)
	}
}

// TestRawEncodedQuery: percent-encoded values take queryValue's decoding
// path exactly as they do under net/http.
func TestRawEncodedQuery(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	writeRaw(t, conn, "GET /price?symbol=%41APL HTTP/1.1\r\nHost: x\r\n\r\n")
	code, body, _ := readRawResponse(t, reader)
	if code != http.StatusOK || body != `{"symbol":"AAPL","price":187.42}` {
		t.Fatalf("encoded query: %d %q", code, body)
	}
	writeRaw(t, conn, "GET /price?other=1&symbol=AMZN&x=y HTTP/1.1\r\nHost: x\r\n\r\n")
	code, body, _ = readRawResponse(t, reader)
	if code != http.StatusOK || body != `{"symbol":"AMZN","price":178.1}` {
		t.Fatalf("multi-parameter query: %d %q", code, body)
	}
}

// TestRawPostPrice: a POST body framed by Content-Length, arriving split from
// its head, updates the price served by later GETs on the same connection.
func TestRawPostPrice(t *testing.T) {
	defer markets["JPM"].setPrice(198.35)
	conn, reader := dialRaw(t, startRawServer(t))
	requestBody := `{"symbol":"JPM","price":42.5}`
	writeRaw(t, conn, fmt.Sprintf("POST /price HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n", len(requestBody)))
	time.Sleep(10 * time.Millisecond)
	writeRaw(t, conn, requestBody[:10])
	time.Sleep(10 * time.Millisecond)
	writeRaw(t, conn, requestBody[10:])
	code, body, _ := readRawResponse(t, reader)
	if code != http.StatusOK || body != `{"symbol":"JPM","price":42.5}` {
		t.Fatalf("post: %d %q", code, body)
	}
	writeRaw(t, conn, "GET /price?symbol=JPM HTTP/1.1\r\nHost: x\r\n\r\n")
	code, body, _ = readRawResponse(t, reader)
	if code != http.StatusOK || body != `{"symbol":"JPM","price":42.5}` {
		t.Fatalf("get after post: %d %q", code, body)
	}
	// Handler-level validation is unchanged: the raw server only frames.
	cases := map[string]int{
		`{"symbol":"JPM","price":-1}`: http.StatusBadRequest,
		`not json`:                    http.StatusBadRequest,
		`{"symbol":"NOPE","price":1}`: http.StatusNotFound,
	}
	for requestBody, want := range cases {
		writeRaw(t, conn, fmt.Sprintf("POST /price HTTP/1.1\r\nHost: x\r\ncontent-length: %d\r\n\r\n%s", len(requestBody), requestBody))
		if code, body, _ = readRawResponse(t, reader); code != want {
			t.Fatalf("%s: %d %q, want %d", requestBody, code, body, want)
		}
	}
}

// TestRawPipelinedPostThenGet: the body consumed by framing must not be
// mistaken for the next request's head when both arrive in one segment.
func TestRawPipelinedPostThenGet(t *testing.T) {
	defer markets["META"].setPrice(502.60)
	conn, reader := dialRaw(t, startRawServer(t))
	requestBody := `{"symbol":"META","price":500}`
	writeRaw(t, conn, fmt.Sprintf("POST /price HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n%s", len(requestBody), requestBody)+
		"GET /price?symbol=META HTTP/1.1\r\nHost: x\r\n\r\n")
	code, body, _ := readRawResponse(t, reader)
	if code != http.StatusOK || body != `{"symbol":"META","price":500}` {
		t.Fatalf("post: %d %q", code, body)
	}
	code, body, _ = readRawResponse(t, reader)
	if code != http.StatusOK || body != `{"symbol":"META","price":500}` {
		t.Fatalf("pipelined get: %d %q", code, body)
	}
}

// TestRawConnectionClose: Connection: close and HTTP/1.0 both end the
// connection after one response, and the response says so.
func TestRawConnectionClose(t *testing.T) {
	address := startRawServer(t)
	for _, request := range []string{
		"GET /health HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n",
		"GET /health HTTP/1.1\r\nHost: x\r\nconnection: Close\r\n\r\n",
		"GET /health HTTP/1.0\r\n\r\n",
	} {
		conn, reader := dialRaw(t, address)
		writeRaw(t, conn, request)
		code, body, response := readRawResponse(t, reader)
		if code != http.StatusOK || body != `{"status":"ok"}` {
			t.Fatalf("%q: %d %q", request, code, body)
		}
		if !response.Close {
			t.Fatalf("%q: response did not announce Connection: close", request)
		}
		if _, err := reader.ReadByte(); err != io.EOF {
			t.Fatalf("%q: connection stayed open (err=%v)", request, err)
		}
	}
}

// TestRawRejects: chunked bodies and malformed request lines close silently.
func TestRawRejects(t *testing.T) {
	address := startRawServer(t)
	for _, request := range []string{
		"POST /price HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n",
		"GARBAGE\r\n\r\n",
		"GET /health HTTP/2.7\r\n\r\n",
		"GET /health HTTP/1.1\r\nContent-Length: -1\r\n\r\n",
		"GET /health HTTP/1.1\r\nContent-Length: 12345678901\r\n\r\n",
	} {
		conn, reader := dialRaw(t, address)
		writeRaw(t, conn, request)
		if value, err := reader.ReadByte(); err == nil {
			t.Fatalf("%q: want silent close, got byte %q", request, value)
		}
	}
}

// TestRawRiskEndToEnd: a /risk request through the raw server parks at the
// gate, is hashed by the worker pool, and answers with the reference digest.
func TestRawRiskEndToEnd(t *testing.T) {
	conn, reader := dialRaw(t, startRawServer(t))
	seed := `raw"test`
	writeRaw(t, conn, "GET /risk?seed="+urlQueryEscape(seed)+" HTTP/1.1\r\nHost: x\r\n\r\n")
	code, body, _ := readRawResponse(t, reader)
	want := `{"seed":"raw\"test","risk_hash":"` + referenceRisk(seed) + `"}`
	if code != http.StatusOK || body != want {
		t.Fatalf("risk: %d %q want %q", code, body, want)
	}
	writeRaw(t, conn, "GET /risk HTTP/1.1\r\nHost: x\r\n\r\n")
	code, body, _ = readRawResponse(t, reader)
	want = `{"seed":"none","risk_hash":"` + referenceRisk("none") + `"}`
	if code != http.StatusOK || body != want {
		t.Fatalf("risk default seed: %d %q want %q", code, body, want)
	}
}

// TestRawServerTimingPassthrough: a header set by a handler is emitted once
// and not repeated on the connection's later responses.
func TestRawServerTimingPassthrough(t *testing.T) {
	previous := emitRiskTiming
	emitRiskTiming = true
	defer func() { emitRiskTiming = previous }()

	conn, reader := dialRaw(t, startRawServer(t))
	writeRaw(t, conn, "GET /risk?seed=timed HTTP/1.1\r\nHost: x\r\n\r\n")
	code, _, response := readRawResponse(t, reader)
	timing := response.Header.Get("Server-Timing")
	if code != http.StatusOK || !strings.Contains(timing, "risk_queue;dur=") || !strings.Contains(timing, "risk_hash;dur=") {
		t.Fatalf("unexpected Server-Timing header: %d %q", code, timing)
	}
	writeRaw(t, conn, "GET /health HTTP/1.1\r\nHost: x\r\n\r\n")
	if _, _, response = readRawResponse(t, reader); response.Header.Get("Server-Timing") != "" {
		t.Fatal("Server-Timing leaked into the next response")
	}
}

// TestRawConcurrentConnections: independent connections interleaving cheap
// and heavy requests all receive correct, correctly framed responses.
func TestRawConcurrentConnections(t *testing.T) {
	address := startRawServer(t)
	const connections = 6
	errorsCh := make(chan error, connections)
	for index := range connections {
		go func() {
			conn, err := net.Dial("tcp", address)
			if err != nil {
				errorsCh <- err
				return
			}
			defer conn.Close()
			reader := bufio.NewReader(conn)
			seed := fmt.Sprintf("conn-%d", index)
			for round := range 5 {
				request := "GET /price?symbol=AAPL HTTP/1.1\r\nHost: x\r\n\r\n"
				want := `{"symbol":"AAPL","price":187.42}`
				if round == 2 {
					request = "GET /risk?seed=" + seed + " HTTP/1.1\r\nHost: x\r\n\r\n"
					want = `{"seed":"` + seed + `","risk_hash":"` + referenceRisk(seed) + `"}`
				}
				if _, err := conn.Write([]byte(request)); err != nil {
					errorsCh <- err
					return
				}
				response, err := http.ReadResponse(reader, nil)
				if err != nil {
					errorsCh <- err
					return
				}
				body, err := io.ReadAll(response.Body)
				response.Body.Close()
				if err != nil {
					errorsCh <- err
					return
				}
				if response.StatusCode != http.StatusOK || string(body) != want {
					errorsCh <- fmt.Errorf("connection %d round %d: %d %q", index, round, response.StatusCode, body)
					return
				}
			}
			errorsCh <- nil
		}()
	}
	for range connections {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
}
