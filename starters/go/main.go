// Obsidio: Go + net/http backend, tuned for resilience under the Obsidio load
// test. Implements the endpoint contract exactly (see OBSIDIO-DETAIL-PAGE.md).
//
// Resilience measures on top of the naive starter:
//   - GOMAXPROCS pinned to 2 to match the container's real CPU cap, instead
//     of the host's visible core count.
//   - /risk (the CPU-heavy endpoint) is gated by a bounded semaphore
//     (riskMaxConcurrent) plus a bounded wait queue (riskMaxQueued), so it
//     cannot starve /price and /stats of CPU time. Requests beyond both
//     bounds are rejected immediately (503) rather than queued past their
//     latency budget.
//   - /price and /stats are left ungated -- they are cheap and should never
//     be blocked by the risk gate.
//   - The HTTP server has explicit read/write/idle timeouts and shuts down
//     gracefully on SIGTERM.
//
// No caching of /stats: the spec requires it be computed fresh per request.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
)

var prices = map[string]float64{
	"AAPL": 187.42, "GOOG": 141.80, "MSFT": 412.30, "AMZN": 178.10,
	"NVDA": 120.15, "META": 502.60, "TSLA": 244.70, "JPM": 198.35,
}
var series = map[string][]float64{}

// /risk concurrency gate: bounds how many risk computations can run at once
// (riskMaxConcurrent) and how many more can wait for a slot (riskMaxQueued)
// before we fail fast instead of letting the queue grow past the latency
// budget. Sized to leave the other CPU core free for /price and /stats.
//
// riskMaxQueued and riskWaitTimeout are deliberately generous, not stingy:
// a rejected request costs an error (counts against the global <1% error
// ceiling, a hard qualifying gate), while a queued-then-completed request
// only costs latency, and /risk has a 1500ms budget against a ~10-15ms
// actual compute cost -- about 100x headroom. It is far cheaper to make a
// request wait than to reject it, right up until the wait itself would
// blow the latency budget. riskWaitTimeout leaves that margin.
const (
	riskMaxConcurrent = 2
	riskMaxQueued     = 300
	riskWaitTimeout   = 1200 * time.Millisecond
)

var (
	riskSem     = make(chan struct{}, riskMaxConcurrent)
	riskWaiting int32
)

func init() {
	for s, base := range prices {
		arr := make([]float64, 500)
		for i := 0; i < 500; i++ {
			arr[i] = base * (1 + math.Sin(float64(i))/50)
		}
		series[s] = arr
	}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func main() {
	// The container is capped at 2 CPUs, but Go reads the HOST's core count
	// for GOMAXPROCS by default. Pin it explicitly so the scheduler sizes
	// itself to the real budget instead of a number it can see but not use.
	runtime.GOMAXPROCS(2)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	// CHEAP (weight 1)
	http.HandleFunc("/price", func(w http.ResponseWriter, r *http.Request) {
		s := r.URL.Query().Get("symbol")
		p, ok := prices[s]
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "unknown symbol"})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"symbol": s, "price": p})
	})

	// MEDIUM (weight 3)
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		s := r.URL.Query().Get("symbol")
		arr, ok := series[s]
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "unknown symbol"})
			return
		}
		n := float64(len(arr))
		sum, mn, mx := 0.0, arr[0], arr[0]
		for _, v := range arr {
			sum += v
			if v < mn {
				mn = v
			}
			if v > mx {
				mx = v
			}
		}
		mean := sum / n
		varr := 0.0
		for _, v := range arr {
			varr += (v - mean) * (v - mean)
		}
		writeJSON(w, 200, map[string]interface{}{
			"symbol": s, "mean": mean, "min": mn, "max": mx,
			"stddev": math.Sqrt(varr / n),
		})
	})

	// HEAVY (weight 10): 50000 iterations of SHA-256 over the seed. Uncacheable.
	//
	// Gated by riskSem so at most riskMaxConcurrent of these run at once, and
	// by riskWaiting so at most riskMaxQueued more wait for a slot. Anything
	// past that is rejected immediately (503) instead of queueing past the
	// latency budget -- a request that times out scores zero either way, so
	// failing fast costs nothing and protects /price and /stats from starving.
	http.HandleFunc("/risk", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&riskWaiting, 1) > riskMaxQueued {
			atomic.AddInt32(&riskWaiting, -1)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "overloaded, try again"})
			return
		}

		// Wait for a slot, but give up -- instead of blocking forever -- if
		// riskWaitTimeout passes or the client disconnects/the connection
		// times out (r.Context() is cancelled by net/http in that case).
		// Without this, a goroutine that loses its client while still
		// waiting for a slot never notices and leaks indefinitely, which
		// under sustained load compounds into exactly the kind of pileup
		// this gate exists to prevent.
		waitCtx, cancel := context.WithTimeout(r.Context(), riskWaitTimeout)
		defer cancel()
		select {
		case riskSem <- struct{}{}:
			atomic.AddInt32(&riskWaiting, -1)
		case <-waitCtx.Done():
			atomic.AddInt32(&riskWaiting, -1)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "overloaded, try again"})
			return
		}
		defer func() { <-riskSem }()

		seed := r.URL.Query().Get("seed")
		if seed == "" {
			seed = "none"
		}
		h := seed
		for i := 0; i < 50000; i++ {
			sum := sha256.Sum256([]byte(h))
			h = hex.EncodeToString(sum[:])
		}
		writeJSON(w, 200, map[string]interface{}{"seed": seed, "risk_hash": h})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Explicit timeouts instead of the zero-value (no timeout) defaults that
	// http.ListenAndServe uses -- a slow or stalled client shouldn't be able
	// to pin a connection open indefinitely.
	srv := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown: stop accepting new connections and let in-flight
	// requests finish (up to 5s) on SIGTERM, so a container restart doesn't
	// cut requests off mid-flight.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("listening on :%s (GOMAXPROCS=%d)", port, runtime.GOMAXPROCS(0))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
