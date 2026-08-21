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
    status: "Measured · median of 3 runs",
    detail: "Untouched, correct starter: general JSON encoding, allocation-heavy risk loop, no explicit concurrency control.",
    score: 1_425_795,
    requests: 570_589,
    tone: "paper",
  },
  {
    number: "01",
    name: "Bounded Go",
    status: "Measured · median of 3 runs",
    detail: "Combined checkpoint: two risk permits, GOMAXPROCS=2, fixed risk buffer, lean response serialization, and a scratch image.",
    score: 2_790_902,
    requests: 1_115_212,
    tone: "paper",
  },
  {
    number: "02",
    name: "Permanent workers + queue",
    status: "Measured · current champion · 3 grouped runs",
    detail: "Two permanent hash workers draw from a bounded FIFO. Price and stats bypass it; risk waits deliberately when both workers are occupied.",
    score: 2_874_253,
    requests: 1_149_663,
    tone: "acid",
  },
  { number: "03", name: "Batched risk kernel", status: "Not implemented", detail: "Record batch sizes 2, 4, and 8; keep only the best full-siege result.", score: null, requests: null, tone: "pending" },
  { number: "04", name: "Fixed-shape SHA path", status: "Not implemented", detail: "Specialize rounds 2–50,000 and verify every digest against reference vectors.", score: null, requests: null, tone: "pending" },
  { number: "05", name: "PGO + runtime tuning", status: "Not implemented", detail: "Record PGO and each runtime control separately so their effects remain attributable.", score: null, requests: null, tone: "pending" },
  { number: "06", name: "Native kernel, if justified", status: "Conditional", detail: "Only add a row if profiling still points to the kernel and a C or Rust prototype passes the same tests.", score: null, requests: null, tone: "pending" },
];

const metrics = [
  { label: "Work score", baseline: "1,425,795", current: "2,874,253", delta: "+101.6%", bar: "higher is better" },
  { label: "Weighted work / sec", baseline: "5,280.72", current: "10,645.59", delta: "+101.6%", bar: "higher is better" },
  { label: "Completed requests", baseline: "570,589", current: "1,149,663", delta: "+101.5%", bar: "higher is better" },
  { label: "Requests / sec", baseline: "2,113.29", current: "4,258.09", delta: "+101.5%", bar: "higher is better" },
  { label: "/price p95", baseline: "85.59 ms", current: "24.34 ms", delta: "−71.6%", bar: "< 200 ms" },
  { label: "/stats p95", baseline: "86.62 ms", current: "24.39 ms", delta: "−71.8%", bar: "< 500 ms" },
  { label: "/risk p95", baseline: "542.33 ms", current: "298.54 ms", delta: "−45.0%", bar: "< 1,500 ms" },
  { label: "HTTP errors", baseline: "0.00%", current: "0.00%", delta: "no change", bar: "< 1%" },
];

const latency = [
  { endpoint: "/price", limit: "200 ms", baseline: "85.59", current: "24.34", baselineWidth: "42.8%", currentWidth: "12.2%", tone: "mint" },
  { endpoint: "/stats", limit: "500 ms", baseline: "86.62", current: "24.39", baselineWidth: "17.3%", currentWidth: "4.9%", tone: "amber" },
  { endpoint: "/risk", limit: "1,500 ms", baseline: "542.33", current: "298.54", baselineWidth: "36.2%", currentWidth: "19.9%", tone: "coral" },
];

