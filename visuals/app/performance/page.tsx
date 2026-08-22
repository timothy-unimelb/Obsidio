import type { Metadata } from "next";
import Link from "next/link";
import { SiteHeader } from "../components/SiteHeader";

export const metadata: Metadata = {
  title: "Performance Record — Obsidio",
  description: "A reproducible, run-by-run record of Obsidio performance using the published grading workload and container limits.",
};

const checkpoints = [
  {
    number: "00",
    name: "Go starter",
    status: "Measured · local · median of 3 runs",
    detail: "Untouched, correct starter: general JSON encoding, allocation-heavy risk loop, no explicit concurrency control.",
    score: 1_425_795,
    requests: 570_589,
    environment: "local arm64",
    tone: "paper",
  },
  {
    number: "01",
    name: "Bounded Go",
    status: "Measured · local · median of 3 runs",
    detail: "Two risk permits, GOMAXPROCS=2, fixed risk buffer, lean response serialization, and a scratch image.",
    score: 2_790_902,
    requests: 1_115_212,
    environment: "local arm64",
    tone: "paper",
  },
  {
    number: "02",
    name: "Permanent workers + queue",
    status: "Measured · local · 3 grouped runs",
    detail: "Two permanent hash workers draw from a bounded FIFO. Price and stats bypass it; risk waits deliberately when both workers are occupied.",
    score: 2_874_253,
    requests: 1_149_663,
    environment: "local arm64",
    tone: "paper",
  },
  {
    number: "03",
    name: "Packed lowercase hex",
    status: "Measured · local bracketed full run",
    detail: "Profiling found hex expansion—not SHA compression—at 62% of CPU. A native-endian pair table halves stores while preserving every required lowercase byte.",
    score: 2_852_984,
    requests: 1_141_016,
    environment: "local arm64",
    tone: "paper",
  },
  {
    number: "04",
    name: "Compact hex unrolling",
    status: "Measured · separated x86 full bracket · +1.70%",
    detail: "Expands four digest bytes per loop. First checkpoint confirmed on a separate x86 load host under the same 2 CPU / 2 GiB target cap.",
    score: 1_987_151,
    requests: 796_456,
    environment: "x86 · c7i",
    tone: "paper",
  },
  { number: "05", name: "Go PGO", status: "Rejected · Level 0", detail: "A representative Go 1.26.6 profile made the risk kernel 19.67% slower and reintroduced 50,000 allocations per request, so it was stopped before load testing.", score: null, requests: null, environment: "", tone: "pending" },
  { number: "06", name: "Fixed-shape scalar SHA", status: "Rejected · Level 0", detail: "A portable two-block implementation was correct and allocation-free, but 5.86× slower than Go's hardware path and still 22.92% slower with optional CPU features disabled.", score: null, requests: null, environment: "", tone: "pending" },
  {
    number: "07",
    name: "Interleaved multi-lane batching",
    status: "Measured · separated x86 full bracket · +2.21%",
    detail: "Each worker drains up to four queued jobs and interleaves their chains so the core overlaps SHA latency. +9.11% on Apple Silicon, +2.21% on SHA-NI x86, neutral without hardware SHA.",
    score: 2_062_911,
    requests: 826_875,
    environment: "x86 · c7i",
    tone: "paper",
  },
  {
    number: "08",
    name: "Two-lane SHA-NI kernel",
    status: "Measured · separated x86 full bracket · +21.76%",
    detail: "One assembly routine hashes both lanes: Go's SHA-NI schedule with the lanes interleaved, and a precomputed schedule for the constant padding block. CPUID-gated with the Go path as fallback. Per-chain cost 6.21 → 2.99 ms.",
    score: 2_545_521,
    requests: 1_018_708,
    environment: "x86 · c7i",
    tone: "acid",
  },
  {
    number: "09",
    name: "In-kernel hex + yield between chunks",
    status: "Measured · separated x86 full bracket · +53.8%",
    detail: "Hex encoding moved into the kernel (−10% per chain) showed nothing at the screen: cheap requests were waiting ~17 ms for a core behind an unpreemptible assembly loop. Calling the kernel in 256-round chunks with a yield between them cut /price p95 from 43 ms to 10 ms and let the closed-loop request rate rise.",
    score: 3_598_675,
    requests: 1_440_622,
    environment: "x86 · c7i",
    tone: "acid",
  },
  { number: "10", name: "Durable POST /price", status: "Measured · inert under the published load", detail: "An fsynced write-ahead log on a /data volume survives docker kill for the persistence bonus. The scored siege is read-only, so the bracket was flat: 3,607,469 → 3,601,743 → 3,611,091 with zero errors.", score: null, requests: null, environment: "x86 · c7i", tone: "pending" },
  {
    number: "11",
    name: "Budgeted shedding governor",
    status: "Measured · current champion · separated x86 full bracket · +17.2%",
    detail: "Ported from Advait's design: when no worker is idle and anyone is already parked, a new /risk arrival is refused in a millisecond inside an 88bp error budget; waiters are served newest-first and stale ones skipped; the budget is reserved atomically. A refused client comes straight back with cheap scoring traffic. Runs at 0.85% errors by design; RISK_SHED=0 restores the zero-error build.",
    score: 4_854_704,
    requests: 2_009_080,
    environment: "x86 · c7i (later instance)",
    tone: "acid",
  },
  { number: "12", name: "Late-serve tail fix", status: "Rejected · 800-VU stress", detail: "Serving held stale waiters after twice their patience bounded the worst wait at 2.4 s and was flat at the published load, but at four times the peak late serves exceeded 5% of risk samples: errors 1.49% and risk p95 2.4 s. Reverted; held waiters stay held.", score: null, requests: null, environment: "x86 · c7i", tone: "pending" },
];

