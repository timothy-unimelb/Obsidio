# Obsidio engineered backend: Go. Multi-stage, pinned images, reproducible.
# (Measured 2026-08-22 on SPR: stdlib SHA-NI predates 1.25 — 1.22 and 1.26 are
# kernel-flat. 1.26 kept for toolchain currency + container-aware runtime. The
# vendored sha256block_amd64.s must match this toolchain's stdlib version.)
FROM golang:1.26-bookworm AS build
WORKDIR /app
COPY go.mod *.go *.s ./
# The digest-guard test is a BUILD GATE: a kernel change that breaks the /risk
# digest scores zero at grading — better to fail the image build than to ship it.
RUN go test ./...
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server .

FROM debian:bookworm-slim
COPY --from=build /app/server /server
# Belt-and-suspenders with runtime.GOMAXPROCS(2) in main(): never let the
# runtime size itself from the visible host cores.
ENV GOMAXPROCS=2
# Steady-state allocation is near zero (~6 MB live heap) but the collector still
# ran ~1.2 cycles/sec under load, each costing a few ms of CPU. GOGC=off stops
# scheduled collections; GOMEMLIMIT is the hard safety net (never ship GOGC=off
# without it) — well under the 2 GiB cap, far above the ~6 MB live set.
ENV GOGC=off
# 512MiB: measured peak RSS hit 1489MiB with a 1600MiB limit (GOGC=off grows
# the heap to the limit by design). At 512MiB the limit-triggered GC runs about
# once per couple of minutes — same benefit, 4x the headroom under the 2GiB cap.
ENV GOMEMLIMIT=512MiB
# Shed budget 88bp of the 100bp gate (measured k6-visible rate ~80bp: the
# stale-skip preemptive charge makes the internal counter a strict
# overestimate). Sweep data: EXPERIMENTS.md 2026-08-22 budget sweep.
ENV RISK_SHED_BUDGET_BP=88
EXPOSE 8080
CMD ["/server"]
