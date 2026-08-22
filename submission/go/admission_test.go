package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestGate(parkMax int, patience time.Duration, budget float64) *riskGate {
	return &riskGate{wake: make(chan struct{}, 64), shed: true, parkMax: parkMax, patience: patience, errorBudget: budget}
}

func testJob(seed string) (riskJob, chan riskResult) {
	result := make(chan riskResult, 1)
	return riskJob{seed: seed, queuedAt: time.Now(), ctx: context.Background(), result: result}, result
}

func TestGateServesFIFOWhileWaitsAreShort(t *testing.T) {
	g := newTestGate(16, time.Second, 0)
	for _, seed := range []string{"first", "second", "third"} {
		job, _ := testJob(seed)
		if !g.admit(job) {
			t.Fatalf("job %q rejected with an empty stack", seed)
		}
	}
	var batch [maxRiskLanes]riskJob
	if count := g.take(2, batch[:]); count != 2 || batch[0].seed != "first" || batch[1].seed != "second" {
		t.Fatalf("expected arrival order, got %d jobs: %q %q", count, batch[0].seed, batch[1].seed)
	}
	if count := g.take(4, batch[:]); count != 1 || batch[0].seed != "third" {
		t.Fatalf("expected the remaining job, got %d: %q", count, batch[0].seed)
	}
}

func TestGateSwitchesToNewestFirstUnderOverload(t *testing.T) {
	g := newTestGate(16, time.Second, 0)
	old, _ := testJob("old")
	old.queuedAt = time.Now().Add(-900 * time.Millisecond) // past 80% of patience
	g.admit(old)
	for _, seed := range []string{"second", "third"} {
		job, _ := testJob(seed)
		g.admit(job)
	}
	var batch [maxRiskLanes]riskJob
	if count := g.take(2, batch[:]); count != 2 || batch[0].seed != "third" || batch[1].seed != "second" {
		t.Fatalf("expected newest-first under overload, got %d jobs: %q %q", count, batch[0].seed, batch[1].seed)
	}
}

func TestGateDiscardsJobsPastPatience(t *testing.T) {
	g := newTestGate(16, 20*time.Millisecond, 0)
	stale, staleResult := testJob("stale")
	stale.queuedAt = time.Now().Add(-time.Second)
	fresh, _ := testJob("fresh")
	g.admit(stale)
	g.admit(fresh)
	var batch [maxRiskLanes]riskJob
	if count := g.take(4, batch[:]); count != 1 || batch[0].seed != "fresh" {
		t.Fatalf("expected only the fresh job, got %d: %q", count, batch[0].seed)
	}
	select {
	case result := <-staleResult:
		if !result.rejected {
			t.Fatal("stale job should have been rejected")
		}
	default:
		t.Fatal("stale job received no rejection")
	}
	if g.discarded.Load() != 1 {
		t.Fatalf("discarded counter = %d", g.discarded.Load())
	}
}

func TestGateSkipsAbandonedJobs(t *testing.T) {
	g := newTestGate(16, time.Second, 0)
	ctx, cancel := context.WithCancel(context.Background())
	gone, _ := testJob("gone")
	gone.ctx = ctx
	live, _ := testJob("live")
	g.admit(live)
	g.admit(gone)
	cancel()
	var batch [maxRiskLanes]riskJob
	if count := g.take(4, batch[:]); count != 1 || batch[0].seed != "live" {
		t.Fatalf("expected the live job only, got %d: %q", count, batch[0].seed)
	}
}

func TestGateFrontDoorRejectRespectsBudget(t *testing.T) {
	g := newTestGate(1, time.Second, 0.01)
	first, _ := testJob("a")
	second, _ := testJob("b")
	third, _ := testJob("c")
	if !g.admit(first) {
		t.Fatal("first job should park")
	}
	// Stack full but no requests counted yet: the budget allows no errors, so
	// the job must park rather than be rejected.
	if !g.admit(second) {
		t.Fatal("budget exhausted: job should park, not be rejected")
	}
	for range 1000 {
		g.countRequest()
	}
	if g.admit(third) {
		t.Fatal("stack full with budget room: job should be rejected at the front door")
	}
	if g.rejected.Load() != 1 || g.parkedCount() != 2 {
		t.Fatalf("rejected=%d parked=%d", g.rejected.Load(), g.parkedCount())
	}
}

func TestRiskRejectionIs503AndPriceUnaffected(t *testing.T) {
	previous := gate
	gate = newTestGate(1, time.Second, 0.01)
	defer func() { gate = previous }()
	for range 1000 {
		gate.countRequest()
	}
	// Fill the park slot directly so no worker consumes it.
	blocker, _ := testJob("blocker")
	gate.admit(blocker)

	response := request(t, "/risk?seed=shed")
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "overloaded") {
		t.Fatalf("expected 503 overloaded, got %d %s", response.Code, response.Body.String())
	}
	price := request(t, "/price?symbol=AAPL")
	if price.Code != http.StatusOK {
		t.Fatalf("price should be unaffected by risk overload, got %d", price.Code)
	}
}

func TestGateDefaultNeverSheds(t *testing.T) {
	g := &riskGate{wake: make(chan struct{}, 64), parkMax: 1, patience: time.Millisecond, errorBudget: 0.01}
	for range 1000 {
		g.countRequest()
	}
	stale, staleResult := testJob("stale")
	stale.queuedAt = time.Now().Add(-time.Second)
	extra, _ := testJob("extra")
	if !g.admit(stale) || !g.admit(extra) {
		t.Fatal("default gate must park every job")
	}
	var batch [maxRiskLanes]riskJob
	if count := g.take(4, batch[:]); count != 2 || batch[0].seed != "stale" {
		t.Fatalf("default gate must serve FIFO without discards, got %d: %q", count, batch[0].seed)
	}
	select {
	case <-staleResult:
		t.Fatal("default gate rejected a job")
	default:
	}
}

func TestGateInertUnderConcurrentLoad(t *testing.T) {
	// Under normal load nothing is rejected or discarded and every answer is
	// correct; this guards the gate's no-op behaviour for the published siege.
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
	if gate.rejected.Load() != 0 || gate.discarded.Load() != 0 {
		t.Fatalf("gate acted under moderate load: rejected=%d discarded=%d", gate.rejected.Load(), gate.discarded.Load())
	}
}
