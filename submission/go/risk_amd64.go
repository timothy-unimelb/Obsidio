//go:build amd64 && !purego

package main

import "os"

// useSHANIPair selects the SHA-NI chain kernels when the processor reports
// SHA, SSSE3, and SSE4.1 support. RISK_SHANI=0 forces the portable path.
var useSHANIPair = cpuidSHA() && os.Getenv("RISK_SHANI") != "0"

// riskChain2x and riskChain4x replace each 64-byte lowercase-hex buffer with
// the lowercase-hex SHA-256 of its contents, `rounds` times, for two or four
// independent chains interleaved on one core.
//
//go:noescape
func riskChain2x(buf0, buf1 *[64]byte, rounds int)

//go:noescape
func riskChain4x(buf0, buf1, buf2, buf3 *[64]byte, rounds int)

//go:noescape
func cpuidSHA() bool
