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

// The raw server's wire output is validated with http.ReadResponse — the
// stdlib parser is the independent reference for framing correctness.

func startRawServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go rawServe(ln)
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

func dialRaw(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c, bufio.NewReader(c)
}

func readResp(t *testing.T, br *bufio.Reader) (int, string, *http.Response) {
	t.Helper()
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("stdlib could not parse raw response: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("body read: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode, string(body), resp
}

func TestRawHealthKeepAlive(t *testing.T) {
	addr := startRawServer(t)
	c, br := dialRaw(t, addr)
	for i := 0; i < 3; i++ {
		if _, err := c.Write([]byte("GET /health HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
			t.Fatalf("request %d write: %v", i, err)
		}
		code, body, resp := readResp(t, br)
		if code != 200 || body != `{"status":"ok"}` {
			t.Fatalf("request %d: %d %q", i, code, body)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type %q", ct)
		}
	}
}

func TestRawPriceAndStats(t *testing.T) {
	addr := startRawServer(t)
	c, br := dialRaw(t, addr)
	c.Write([]byte("GET /price?symbol=AAPL HTTP/1.1\r\nHost: x\r\n\r\n"))
	code, body, _ := readResp(t, br)
	if code != 200 || !strings.Contains(body, `"symbol":"AAPL"`) {
		t.Fatalf("price: %d %q", code, body)
	}
	c.Write([]byte("GET /stats?symbol=NVDA HTTP/1.1\r\nHost: x\r\n\r\n"))
	code, body, _ = readResp(t, br)
	if code != 200 || !strings.Contains(body, `"stddev":`) {
		t.Fatalf("stats: %d %q", code, body)
	}
	// Unknown symbol and unknown path must both give the contract 404 shape.
	for _, target := range []string{"/price?symbol=NOPE", "/stats?symbol=", "/price", "/nope"} {
		fmt.Fprintf(c, "GET %s HTTP/1.1\r\nHost: x\r\n\r\n", target)
		code, body, _ = readResp(t, br)
		if code != 404 || body != `{"error":"unknown symbol"}` {
			t.Fatalf("%s: %d %q", target, code, body)
		}
	}
}

// TestRawSplitReads: a request head arriving in arbitrary chunks must parse
// identically (the buffer refill path).
func TestRawSplitReads(t *testing.T) {
	addr := startRawServer(t)
	c, br := dialRaw(t, addr)
	full := "GET /price?symbol=GOOG HTTP/1.1\r\nHost: x\r\nAccept: */*\r\n\r\n"
	for _, chunk := range []string{full[:3], full[3:20], full[20 : len(full)-2], full[len(full)-2:]} {
		if _, err := c.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	code, body, _ := readResp(t, br)
	if code != 200 || !strings.Contains(body, `"symbol":"GOOG"`) {
		t.Fatalf("split read: %d %q", code, body)
	}
}

// TestRawPipelining: multiple requests in one segment answered in order.
func TestRawPipelining(t *testing.T) {
	addr := startRawServer(t)
	c, br := dialRaw(t, addr)
	c.Write([]byte("GET /price?symbol=MSFT HTTP/1.1\r\nHost: x\r\n\r\n" +
		"GET /health HTTP/1.1\r\nHost: x\r\n\r\n" +
		"GET /price?symbol=TSLA HTTP/1.1\r\nHost: x\r\n\r\n"))
	code, body, _ := readResp(t, br)
	if code != 200 || !strings.Contains(body, `"MSFT"`) {
		t.Fatalf("pipelined 1: %d %q", code, body)
	}
	code, body, _ = readResp(t, br)
	if code != 200 || body != `{"status":"ok"}` {
		t.Fatalf("pipelined 2: %d %q", code, body)
	}
	code, body, _ = readResp(t, br)
	if code != 200 || !strings.Contains(body, `"TSLA"`) {
		t.Fatalf("pipelined 3: %d %q", code, body)
	}
}

// TestRawOversizedHeadCloses: a head that cannot fit the 4KB buffer closes
// the connection without any response bytes.
func TestRawOversizedHeadCloses(t *testing.T) {
	addr := startRawServer(t)
	c, br := dialRaw(t, addr)
	c.Write([]byte("GET /health HTTP/1.1\r\nX-Pad: " + strings.Repeat("a", 5000) + "\r\n\r\n"))
	// Unread bytes at close may surface as RST rather than clean EOF — the
	// requirement is only: no response bytes, connection dead.
	if b, err := br.ReadByte(); err == nil {
		t.Fatalf("want closed connection, got response byte %q", b)
	}
}

// TestRawEncodedQuery: percent-encoded symbols route through symbolParam's
// full-parse fallback exactly like net/http.
func TestRawEncodedQuery(t *testing.T) {
	addr := startRawServer(t)
	c, br := dialRaw(t, addr)
	c.Write([]byte("GET /price?symbol=%41APL HTTP/1.1\r\nHost: x\r\n\r\n"))
	code, body, _ := readResp(t, br)
	if code != 200 || !strings.Contains(body, `"symbol":"AAPL"`) {
		t.Fatalf("encoded query: %d %q", code, body)
	}
}

// TestRawPostPrice: POST body framed by Content-Length, arriving split from
// the head, updates the price served by subsequent GETs.
func TestRawPostPrice(t *testing.T) {
	addr := startRawServer(t)
	c, br := dialRaw(t, addr)
	reqBody := `{"symbol":"RAWT","price":42.5}`
	head := fmt.Sprintf("POST /price HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n", len(reqBody))
	c.Write([]byte(head))
	time.Sleep(10 * time.Millisecond)
	c.Write([]byte(reqBody[:10]))
	time.Sleep(10 * time.Millisecond)
	c.Write([]byte(reqBody[10:]))
	code, body, _ := readResp(t, br)
	if code != 200 || body != `{"symbol":"RAWT","price":42.5}` {
		t.Fatalf("post: %d %q", code, body)
	}
	c.Write([]byte("GET /price?symbol=RAWT HTTP/1.1\r\nHost: x\r\n\r\n"))
	code, body, _ = readResp(t, br)
	if code != 200 || body != `{"symbol":"RAWT","price":42.5}` {
		t.Fatalf("get-after-post: %d %q", code, body)
	}
	// Malformed body → 400 with the handler's shape (accounting unchanged).
	bad := `{"symbol":"","price":1}`
	fmt.Fprintf(c, "POST /price HTTP/1.1\r\nHost: x\r\nContent-Length: %d\r\n\r\n%s", len(bad), bad)
	code, _, _ = readResp(t, br)
	if code != 400 {
		t.Fatalf("bad post: %d", code)
	}
}

// TestRawConnectionClose: Connection: close and HTTP/1.0 both end the
// connection after one response.
func TestRawConnectionClose(t *testing.T) {
	addr := startRawServer(t)
	for _, req := range []string{
		"GET /health HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n",
		"GET /health HTTP/1.0\r\n\r\n",
	} {
		c, br := dialRaw(t, addr)
		c.Write([]byte(req))
		code, body, _ := readResp(t, br)
		if code != 200 || body != `{"status":"ok"}` {
			t.Fatalf("%q: %d %q", req, code, body)
		}
		if _, err := br.ReadByte(); err != io.EOF {
			t.Fatalf("%q: connection stayed open (err=%v)", req, err)
		}
	}
}

// TestRawRejects: chunked bodies and malformed request lines close silently.
func TestRawRejects(t *testing.T) {
	addr := startRawServer(t)
	for _, req := range []string{
		"POST /price HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\n\r\n",
		"GARBAGE\r\n\r\n",
		"GET /health HTTP/2.7\r\n\r\n",
	} {
		c, br := dialRaw(t, addr)
		c.Write([]byte(req))
		if b, err := br.ReadByte(); err != io.EOF {
			t.Fatalf("%q: want silent close, got byte %q err %v", req, b, err)
		}
	}
}

// TestRawRiskEndToEnd: a /risk request through the raw server must park,
// be served by a worker, and answer with the exact chain digest. A one-shot
// manual worker stands in for the boot-time pool (which tests do not start).
func TestRawRiskEndToEnd(t *testing.T) {
	addr := startRawServer(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if ws := riskTakeWork(1); len(ws) == 1 {
				ws[0].result <- riskChain(ws[0].seed)
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	c, br := dialRaw(t, addr)
	c.Write([]byte("GET /risk?seed=rawtest HTTP/1.1\r\nHost: x\r\n\r\n"))
	code, body, _ := readResp(t, br)
	<-done
	want := fmt.Sprintf(`{"seed":"rawtest","risk_hash":"%s"}`, riskChain("rawtest"))
	if code != 200 || body != want {
		t.Fatalf("risk: %d %q want %q", code, body, want)
	}
}
