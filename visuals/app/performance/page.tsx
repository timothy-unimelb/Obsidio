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
    status: "Measured · run 1 of 3",
    detail: "Untouched, correct starter: general JSON encoding, allocation-heavy risk loop, no explicit concurrency control.",
    score: 1_425_795,
    requests: 570_589,
    tone: "paper",
  },
  {
    number: "01",
    name: "Current bounded Go",
    status: "Measured · run 1 of 3",
    detail: "Combined checkpoint: two risk permits, GOMAXPROCS=2, fixed risk buffer, lean response serialization, and a scratch image.",
    score: 2_790_902,
    requests: 1_115_212,
    tone: "acid",
  },
  { number: "02", name: "Permanent workers + queue", status: "Not implemented", detail: "Measure the scheduling change independently before adding batching.", score: null, requests: null, tone: "pending" },
  { number: "03", name: "Batched risk kernel", status: "Not implemented", detail: "Record batch sizes 2, 4, and 8; keep only the best full-siege result.", score: null, requests: null, tone: "pending" },
  { number: "04", name: "Fixed-shape SHA path", status: "Not implemented", detail: "Specialize rounds 2–50,000 and verify every digest against reference vectors.", score: null, requests: null, tone: "pending" },
  { number: "05", name: "PGO + runtime tuning", status: "Not implemented", detail: "Record PGO and each runtime control separately so their effects remain attributable.", score: null, requests: null, tone: "pending" },
  { number: "06", name: "Native kernel, if justified", status: "Conditional", detail: "Only add a row if profiling still points to the kernel and a C or Rust prototype passes the same tests.", score: null, requests: null, tone: "pending" },
];

const metrics = [
  { label: "Work score", baseline: "1,425,795", current: "2,790,902", delta: "+95.7%", bar: "higher is better" },
  { label: "Weighted work / sec", baseline: "5,280.72", current: "10,336.65", delta: "+95.7%", bar: "higher is better" },
  { label: "Completed requests", baseline: "570,589", current: "1,115,212", delta: "+95.4%", bar: "higher is better" },
  { label: "Requests / sec", baseline: "2,113.29", current: "4,130.41", delta: "+95.4%", bar: "higher is better" },
  { label: "/price p95", baseline: "85.59 ms", current: "50.82 ms", delta: "−40.6%", bar: "< 200 ms" },
  { label: "/stats p95", baseline: "86.62 ms", current: "50.85 ms", delta: "−41.3%", bar: "< 500 ms" },
  { label: "/risk p95", baseline: "542.33 ms", current: "55.84 ms", delta: "−89.7%", bar: "< 1,500 ms" },
  { label: "HTTP errors", baseline: "0.00%", current: "0.00%", delta: "no change", bar: "< 1%" },
];

const latency = [
  { endpoint: "/price", limit: "200 ms", baseline: "85.59", current: "50.82", baselineWidth: "42.8%", currentWidth: "25.4%", tone: "mint" },
  { endpoint: "/stats", limit: "500 ms", baseline: "86.62", current: "50.85", baselineWidth: "17.3%", currentWidth: "10.2%", tone: "amber" },
  { endpoint: "/risk", limit: "1,500 ms", baseline: "542.33", current: "55.84", baselineWidth: "36.2%", currentWidth: "3.7%", tone: "coral" },
];

const evidenceLinks = [
  ["Baseline raw k6 summary", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/results/baseline-summary.json"],
  ["Current raw k6 summary", "https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/results/current-summary.json"],
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
            <div className="recordStatus"><span>LOCAL GRADER-SHAPED RUNS</span><b>2 measured</b><i>5 awaiting implementation</i></div>
          </div>
          <div className="scoreComparison" aria-label="Work score increased from 1,425,795 for the starter to 2,790,902 for the current submission">
            <div className="scoreCompareHead"><span>WORK SCORE</span><small>same script · same caps · one full run each</small></div>
            <div className="scoreBar baselineScore"><span>STARTER</span><i /><strong>1,425,795</strong></div>
            <div className="scoreBar currentScore"><span>CURRENT</span><i /><strong>2,790,902</strong></div>
            <div className="scoreDelta"><strong>+95.7%</strong><span>weighted work</span><small>provisional until repeated</small></div>
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
          <p>The first optimized entry combines work completed before this log existed, so it proves the total improvement but cannot attribute it to one change. From checkpoint 02 onward, each row changes one layer.</p>
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
        <div className="metricsIntro"><span className="sectionNumber inverse">BASELINE → CURRENT</span><h2>The complete<br />comparison.</h2><p>All values come from the two raw k6 summary files generated on 21 August 2026.</p></div>
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
      </section>

      <section className="evidenceSection">
        <div className="evidenceIntro"><span className="sectionNumber inverse">AUDIT TRAIL</span><h2>Keep the evidence<br />beside the claim.</h2><p>The raw summaries, recorded environment, runner, and script fingerprint remain in the repository so future rows can be checked and reproduced.</p></div>
        <div className="fingerprintCard">
          <div><small>GRADING SCRIPT SHA-256</small><code>d7b259eb36cd…9f56998d20f</code></div>
          <div><small>COMPARISON SET</small><strong>2026-08-21 · local-arm64-01</strong></div>
          <div><small>REPETITION STATUS</small><strong>1 of 3 runs per measured checkpoint</strong></div>
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
