//go:build !amd64 || purego

package main

const riskX16Enabled = false

var riskX16MinBatch = x16Lanes

func initRiskX16() {}

func runRiskBatchX16(batch []*riskJob) { panic("kernel unavailable") }
