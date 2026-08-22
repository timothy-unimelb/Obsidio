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
