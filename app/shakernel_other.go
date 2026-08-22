//go:build !amd64

package main

// Non-amd64 (e.g. the arm64 dev containers): riskSum64 keeps its stdlib
// default; arm64 crypto/sha256 already uses the ARMv8 SHA2 instructions.
func initRiskKernel() {}
