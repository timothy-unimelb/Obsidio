import type { Metadata } from "next";
import Link from "next/link";
import { SiteHeader } from "../components/SiteHeader";

export const metadata: Metadata = {
  title: "Testing Protocol — Obsidio",
  description: "How Obsidio performance changes move from correctness checks to screening, exact grading runs, and repeated milestone evidence.",
};

const levels = [
  {
    number: "0",
    name: "Correctness",
    duration: "SECONDS",
    description: "Endpoint tests, independent risk vectors, allocations, and focused microbenchmarks.",
    result: "Reject incorrect ideas before load testing.",
    tone: "mint",
  },
  {
    number: "1",
    name: "Screen",
    duration: "90 SECONDS",
    description: "The same 60/30/10 traffic mix and proportional ramp, compressed for candidate selection.",
    result: "A promotion signal, never a published score.",
    tone: "amber",
  },
  {
    number: "2",
    name: "Full comparison",
    duration: "4M30S / RUN",
    description: "The untouched published grading script against the champion and promoted candidate.",
    result: "Exact grader-shaped evidence.",
    tone: "coral",
  },
  {
    number: "3",
    name: "Milestone",
    duration: "6 FULL RUNS",
    description: "Three complete runs per implementation, interleaved to expose drift and noise.",
    result: "Median, range, raw summaries, and a verdict.",
    tone: "acid",
  },
];

const decisions = [
  { range: "> 2%", label: "PROMOTE", detail: "Move a screening winner to the exact full comparison.", width: "100%", tone: "acid" },
  { range: "0.5–2%", label: "PAIR", detail: "Bracket the candidate with champion runs or repeat the pair.", width: "68%", tone: "amber" },
  { range: "< 0.5%", label: "UNRESOLVED", detail: "Do not claim a gain unless profiling explains it and more evidence confirms it.", width: "42%", tone: "paper" },
  { range: "FAIL", label: "REJECT", detail: "Any correctness failure or error rate at or above the current 1% gate.", width: "82%", tone: "coral" },
];

const evidence = [
  ["SOURCE", "Git SHA + meaningful dirty files"],
  ["WORKLOAD", "Profile + script SHA-256 + k6 version"],
  ["ENVIRONMENT", "Environment ID + relationship + CPU mode"],
  ["ARTIFACT", "Image ID + raw-summary path"],
  ["OUTCOME", "Score + requests + errors + three p95 values"],
  ["DECISION", "Pending, kept, reverted, invalid, or superseded"],
];

