//go:build !amd64 || purego

package main

const useSHANIPair = false

func riskChain2x(buf0, buf1 *[64]byte, rounds int) { panic("kernel unavailable") }

func riskChain4x(buf0, buf1, buf2, buf3 *[64]byte, rounds int) { panic("kernel unavailable") }
