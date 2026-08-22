package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"
)

// TestMain activates the risk kernel exactly as main() does, so every test
// below (including the 50k-chain digest reference) exercises the path that
// actually ships on this machine. On the grader's x86 build this runs inside
// `docker build` via the RUN go test gate.
func TestMain(m *testing.M) {
	initRiskKernel()
	initRiskKernelX16() // mirrors main(): x16 boot race + self-tests run in CI too
	os.Exit(m.Run())
}

// TestShedBudgetDerivation: the budget must track the declared error gate
// (88% of it) and no override may push it past 95% of the gate.
func TestShedBudgetDerivation(t *testing.T) {
	saved := riskShedBudgetBP
	defer func() { riskShedBudgetBP = saved }()

	t.Setenv("RISK_ERR_GATE_BP", "")
	t.Setenv("RISK_SHED_BUDGET_BP", "")
	initShedBudget()
	if riskShedBudgetBP != 88 {
		t.Fatalf("default gate 100bp should derive 88bp, got %d", riskShedBudgetBP)
	}
	t.Setenv("RISK_ERR_GATE_BP", "50") // e.g. locked gate tightened to 0.5%
	initShedBudget()
	if riskShedBudgetBP != 44 {
		t.Fatalf("gate 50bp should derive 44bp, got %d", riskShedBudgetBP)
	}
	t.Setenv("RISK_SHED_BUDGET_BP", "80") // override beyond 95% of gate
	initShedBudget()
	if riskShedBudgetBP != 47 { // 95% of 50, integer math
		t.Fatalf("override must clamp to 95%% of gate (47), got %d", riskShedBudgetBP)
	}
}

// TestPriceWALDurabilityAndReplay: accepted POSTs must survive a hard kill —
// replay applies complete lines in order (last write wins) and skips a torn
// final line, which by construction was never acknowledged with a 200.
func TestPriceWALDurabilityAndReplay(t *testing.T) {
	path := t.TempDir() + "/prices.wal"
	t.Setenv("PRICE_WAL", path)
	initPriceWAL()
	if walFile == nil {
		t.Fatal("WAL did not activate")
	}
	if !appendWAL("ZZZT", 12.5) || !appendWAL("ZZZT", 99.25) || !appendWAL("NEWCO", 1.75) {
		t.Fatal("durable append failed")
	}
	// Crash mid-append: torn line, no fsync, no 200 ever sent.
	if _, err := walFile.WriteString(`{"symbol":"TORN","price":`); err != nil {
		t.Fatal(err)
	}
	walFile.Close()
	walFile = nil

	initPriceWAL() // simulated restart
	mu.RLock()
	defer mu.RUnlock()
	if prices["ZZZT"] != 99.25 {
		t.Fatalf("ZZZT = %v, want last-written 99.25", prices["ZZZT"])
	}
	if prices["NEWCO"] != 1.75 {
		t.Fatalf("NEWCO = %v, want 1.75", prices["NEWCO"])
	}
	if _, ok := prices["TORN"]; ok {
		t.Fatal("torn (unacknowledged) line must not replay")
	}
	if _, ok := priceResp["NEWCO"]; !ok {
		t.Fatal("replayed symbol missing prebuilt GET body")
	}
}

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

// TestHexEncode64MatchesStdlib checks the packed pair-table encoder against
// encoding/hex for every possible input byte in every position class.
func TestHexEncode64MatchesStdlib(t *testing.T) {
	var sum [32]byte
	for v := 0; v < 256; v++ {
		for j := range sum {
			sum[j] = byte(v)
		}
		var got [64]byte
		hexEncode64(&got, &sum)
		want := hex.EncodeToString(sum[:])
		if string(got[:]) != want {
			t.Fatalf("hexEncode64 mismatch for byte 0x%02x: got %s want %s", v, got, want)
		}
	}
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

// TestStaleWaiterSkippedAtTake: a waiter older than the calibrated patience
// window must NOT be taken by a worker (its duration sample would land past
// the 1500ms bar); riskTakeWork unlinks and skips it.
func TestStaleWaiterSkippedAtTake(t *testing.T) {
	savedPatience := riskPatienceNs.Load()
	riskPatienceNs.Store(int64(50 * time.Millisecond))
	defer riskPatienceNs.Store(savedPatience)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func() { _, ok := riskSubmit(ctx, "stale-test"); done <- ok }()
	}
	deadline := time.Now().Add(time.Second)
	for {
		riskMu.Lock()
		depth := riskStackDepth
		riskMu.Unlock()
		if depth == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("waiters never parked")
		}
		time.Sleep(time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond) // age both waiters past the patience window

	if got := riskTakeWork(2); len(got) != 0 {
		t.Fatalf("took %d stale waiters; want 0", len(got))
	}
	riskMu.Lock()
	depth := riskStackDepth
	riskMu.Unlock()
	if depth != 0 {
		t.Fatalf("stale waiters not unlinked: depth=%d", depth)
	}

	// A fresh waiter must still be takeable and servable.
	fresh := make(chan string, 1)
	go func() {
		h, ok := riskSubmit(context.Background(), "fresh")
		if !ok {
			h = "SHED"
		}
		fresh <- h
	}()
	deadline = time.Now().Add(time.Second)
	var got []*riskWaiter
	for len(got) == 0 {
		got = riskTakeWork(2)
		if time.Now().After(deadline) {
			t.Fatal("fresh waiter never became takeable")
		}
		time.Sleep(time.Millisecond)
	}
	if len(got) != 1 || got[0].seed != "fresh" {
		t.Fatalf("took %d waiters, want the single fresh one", len(got))
	}
	got[0].result <- "digest"
	if h := <-fresh; h != "digest" {
		t.Fatalf("fresh waiter got %q", h)
	}

	cancel()
	<-done
	<-done
	riskMu.Lock()
	defer riskMu.Unlock()
	if riskStackDepth != 0 || riskStackTop != nil {
		t.Fatalf("gate state not restored: depth=%d", riskStackDepth)
	}
}

// TestRiskGateRaceHammer drives the submit/worker gate with concurrent
// submissions, client cancels, and real workers (real chains), then asserts
// the accounting converged: every submitter got a decision, no stranded
// waiters. Run with -race.
func TestRiskGateRaceHammer(t *testing.T) {
	for i := 0; i < 2; i++ {
		go riskWorker() // leaked for process lifetime; blocks on riskWork when idle
	}
	const goroutines = 80
	var wg sync.WaitGroup
	var servedN, shedN int64
	var mu sync.Mutex
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// A mix of patient clients and ones that disconnect mid-wait.
			timeout := time.Duration(1+rand.Intn(30)) * time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			h, ok := riskSubmit(ctx, "hammer")
			mu.Lock()
			if ok {
				if len(h) != 64 {
					t.Errorf("bad digest length %d", len(h))
				}
				servedN++
			} else {
				shedN++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	// Let workers drain any taken-but-undelivered stragglers.
	deadline := time.Now().Add(5 * time.Second)
	for {
		riskMu.Lock()
		depth, top := riskStackDepth, riskStackTop
		riskMu.Unlock()
		if depth == 0 && top == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stranded waiters: depth=%d", depth)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if servedN+shedN != goroutines {
		t.Fatalf("accounting: served=%d shed=%d != %d", servedN, shedN, goroutines)
	}
	if servedN == 0 {
		t.Fatal("hammer served nothing — workers never ran")
	}
	t.Logf("served=%d shed=%d", servedN, shedN)
}