const evidenceLinks = [
  ["All raw k6 summaries", "https://github.com/timothy-unimelb/Obsidio/tree/draft/benchmarks/results"],
  ["Peak queue timing summary", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/results/workers-timing-summary.json"],
  ["Recorded protocol", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/protocol.json"],
  ["Repeatable runner", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/run-stage.sh"],
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
            <div className="recordStatus"><span>LOCAL GRADER-SHAPED RUNS</span><b>3 measured</b><i>4 awaiting implementation</i></div>
          </div>
          <div className="scoreComparison" aria-label="Median work score increased from 1,425,795 for the starter to 2,874,253 for the current submission">
            <div className="scoreCompareHead"><span>WORK SCORE</span><small>same script · same caps · median of three</small></div>
            <div className="scoreBar baselineScore"><span>STARTER</span><i /><strong>1,425,795</strong></div>
            <div className="scoreBar currentScore"><span>CURRENT</span><i /><strong>2,874,253</strong></div>
            <div className="scoreDelta"><strong>+101.6%</strong><span>weighted work</span><small>three complete runs per checkpoint</small></div>
          </div>
        </div>
      </section>

      <section className="comparabilitySection">
        <div className="comparabilityIntro">
          <span className="sectionNumber">HOW CLOSE IS THIS TO GRADING?</span>
          <h2>The protocol matches.<br />The hardware does not.</h2>
          <p>These results are strong for comparing our own checkpoints. They are not a prediction of the final leaderboard because the organizer’s machine, CPU architecture, and separated load generator are not available locally.</p>
        </div>
        <div className="matchGrid">
          <article><span className="match yes">MATCH</span><h3>Workload</h3><p>The exact published <code>k6/grading.js</code>, including its current ramp and traffic mix.</p></article>
          <article><span className="match yes">MATCH</span><h3>Target limits</h3><p>A fresh Docker container capped at exactly <code>2 CPU</code> and <code>2 GB</code>.</p></article>
          <article><span className="match yes">MATCH</span><h3>Scoring metrics</h3><p>Work score, completed requests, errors, and endpoint-specific p95 latency.</p></article>
          <article><span className="match no">DIFFERS</span><h3>Physical environment</h3><p>Local Apple Silicon host and Linux arm64 VM; the grader keeps load generation separate.</p></article>
        </div>
        <div className="environmentDiagram" aria-label="Local load generator sends the published workload to a resource-capped Linux container">
          <div className="loadMachine"><small>LOAD GENERATOR</small><b>k6 v2.2.0</b><span>macOS · arm64</span></div>
          <div className="loadWire"><span>4m30s · 200 VUs</span><i>→</i><small>60% price · 30% stats · 10% risk</small></div>
          <div className="targetMachine"><small>TARGET</small><b>LINUX CONTAINER</b><div><span>2 CPU</span><span>2 GB</span><span>:8080</span></div></div>
        </div>
        <p className="placeholderWarning"><b>CURRENT PUBLISHED VALUES</b>The challenge file says its thresholds are placeholders. This comparison set must be rerun unchanged when the organizers publish the locked grader.</p>
      </section>

      <section className="section progressionSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">THE PROGRESSION</span><h2>One checkpoint.<br />One complete siege.</h2></div>
          <p>The first optimized entry combines work completed before this log existed, so it proves the total improvement but cannot attribute it to one change. Checkpoint 02 changes one scheduling layer and clears the observed local noise, so it is the current champion; the grouped run order predates protocol v2 and still needs interleaved milestone validation.</p>
        </div>
        <div className="checkpointList">
          {checkpoints.map((checkpoint) => (
            <article className={`checkpoint ${checkpoint.tone}`} key={checkpoint.number}>
              <span className="checkpointNumber">{checkpoint.number}</span>
              <div className="checkpointCopy"><small>{checkpoint.status}</small><h3>{checkpoint.name}</h3><p>{checkpoint.detail}</p></div>
              <div className="checkpointResult">
                {checkpoint.score === null ? <><span className="emptyResult">NO RESULT</span><i /></> : <><small>WORK SCORE</small><strong>{checkpoint.score.toLocaleString("en-AU")}</strong><span>{checkpoint.requests?.toLocaleString("en-AU")} requests</span></>}
              </div>
            </article>
          ))}
        </div>
      </section>

      <section className="metricsSection">
        <div className="metricsIntro"><span className="sectionNumber inverse">BASELINE → CURRENT</span><h2>The complete<br />comparison.</h2><p>Each value is the median of three complete grouped runs generated on 21 August 2026. Nine full score summaries remain in the repository. Protocol v2 records this set as provisional until an interleaved milestone repeats it.</p></div>
        <div className="metricTable" role="table" aria-label="Baseline and current benchmark metrics">
          <div className="metricHeader" role="row"><span>METRIC</span><span>STARTER</span><span>CURRENT</span><span>CHANGE</span><span>BAR</span></div>
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
        <div className="latencyLegend"><span><i className="baseline" />starter</span><span><i className="current" />current</span></div>
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
        <div className="riskTimingCard" aria-label="At peak load, median risk time was 4 milliseconds hashing and 321 milliseconds waiting in the queue">
          <div><span className="sectionNumber">INSIDE A PEAK /RISK REQUEST</span><h3>The queue is the policy.</h3><p>The work did not become slower. It waits so price and stats can finish first.</p></div>
          <div className="riskTimingVisual">
            <div className="riskTimingBar"><i className="hashSlice" /><i className="queueSlice" /></div>
            <div className="riskTimingValues"><span><b>4.00 ms</b>median hashing</span><span><b>320.70 ms</b>median queue wait</span></div>
            <small>30-second diagnostic at 200 VUs · timing headers disabled during scoring runs</small>
          </div>
        </div>
      </section>

      <section className="evidenceSection">
        <div className="evidenceIntro"><span className="sectionNumber inverse">AUDIT TRAIL</span><h2>Keep the evidence<br />beside the claim.</h2><p>The raw summaries, recorded environment, runner, and script fingerprint remain in the repository so future rows can be checked and reproduced.</p></div>
        <div className="fingerprintCard">
          <div><small>GRADING SCRIPT SHA-256</small><code>d7b259eb36cd…9f56998d20f</code></div>
          <div><small>COMPARISON SET</small><strong>2026-08-21 · local-arm64-01</strong></div>
          <div><small>REPETITION STATUS</small><strong>Legacy grouped set · provisional under protocol v2</strong></div>
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
