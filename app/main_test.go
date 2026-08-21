package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
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
