//go:build !amd64 || purego

package main

const useSHANIPair = false

func sum256x2(inA, inB *[64]byte, outA, outB *[32]byte) {
	*outA = sum256Portable(inA)
	*outB = sum256Portable(inB)
}
