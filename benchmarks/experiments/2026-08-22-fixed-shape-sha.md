# Fixed-shape SHA-256 experiment

## Hypothesis

After the first `/risk` iteration, every SHA-256 input is exactly 64 lowercase
hexadecimal bytes. A specialized implementation could avoid the standard
library's general length handling and reuse the constant padding-block message
schedule.

## Prototype

The isolated test-only prototype implemented FIPS 180-4 SHA-256 compression in
portable Go. It:

- parsed the first variable 64-byte block directly into the message schedule;
- precomputed all 64 schedule words for the constant second padding block;
- initialized the standard SHA-256 state directly;
- compressed exactly those two blocks; and
- serialized the resulting eight state words without allocation.

The production server and Dockerfile were not changed.

## Correctness

The prototype passed:

- 10,000 deterministic randomized 64-byte vectors against
  `crypto/sha256.Sum256`;
- complete 50,000-round risk vectors against the independent reference
  implementation; and
- the same test set with `GODEBUG=cpu.all=off`.

## Level 0 performance

Both implementations used the pinned Go 1.26.6 Linux arm64 builder. The
isolated SHA benchmark used eight one-second samples; the complete risk
benchmark used five.

| Measurement | Standard library | Fixed-shape Go | Result |
| --- | ---: | ---: | ---: |
| One 64-byte SHA-256, median | 43.24 ns | 382.85 ns | 8.86x slower |
| Complete risk kernel, median | 3.579 ms | 20.964 ms | 5.86x slower |
| Complete kernel allocations | 0 | 0 | equal |

Go's selected standard-library path uses processor acceleration and overwhelms
the small amount of general-purpose setup that specialization removes. The
portable scalar rounds therefore have no plausible route through screening on
this toolchain and host.

## Verdict

**Reject the portable scalar fixed-shape SHA path at Level 0.** Do not spend a
90-second screen or AWS comparison on it. The experimental test file was
removed; only this evidence and the decision record remain. Any later SHA work
must preserve the standard library's accelerated path or process multiple
independent messages in parallel strongly enough to offset integration cost.
