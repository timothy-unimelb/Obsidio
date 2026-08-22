package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// naiveRiskChain is the straightforward starter algorithm, kept as an
// independent reference so kernel optimisations can't silently change the
// digest (a wrong digest scores zero).
func naiveRiskChain(seed string) string {
	h := seed
	for i := 0; i < 50000; i++ {
		sum := sha256.Sum256([]byte(h))
		h = hex.EncodeToString(sum[:])
	}
	return h
}

func TestRiskChainMatchesReference(t *testing.T) {
	for _, seed := range []string{"none", "abc", "0.12345", ""} {
		if got, want := riskChain(seed), naiveRiskChain(seed); got != want {
			t.Fatalf("riskChain(%q) = %s, want %s", seed, got, want)
		}
	}
}

// Tier-0 iteration loop for kernel changes: seconds, ~1-2% noise.
//
//	cd app && go test -bench=RiskChain -benchtime=10x -count=3
func BenchmarkRiskChain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		riskChain("bench-seed")
	}
}

// TestStaleWaiterSkippedAtGrant: a waiter older than the calibrated patience
// window must NOT be served (its duration sample would land past the 1500ms
// bar); the granter unlinks it and retires the slot instead.
func TestStaleWaiterSkippedAtGrant(t *testing.T) {
	savedTimeout := riskWaitTimeout
	riskWaitTimeout = 50 * time.Millisecond
	defer func() { riskWaitTimeout = savedTimeout }()

	if !riskAcquire(context.Background()) || !riskAcquire(context.Background()) {
		t.Fatal("could not fill the execution slots")
	}

	ctx, cancel := context.WithCancel(context.Background())
	granted := make(chan bool, 1)
	go func() { granted <- riskAcquire(ctx) }()

	deadline := time.Now().Add(time.Second)
	for {
		riskMu.Lock()
		depth := riskStackDepth
		riskMu.Unlock()
		if depth == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("waiter never parked")
		}
		time.Sleep(time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond) // age the waiter past riskWaitTimeout

	releaseRiskSlot()
	select {
	case g := <-granted:
		t.Fatalf("stale waiter got a grant-time decision: granted=%v", g)
	case <-time.After(50 * time.Millisecond):
		// still parked: correct — shed budget is closed (tiny denominator)
	}
	riskMu.Lock()
	running := riskRunning
	riskMu.Unlock()
	if running != 1 {
		t.Fatalf("slot not retired past the stale waiter: riskRunning=%d", running)
	}

	cancel()
	if g := <-granted; g {
		t.Fatal("stale waiter reported granted after client disconnect")
	}
	releaseRiskSlot()
	riskMu.Lock()
	defer riskMu.Unlock()
	if riskRunning != 0 || riskStackDepth != 0 || riskStackTop != nil {
		t.Fatalf("gate state not restored: running=%d depth=%d", riskRunning, riskStackDepth)
	}
}

// TestRiskGateRaceHammer drives the LIFO admission gate with concurrent
// acquire / client-cancel / release traffic and then asserts the accounting
// converged: no leaked slots, no stranded waiters. Run with -race.
func TestRiskGateRaceHammer(t *testing.T) {
	const goroutines = 400
	var wg sync.WaitGroup
	var servedN, shedN int64
	var mu sync.Mutex
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// A mix of patient clients and ones that disconnect mid-wait.
			timeout := time.Duration(rand.Intn(3000)) * time.Microsecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if riskAcquire(ctx) {
				// simulate a sliver of chain work while holding the slot
				time.Sleep(time.Duration(rand.Intn(200)) * time.Microsecond)
				releaseRiskSlot()
				mu.Lock()
				servedN++
				mu.Unlock()
			} else {
				mu.Lock()
				shedN++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	riskMu.Lock()
	running, depth, top := riskRunning, riskStackDepth, riskStackTop
	riskMu.Unlock()
	if running != 0 {
		t.Fatalf("leaked slots: riskRunning=%d after all goroutines finished", running)
	}
	if depth != 0 || top != nil {
		t.Fatalf("stranded waiters: depth=%d top=%v", depth, top)
	}
	if servedN+shedN != goroutines {
		t.Fatalf("accounting: served=%d shed=%d != %d", servedN, shedN, goroutines)
	}
	t.Logf("served=%d shed=%d", servedN, shedN)
}
