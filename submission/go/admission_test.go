package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestGate(shed bool, budgetBP int64, patience time.Duration) *riskGate {
	g := &riskGate{wake: make(chan struct{}, 16), shed: shed, budgetBP: budgetBP}
	g.patienceNs.Store(int64(patience))
	return g
}

func testJob(seed string) *riskJob {
	return &riskJob{seed: seed, queuedAt: time.Now(), ctx: context.Background(), result: make(chan riskResult, 1)}
}

func fillBudget(g *riskGate, responses int) {
	for range responses {
		g.countResponse()
	}
}

func TestGateServesNewestFirstWhenShedding(t *testing.T) {
	g := newTestGate(true, 88, time.Second)
	for _, seed := range []string{"first", "second", "third"} {
		if !g.admit(testJob(seed)) {
			t.Fatalf("job %q rejected while a worker could be idle", seed)
		}
	}
	var batch [maxRiskLanes]*riskJob
	if count := g.take(2, batch[:]); count != 2 || batch[0].seed != "third" || batch[1].seed != "second" {
		t.Fatalf("expected newest-first, got %d: %q %q", count, batch[0].seed, batch[1].seed)
	}
	if count := g.take(4, batch[:]); count != 1 || batch[0].seed != "first" {
		t.Fatalf("expected the remaining job, got %d: %q", count, batch[0].seed)
	}
}

func TestGateServesFIFOWhenSheddingDisabled(t *testing.T) {
	g := newTestGate(false, 88, time.Millisecond)
	fillBudget(g, 1000)
	stale := testJob("stale")
	stale.queuedAt = time.Now().Add(-time.Second)
	g.admit(stale)
	g.admit(testJob("second"))
	g.admit(testJob("third")) // never rejected: no idle-worker rule without shedding
	var batch [maxRiskLanes]*riskJob
	if count := g.take(2, batch[:]); count != 2 || batch[0].seed != "stale" || batch[1].seed != "second" {
		t.Fatalf("expected arrival order with no discards, got %d: %q %q", count, batch[0].seed, batch[1].seed)
	}
	if g.errors.Load() != 0 {
		t.Fatalf("shedding disabled must not charge errors, got %d", g.errors.Load())
	}
}

func TestGateSkipsStaleAndAbandonedJobs(t *testing.T) {
	g := newTestGate(true, 88, 20*time.Millisecond)
	stale := testJob("stale")
	stale.queuedAt = time.Now().Add(-time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	gone := testJob("gone")
	gone.ctx = ctx
	g.admit(testJob("live"))
	g.admit(stale)
	g.admit(gone)
	cancel()
	var batch [maxRiskLanes]*riskJob
	if count := g.take(4, batch[:]); count != 1 || batch[0].seed != "live" {
		t.Fatalf("expected only the live job, got %d: %q", count, batch[0].seed)
	}
	if g.errors.Load() != 1 {
		t.Fatalf("stale skip must be charged once, got %d", g.errors.Load())
	}
}

func TestGateFrontDoorShedsOnlyWhenBusyAndBudgeted(t *testing.T) {
	g := newTestGate(true, 88, time.Second)
	// Waiters parked but the budget denominator is tiny: park, do not shed.
	g.admit(testJob("a"))
	if !g.admit(testJob("b")) {
		t.Fatal("tiny denominator: job should park")
	}
	fillBudget(g, 10000)
	if g.admit(testJob("c")) {
		t.Fatal("no idle worker, waiters parked, budget room: job should be rejected")
	}
	// Spend the budget (88bp of 10,000 responses is 88 errors).
	for range 88 {
		g.countError()
	}
	if !g.admit(testJob("d")) {
		t.Fatal("budget exhausted: job should park rather than be rejected")
	}
	// With an idle worker, nothing is shed regardless of budget.
	g2 := newTestGate(true, 88, time.Second)
	fillBudget(g2, 10000)
	g2.mu.Lock()
	g2.idleWorkers = 1
	g2.mu.Unlock()
	g2.admit(testJob("x"))
	if !g2.admit(testJob("y")) {
		t.Fatal("an idle worker means no overload: job should park")
	}
}

func TestWaitShedsAfterPatienceWithinBudget(t *testing.T) {
	g := newTestGate(true, 88, 10*time.Millisecond)
	fillBudget(g, 10000)
	job := testJob("slow")
	g.admit(job)
	started := time.Now()
	if _, ok := g.wait(job); ok {
		t.Fatal("expected the waiter to shed itself after patience")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("patience shed took too long")
	}
	if !job.abandoned {
		t.Fatal("shed waiter must be marked abandoned for take-time skipping")
	}
	if g.errors.Load() != 1 {
		t.Fatalf("patience shed must reserve exactly one error, got %d", g.errors.Load())
	}
}

func TestReserveErrorIsAtomicUnderContention(t *testing.T) {
	g := newTestGate(true, 88, time.Second)
	fillBudget(g, 100000) // budget: 880 errors
	var granted atomic.Int64
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				if g.reserveError() {
					granted.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	if granted.Load() != 880 || g.errors.Load() != 880 {
		t.Fatalf("reserved %d errors, charged %d; budget is 880", granted.Load(), g.errors.Load())
	}
}

func TestRiskRejectionIs503AndPriceUnaffected(t *testing.T) {
	previous := gate
	gate = newTestGate(true, 88, time.Second)
	defer func() { gate = previous }()
	fillBudget(gate, 10000)
	gate.admit(testJob("blocker")) // parked, no worker consumes it, no idle worker

	response := request(t, "/risk?seed=shed")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "overloaded") {
		t.Fatalf("expected 503 overloaded, got %d %s", response.Code, response.Body.String())
	}
	if gate.errors.Load() != 1 {
		t.Fatalf("front-door rejection must be charged, got %d", gate.errors.Load())
	}
	price := request(t, "/price?symbol=AAPL")
	if price.Code != http.StatusOK {
		t.Fatalf("price should be unaffected by risk overload, got %d", price.Code)
	}
}

func TestGateInertUnderModerateLoad(t *testing.T) {
	// Early in a run the budget denominator is small, so nothing is shed and
	// every answer is correct.
	const requests = 24
	results := make(chan *httptest.ResponseRecorder, requests)
	for index := range requests {
		go func() {
			results <- request(t, "/risk?seed=load-"+strconv.Itoa(index))
		}()
	}
	for range requests {
		if response := <-results; response.Code != http.StatusOK {
			t.Fatalf("unexpected status %d under moderate load", response.Code)
		}
	}
}

func TestPatienceTracksChainCost(t *testing.T) {
	g := newTestGate(true, 88, time.Second)
	g.observeChainCost(12 * time.Millisecond)
	if got := g.patience(); got != riskLatencyBar-12*time.Millisecond-riskSafetyMargin {
		t.Fatalf("patience after first sample = %s", got)
	}
	g.observeChainCost(10 * time.Second) // clamped to 500 ms
	if got := g.patience(); got < minimumPatience || got > riskLatencyBar {
		t.Fatalf("patience out of range after a freak sample: %s", got)
	}
}
