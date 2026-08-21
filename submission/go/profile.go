//go:build profile

package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
)

// Profiling is compiled out of normal submission builds. Diagnostic images use
// --build-arg GO_BUILD_TAGS=profile and publish this listener separately from
// the scored API port.
func init() {
	runtime.SetBlockProfileRate(1)
	runtime.SetMutexProfileFraction(1)
	go func() {
		log.Print("diagnostic pprof listener on :6060")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Printf("diagnostic pprof listener stopped: %v", err)
		}
	}()
}
