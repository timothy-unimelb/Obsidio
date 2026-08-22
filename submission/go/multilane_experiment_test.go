package main

import (
	"strconv"
	"testing"
)

func TestMultiLaneRiskMatchesReference(t *testing.T) {
	seeds := [maxRiskLanes]string{"0.4821", "none", "multi lane", "0.000001"}
	a, b := calculateRiskPair(seeds[0], seeds[1])
	if string(a[:]) != referenceRisk(seeds[0]) || string(b[:]) != referenceRisk(seeds[1]) {
		t.Fatal("pair lanes diverged from reference")
	}
	quad := calculateRiskQuad(seeds)
	for index, seed := range seeds {
		if string(quad[index][:]) != referenceRisk(seed) {
			t.Fatalf("quad lane %d diverged from reference for %q", index, seed)
		}
	}
}

func TestRiskBatchSizesMatchReference(t *testing.T) {
	for size := 1; size <= maxRiskLanes; size++ {
		batch := make([]*riskJob, size)
		channels := make([]chan riskResult, size)
		for index := range batch {
			channels[index] = make(chan riskResult, 1)
			batch[index] = &riskJob{seed: "batch-" + strconv.Itoa(size) + "-" + strconv.Itoa(index), result: channels[index]}
		}
		runRiskBatch(batch)
		for index := range batch {
			got := <-channels[index]
			if string(got.hash[:]) != referenceRisk(batch[index].seed) {
				t.Fatalf("batch size %d lane %d diverged from reference", size, index)
			}
		}
	}
}

// Per-request cost is what matters: divide the pair/quad time by the lane
// count when comparing with BenchmarkRisk.
func BenchmarkRiskPair(b *testing.B) {
	for index := 0; index < b.N; index++ {
		_, _ = calculateRiskPair(strconv.Itoa(index), strconv.Itoa(-index))
	}
}

func BenchmarkRiskQuad(b *testing.B) {
	for index := 0; index < b.N; index++ {
		base := index * 4
		_ = calculateRiskQuad([maxRiskLanes]string{
			strconv.Itoa(base), strconv.Itoa(base + 1), strconv.Itoa(base + 2), strconv.Itoa(base + 3),
		})
	}
}
