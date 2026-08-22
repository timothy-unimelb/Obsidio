//go:build amd64 && !purego

package main

import "os"

// useSHANIPair selects the two-lane SHA-NI kernel when the processor reports
// SHA, SSSE3, and SSE4.1 support. RISK_SHANI=0 forces the portable path.
var useSHANIPair = cpuidSHA() && os.Getenv("RISK_SHANI") != "0"

//go:noescape
func sum256x2(inA, inB *[64]byte, outA, outB *[32]byte)

//go:noescape
func cpuidSHA() bool