const headToHead = [
  { build: "Advait's frozen build c0c8f95 (control A1)", score: "4,321,831", errors: "0.84%", price: "6.79 ms", risk: "76 ms" },
  { build: "Ours c6bf541", score: "4,835,626", errors: "0.87%", price: "3.35 ms", risk: "89 ms" },
  { build: "Advait's frozen build c0c8f95 (control A2)", score: "4,285,200", errors: "0.85%", price: "7.06 ms", risk: "74 ms" },
];

const metrics = [
  { label: "Work score", baseline: "4,143,222", current: "4,854,704", delta: "+17.2%", bar: "higher is better" },
  { label: "Weighted work / sec", baseline: "15,345", current: "17,980", delta: "+17.2%", bar: "higher is better" },
  { label: "Completed requests", baseline: "1,659,471", current: "2,009,080", delta: "+21.1%", bar: "higher is better" },
  { label: "Requests / sec", baseline: "6,146", current: "7,441", delta: "+21.1%", bar: "higher is better" },
  { label: "/price p95", baseline: "10.91 ms", current: "9.35 ms", delta: "−14.3%", bar: "< 200 ms" },
  { label: "/stats p95", baseline: "10.93 ms", current: "9.34 ms", delta: "−14.5%", bar: "< 500 ms" },
  { label: "/risk p95", baseline: "215.35 ms", current: "79.50 ms", delta: "−63.1%", bar: "< 1,500 ms" },
  { label: "HTTP errors", baseline: "0.00%", current: "0.85%", delta: "by design", bar: "< 1%" },
];

const latency = [
  { endpoint: "/price", limit: "200 ms", baseline: "10.91", current: "9.35", baselineWidth: "5.5%", currentWidth: "4.7%", tone: "mint" },
  { endpoint: "/stats", limit: "500 ms", baseline: "10.93", current: "9.34", baselineWidth: "2.2%", currentWidth: "1.9%", tone: "amber" },
  { endpoint: "/risk", limit: "1,500 ms", baseline: "215.35", current: "79.50", baselineWidth: "14.4%", currentWidth: "5.3%", tone: "coral" },
];

