//go:build amd64 && !purego

package main

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

// fill normalizes arbitrary fuzz bytes into a fixed 64-byte kernel input: the
// kernel is specialized to a single 64-byte block, which is all the feedback
// chain ever feeds it. Short inputs are zero-padded, long inputs truncated.
func fill(dst *[sha256.Size * 2]byte, src []byte) {
	*dst = [sha256.Size * 2]byte{}
	copy(dst[:], src)
}

// FuzzSum256x2 is the highest-leverage automated catch for an input-dependent
// kernel bug — a wrong shuffle mask or schedule constant that only shows for
// certain byte patterns. It differentially compares both lanes against the
// standard library for every input the fuzzer explores. The seed corpus covers
// boundary byte patterns and the restricted ASCII hex-digit distribution the
// real chain actually produces, which uniform random inputs under-sample.
func FuzzSum256x2(f *testing.F) {
	if !cpuidSHA() {
		f.Skip("processor lacks SHA-NI; kernel not exercised here")
	}
	prev := useSHANIPair
	useSHANIPair = true
	f.Cleanup(func() { useSHANIPair = prev })

	zeros := bytes.Repeat([]byte{0x00}, 64)
	ones := bytes.Repeat([]byte{0xff}, 64)
	alt := bytes.Repeat([]byte{0xaa, 0x55}, 32)
	hexLo := bytes.Repeat([]byte("0123456789abcdef"), 4) // the chain's real alphabet
	hexHi := bytes.Repeat([]byte("fedcba9876543210"), 4)
	f.Add(zeros, ones)
	f.Add(ones, zeros)
	f.Add(alt, hexLo)
	f.Add(hexLo, hexHi)
	f.Add([]byte("abc"), []byte(""))

	f.Fuzz(func(t *testing.T, a, b []byte) {
		var inA, inB [sha256.Size * 2]byte
		fill(&inA, a)
		fill(&inB, b)

		var outA, outB [sha256.Size]byte
		sum256x2(&inA, &inB, &outA, &outB)

		if outA != sha256.Sum256(inA[:]) {
			t.Fatalf("lane A mismatch for input %x", inA)
		}
		if outB != sha256.Sum256(inB[:]) {
			t.Fatalf("lane B mismatch for input %x", inB)
		}
	})
}