export default function TestingPage() {
  return (
    <main className="testingPage">
      <SiteHeader active="testing" />

      <section className="testingHero">
        <div className="eyebrow light"><span>05</span> Testing protocol</div>
        <div className="testingHeroGrid">
          <div>
            <h1>From change<br />to <em>evidence.</em></h1>
            <p className="lede lightText">Not every idea needs six long runs. The protocol spends time in proportion to uncertainty: fast checks first, exact grading only for candidates that earn it.</p>
            <div className="protocolAuthority"><b>AUTHORITATIVE SOURCE</b><code>benchmarks/PROTOCOL.md</code></div>
          </div>
          <div className="testFunnel" aria-label="Testing narrows from many ideas through correctness, screening, exact comparison, and milestone validation">
            <div className="funnelIdea"><span>MANY IDEAS</span><i>cheap to test</i></div>
            <div className="funnelStep f0"><b>0</b><span>correctness</span><small>seconds</small></div>
            <div className="funnelStep f1"><b>1</b><span>screen</span><small>90s</small></div>
            <div className="funnelStep f2"><b>2</b><span>full</span><small>4m30s</small></div>
            <div className="funnelStep f3"><b>3</b><span>milestone</span><small>median + range</small></div>
            <div className="funnelOutcome">ONE DEFENSIBLE CHANGE</div>
          </div>
        </div>
      </section>

      <section className="testingEnvironment">
        <div className="testingEnvironmentIntro">
          <span className="sectionNumber">REFERENCE ENVIRONMENT</span>
          <h2>Separate the load<br />from the target.</h2>
          <p>The judge guarantees x86-64 Linux, a two-CPU cgroup quota, and 2 GiB of memory—not a CPU model. The reference environment reproduces those known constraints without pretending to know the final silicon.</p>
        </div>
        <div className="machineDiagram" aria-label="A separate k6 host sends traffic over a private network to an x86 Linux target whose container is capped at two CPUs and two gibibytes">
          <article className="machine loadHost">
            <small>LOAD HOST</small>
            <div className="machineCores"><i /><i /><i /><i /></div>
            <h3>k6</h3>
            <p>Fixed-performance x86 Linux. Uncapped enough that it cannot become the bottleneck.</p>
          </article>
          <div className="privateWire"><span>PRIVATE NETWORK</span><b>→</b><small>same zone · low latency</small></div>
          <article className="machine targetHost">
            <small>TARGET HOST · MORE THAN 2 VCPUS VISIBLE</small>
            <div className="quotaBox"><span>CONTAINER</span><strong>2.0 CPU</strong><strong>2 GiB</strong><code>cpu.max 200000 100000</code></div>
            <p>No cpuset and no assumed SHA, AVX, cache, or processor generation.</p>
          </article>
        </div>
        <div className="environmentContrast">
          <div><b>LOCAL</b><span>Useful relative evidence</span><small>arm64 · shared physical host</small></div>
          <i>≠</i>
          <div><b>REFERENCE</b><span>Cleaner promotion evidence</span><small>x86-64 · separated load</small></div>
          <i>≠</i>
          <div><b>JUDGE</b><span>Final score</span><small>specific CPU deliberately unknown</small></div>
        </div>
      </section>

      <section className="section testLevelsSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">FOUR LEVELS</span><h2>Spend time only<br />after confidence grows.</h2></div>
          <p>A candidate moves forward only when the previous level supplies the evidence needed for the next decision.</p>
        </div>
        <div className="testLevelGrid">
          {levels.map((level) => (
            <article className={`testLevel ${level.tone}`} key={level.number}>
              <div className="testLevelTop"><span>{level.number}</span><b>{level.duration}</b></div>
              <div className="levelPulse"><i /><i /><i /></div>
              <h3>{level.name}</h3>
              <p>{level.description}</p>
              <small>{level.result}</small>
            </article>
          ))}
        </div>
      </section>

      <section className="sequenceSection">
        <div className="sequenceIntro">
          <span className="sectionNumber inverse">ORDER CONTROLS DRIFT</span>
          <h2>Compare beside,<br />not across time.</h2>
          <p>Cloud hosts, local thermals, and background activity drift. Alternating the champion and candidate makes that drift visible instead of quietly assigning it to the code.</p>
        </div>
        <div className="sequenceCards">
          <article>
            <header><span>SCREEN OR SMALL CHANGE</span><b>3 RUNS</b></header>
            <div className="sequenceTrack"><i className="champion">A</i><b>→</b><i className="candidate">B</i><b>→</b><i className="champion">A</i></div>
            <p>Candidate B is interpreted against the champion before and after it.</p>
          </article>
          <article>
            <header><span>MILESTONE</span><b>6 RUNS</b></header>
            <div className="sequenceTrack long"><i className="champion">A</i><b>→</b><i className="candidate">B</i><b>→</b><i className="candidate">B</i><b>→</b><i className="champion">A</i><b>→</b><i className="champion">A</i><b>→</b><i className="candidate">B</i></div>
            <p>Each implementation receives three full runs; report the median and range.</p>
          </article>
        </div>
      </section>

      <section className="section decisionSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">DECISION RULES</span><h2>A number becomes<br />a claim carefully.</h2></div>
          <p>Score is not the only output. Correctness, the error gate, latency headroom, environmental validity, and observed noise can all veto an apparent gain.</p>
        </div>
        <div className="decisionBoard">
          {decisions.map((decision) => (
            <article key={decision.label}>
              <strong>{decision.range}</strong>
              <div><i className={decision.tone} style={{ width: decision.width }}><span>{decision.label}</span></i></div>
              <p>{decision.detail}</p>
            </article>
          ))}
        </div>
        <div className="vetoStrip"><b>CORRECTNESS</b><i>+</i><b>ERRORS</b><i>+</i><b>LATENCY</b><i>+</i><b>SCORE</b><i>+</i><b>NOISE</b><span>ALL FIVE INFORM THE VERDICT</span></div>
      </section>

      <section className="evidenceChainSection">
        <div className="evidenceChainIntro">
          <span className="sectionNumber inverse">CHAIN OF CUSTODY</span>
          <h2>Keep enough context<br />to believe the result later.</h2>
          <p>Raw output without source, workload, and environment identity cannot answer what caused a change.</p>
        </div>
        <div className="evidenceChain">
          {evidence.map(([label, detail], index) => (
            <div className="evidenceNode" key={label}><span>{String(index + 1).padStart(2, "0")}</span><small>{label}</small><b>{detail}</b></div>
          ))}
        </div>
        <div className="sourceLinks">
          <a href="https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/PROTOCOL.md" target="_blank" rel="noreferrer"><span>Canonical Markdown protocol</span><b>↗</b></a>
          <a href="https://github.com/timothy-unimelb/Obsidio/blob/draft/benchmarks/protocol.json" target="_blank" rel="noreferrer"><span>Machine-readable settings</span><b>↗</b></a>
          <a href="https://github.com/timothy-unimelb/Obsidio/blob/draft/AGENTS.md" target="_blank" rel="noreferrer"><span>Future-session guidance</span><b>↗</b></a>
        </div>
      </section>

      <section className="testingFooter">
        <div><span className="sectionNumber">THE RESULT</span><h2>Fast iteration.<br /><em>Slow confidence.</em></h2><p>Most ideas finish in minutes. Only accepted milestones pay for repeated full sieges.</p></div>
        <Link className="backLink" href="/performance"><b>←</b><span>Performance record</span></Link>
      </section>
    </main>
  );
}