const evidenceLinks = [
  ["All raw k6 summaries", "https://github.com/timothy-unimelb/Obsidio/tree/draft/benchmarks/results"],
  ["Peak queue timing summary", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/results/workers-timing-summary.json"],
  ["Recorded protocol", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/protocol.json"],
  ["Current champion report: the shedding governor", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/experiments/2026-08-22-governor.md"],
  ["In-kernel hex and yield report", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/experiments/2026-08-22-kernel-hex-yield.md"],
  ["SHA-NI kernel report", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/experiments/2026-08-22-shani-kernel.md"],
  ["Multi-lane batching report", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/experiments/2026-08-22-risk-lanes.md"],
  ["Compact hex unrolling report", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/experiments/2026-08-22-packed-champion-profile.md"],
  ["Rejected PGO experiment", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/experiments/2026-08-22-go-pgo.md"],
  ["Rejected fixed-shape SHA experiment", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/experiments/2026-08-22-fixed-shape-sha.md"],
];

export default function PerformancePage() {
  return (
    <main className="performancePage">
      <SiteHeader active="performance" />

      <section className="performanceHero">
        <div className="eyebrow light"><span>04</span> Performance record</div>
        <div className="performanceHeroGrid">
          <div>
            <h1>Performance,<br /><em>run by run.</em></h1>
            <p className="lede lightText">A checkpoint enters this record only after a complete published siege. Measured stages show raw results; unimplemented ideas stay visibly empty.</p>
            <div className="recordStatus"><span>GRADER-SHAPED RUNS</span><b>9 measured</b><i>2 rejected at Level 0 · 1 rejected under overload · 1 inert</i></div>
          </div>
          <div className="scoreComparison" aria-label="In the latest separated x86 full bracket, work score increased from 4,143,222 for the stronger zero-error reference to 4,854,704 for the shedding governor">
            <div className="scoreCompareHead"><span>LATEST X86 FULL BRACKET</span><small>separate load host · same script · same caps</small></div>
            <div className="scoreBar baselineScore"><span>ZERO-ERROR BUILD A1</span><i /><strong>4,143,222</strong></div>
            <div className="scoreBar currentScore"><span>+ GOVERNOR B1</span><i /><strong>4,854,704</strong></div>
            <div className="scoreDelta"><strong>+17.2%</strong><span>vs stronger side</span><small>−0.03% control drift · 0.85% errors by design</small></div>
          </div>
        </div>
      </section>

      <section className="comparabilitySection">
        <div className="comparabilityIntro">
          <span className="sectionNumber">HOW CLOSE IS THIS TO GRADING?</span>
          <h2>The protocol matches.<br />The silicon is still unknown.</h2>
          <p>The latest comparison uses Linux x86-64 and a separate load generator, matching the judge’s disclosed shape. It is still not a leaderboard prediction because the organizer has not specified the final CPU model.</p>
        </div>
        <div className="matchGrid">
          <article><span className="match yes">MATCH</span><h3>Workload</h3><p>The exact published <code>k6/grading.js</code>, including its current ramp and traffic mix.</p></article>
          <article><span className="match yes">MATCH</span><h3>Target limits</h3><p>A fresh Docker container capped at exactly <code>2 CPU</code> and <code>2 GB</code>.</p></article>
          <article><span className="match yes">MATCH</span><h3>Scoring metrics</h3><p>Work score, completed requests, errors, and endpoint-specific p95 latency.</p></article>
          <article><span className="match no">UNKNOWN</span><h3>Exact silicon</h3><p>The reference run used a fixed-performance C7i host; the judge may use a different x86-64 CPU.</p></article>
        </div>
        <div className="environmentDiagram" aria-label="A separate Linux x86 load generator sends the published workload to a resource-capped Linux x86 container">
          <div className="loadMachine"><small>LOAD GENERATOR</small><b>k6 v2.2.0</b><span>separate Linux x86-64 host</span></div>
          <div className="loadWire"><span>4m30s · 200 VUs</span><i>→</i><small>60% price · 30% stats · 10% risk</small></div>
          <div className="targetMachine"><small>TARGET</small><b>LINUX CONTAINER</b><div><span>2 CPU</span><span>2 GB</span><span>:8080</span></div></div>
        </div>
        <p className="placeholderWarning"><b>CURRENT PUBLISHED VALUES</b>The challenge file says its thresholds are placeholders. This comparison set must be rerun unchanged when the organizers publish the locked grader.</p>
      </section>

      <section className="section progressionSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">THE PROGRESSION</span><h2>One checkpoint.<br />One complete siege.</h2></div>
          <p>Checkpoints 00–03 are local Apple Silicon medians; from 04 onward every row is a separated x86 bracket, so absolute scores are only comparable within each environment — and between AWS instances they move by about 15% (the zero-error build scored 3.60M on one box and 4.14M on another). Checkpoints 05, 06, and 12 record useful failures. Checkpoint 11 is the current champion: it spends an error budget on instant refusals so that refused clients keep ordering cheap work. A six-run finalist milestone remains outstanding.</p>
        </div>
        <div className="checkpointList">
          {checkpoints.map((checkpoint) => (
            <article className={`checkpoint ${checkpoint.tone}`} key={checkpoint.number}>
              <span className="checkpointNumber">{checkpoint.number}</span>
              <div className="checkpointCopy"><small>{checkpoint.status}</small><h3>{checkpoint.name}</h3><p>{checkpoint.detail}</p></div>
              <div className="checkpointResult">
                {checkpoint.score === null ? <><span className="emptyResult">NO RESULT</span><i /></> : <><small>WORK SCORE · {checkpoint.environment}</small><strong>{checkpoint.score.toLocaleString("en-AU")}</strong><span>{checkpoint.requests?.toLocaleString("en-AU")} requests</span></>}
              </div>
            </article>
          ))}
        </div>
      </section>

      <section className="metricsSection">
        <div className="metricsIntro"><span className="sectionNumber inverse">ZERO-ERROR → GOVERNOR</span><h2>The x86<br />reference.</h2><p>Both columns come from the same separated x86 comparison set on 22 August 2026: the zero-error control and the current champion, each a complete 4m30s published siege. The champion trades 0.85% refused risk requests for 21% more completed requests. A six-run interleaved milestone will replace these single brackets with medians and ranges.</p></div>
        <div className="metricTable" role="table" aria-label="Starter and permanent-worker benchmark metrics">
          <div className="metricHeader" role="row"><span>METRIC</span><span>ZERO-ERROR</span><span>CHAMPION</span><span>CHANGE</span><span>BAR</span></div>
          {metrics.map((metric) => (
            <div className="metricRow" role="row" key={metric.label}><b>{metric.label}</b><span>{metric.baseline}</span><span>{metric.current}</span><strong>{metric.delta}</strong><small>{metric.bar}</small></div>
          ))}
        </div>
      </section>

      <section className="section latencySection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">LATENCY HEADROOM</span><h2>Distance from<br />each p95 bar.</h2></div>
          <p>Each track ends at the published threshold. Shorter bars mean more headroom; they do not imply the final grader will produce the same absolute time.</p>
        </div>
        <div className="latencyLegend"><span><i className="baseline" />zero-error build</span><span><i className="current" />current champion</span></div>
        <div className="latencyTracks">
          {latency.map((item) => (
            <article key={item.endpoint}>
              <div className="latencyLabel"><code>{item.endpoint}</code><span>published bar</span><strong>{item.limit}</strong></div>
              <div className="latencyPair">
                <div><span style={{ width: item.baselineWidth }} className={`baseline ${item.tone}`} /><b>{item.baseline} ms</b></div>
                <div><span style={{ width: item.currentWidth }} className={`current ${item.tone}`} /><b>{item.current} ms</b></div>
              </div>
            </article>
          ))}
        </div>
        <div className="riskTimingCard" aria-label="Head to head on one x86 box, ours served 4,835,626 against Advait's 4,321,831 and 4,285,200 at the same error budget">
          <div><span className="sectionNumber">HEAD TO HEAD · SAME BOX · SAME BUDGET</span><h3>Two governors, one difference.</h3><p>Advait's frozen build bracketed ours on the exact grader. Risk latency is equivalent; the gap is the cheap path — a price median of 3.4 ms against 6.8–7.1 ms — which a budget-limited closed loop converts directly into requests.</p></div>
          <div className="riskTimingVisual">
            <div className="metricTable" role="table" aria-label="Head-to-head results">
              <div className="metricHeader" role="row"><span>BUILD</span><span>SCORE</span><span>ERRORS</span><span>PRICE MEDIAN</span><span>RISK P95</span></div>
              {headToHead.map((row) => (
                <div className="metricRow" role="row" key={row.build}><b>{row.build}</b><span>{row.score}</span><span>{row.errors}</span><strong>{row.price}</strong><small>{row.risk}</small></div>
              ))}
            </div>
            <small>+11.9% vs the stronger control · −0.85% drift · candidates for the gap: 256-round yield cadence vs ~8,192, 4-lane batches vs pairs</small>
          </div>
        </div>
      </section>

      <section className="evidenceSection">
        <div className="evidenceIntro"><span className="sectionNumber inverse">AUDIT TRAIL</span><h2>Keep the evidence<br />beside the claim.</h2><p>The raw summaries, recorded environment, runner, and script fingerprint remain in the repository so future rows can be checked and reproduced.</p></div>
        <div className="fingerprintCard">
          <div><small>GRADING SCRIPT SHA-256</small><code>d7b259eb36cd…9f56998d20f</code></div>
          <div><small>LATEST COMPARISON SET</small><strong>2026-08-22 · governor-x86-full · advait-vs-tim-x86-full</strong></div>
          <div><small>REPETITION STATUS</small><strong>A → B → A full bracket · pending six-run milestone</strong></div>
        </div>
        <div className="evidenceLinks">
          {evidenceLinks.map(([label, href]) => <a href={href} target="_blank" rel="noreferrer" key={label}><span>{label}</span><b>↗</b></a>)}
        </div>
      </section>

      <section className="performanceFooter">
        <div><span className="sectionNumber">NEXT ENTRY</span><h2>Change one thing.<br /><em>Run the same siege.</em></h2></div>
        <Link className="backLink" href="/optimizations"><b>←</b><span>Optimization guide</span></Link>
      </section>
    </main>
  );
}
