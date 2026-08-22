import type { Metadata } from "next";
import Link from "next/link";
import { SiteHeader } from "../components/SiteHeader";

export const metadata: Metadata = {
  title: "Optimization Guide — Obsidio",
  description: "A visual guide to where the Obsidio service spends time, how each optimization works, and how to choose what to test next.",
};

const roadmap = [
  {
    number: "01",
    question: "Where does the time go?",
    title: "Establish evidence",
    copy: "Save correctness vectors, a CPU profile, an allocation profile, and several full-load baseline runs. Without these, later comparisons are guesses.",
  },
  {
    number: "02",
    question: "How should heavy work enter the CPUs?",
    title: "Control scheduling",
    copy: "Use permanent risk workers and a bounded queue. This separates HTTP request handling from the decision to begin expensive hashing.",
  },
  {
    number: "03",
    question: "What surrounds every hash?",
    title: "Pack the hex feedback",
    copy: "Profiling found lowercase hex expansion using more CPU than SHA compression. A two-byte lookup now preserves the exact feedback string with half as many stores.",
  },
  {
    number: "04",
    question: "Can each CPU cycle carry more useful work?",
    title: "Test small batches",
    copy: "Advance 2, 4, then 8 independent hash chains together. Keep a short fill deadline so the oldest request does not wait too long for companions.",
  },
  {
    number: "05",
    question: "What repeats 49,999 times?",
    title: "Specialize the fixed input shape",
    copy: "After the first hash, every message is 64 ASCII bytes. The message changes, but its size and SHA-256 padding layout do not.",
  },
  {
    number: "06",
    question: "What can the existing toolchain improve?",
    title: "Apply profile-guided tuning",
    copy: "Try Go PGO, then sweep GC, worker count, queue depth, and batch size one variable at a time under the 2 CPU / 2 GB limit.",
  },
  {
    number: "07",
    question: "Is request handling visible in the profile?",
    title: "Trim the edges if necessary",
    copy: "Only then compare HTTP parsing, query handling, JSON assembly, symbol lookup, and buffer reuse. These affect many requests, but do little to the SHA loop.",
  },
  {
    number: "08",
    question: "Does pure Go still limit the measured kernel?",
    title: "Evaluate a native kernel",
    copy: "Keep the Go server. Prototype only a narrow batched SHA function in C or Rust, and retain it only if the full mixed workload improves materially.",
  },
];

const options = [
  {
    number: "01",
    area: "Scheduling",
    title: "Permanent risk workers",
    change: "Risk handlers place jobs into one bounded FIFO. Two long-lived workers perform the hashes and send results back.",
    helps: "The number of active hash loops becomes explicit, and workers can collect jobs into batches without coupling that logic to HTTP handlers.",
    cost: "Every risk request gains a queue hand-off. A queue that is too long hides overload as latency.",
    evidence: "Queue wait, risk service time, cheap-path p95, weighted work score",
    outcome: "Kept · local +3% over bounded permits; the base every later row builds on",
  },
  {
    number: "02",
    area: "CPU kernel",
    title: "Batch independent requests",
    change: "One worker advances several unrelated SHA feedback chains side by side instead of completing one entire chain before touching the next.",
    helps: "Dependencies inside one chain leave limited instruction-level parallelism. Independent chains give the CPU more unrelated work to interleave.",
    cost: "Waiting to fill a batch adds latency. Larger batches use more registers and may reduce rather than improve throughput.",
    evidence: "Hashes per second at batch sizes 1, 2, 4, 8; batch-fill wait; risk p95",
    outcome: "Kept · 4 lanes: +9.11% local, +2.21% on x86 in Go; then a two-lane SHA-NI assembly routine took it to +21.76% on x86",
  },
  {
    number: "03",
    area: "CPU kernel",
    title: "Specialize the 64-byte rounds",
    change: "Keep the generic first round, then use a path designed specifically for a 64-byte ASCII-hex message and its fixed second padding block.",
    helps: "Rounds 2–50,000 share the same message length and padding structure, so general-purpose length and padding setup can be removed.",
    cost: "A small padding or state error produces a plausible but wrong digest. The standard library may already optimize much of this path in assembly.",
    evidence: "Reference-vector equality; nanoseconds per 50,000-round chain; CPU profile",
    outcome: "Rejected in scalar Go (5.86× slower than the hardware path); kept inside the assembly kernel, where the constant padding block's schedule is precomputed",
  },
  {
    number: "04",
    area: "Compiler",
    title: "Go profile-guided optimization",
    change: "Build with a CPU profile captured from the real 60/30/10 request mix rather than from the risk endpoint alone.",
    helps: "The compiler can make better inlining, layout, and devirtualization decisions around code paths that the service actually uses.",
    cost: "The improvement is usually incremental, and an unrepresentative or stale profile can guide the build in the wrong direction.",
    evidence: "Binary-to-binary full-load comparison across repeated runs",
    outcome: "Rejected at Level 0 · kernel 19.67% slower, 50,000 allocations per request reintroduced",
  },
  {
    number: "05",
    area: "Runtime",
    title: "GC, queue, and worker tuning",
    change: "Sweep one bounded set of values for GOGC, queue capacity, worker count, and batch size while the container remains capped.",
    helps: "It can reduce scheduler and collection overhead and reveal whether the system is CPU-limited, queue-limited, or memory-sensitive.",
    cost: "The controls interact. For example, a higher GOGC may save CPU while consuming enough memory to make a long run unstable.",
    evidence: "CPU time, heap peak, GC pauses, queue wait, endpoint p95s",
    outcome: "Partly done · yield cadence 256 rounds beats 2,048 by 9.8%; lane and worker sweeps under the new regime still open",
  },
  {
    number: "06",
    area: "Request path",
    title: "HTTP and JSON handling",
    change: "Compare direct query parsing, prebuilt response fragments, tighter buffer reuse, or a leaner HTTP stack.",
    helps: "Small savings apply to price and stats as well as risk, and fewer allocations reduce background GC work.",
    cost: "Custom protocol code is harder to validate, while hashing still dominates the expensive endpoint.",
    evidence: "Allocations per endpoint; non-hash CPU share; price and stats throughput",
    outcome: "Open · now the largest remaining lever: with risk cheap, the request rate is bound by net/http overhead on 1.4M cheap requests",
  },
  {
    number: "07",
    area: "Admission",
    title: "Pressure-aware scheduling",
    change: "Temporarily reduce risk admission when cheap-request latency or runnable work rises, then restore it after the fast path clears.",
    helps: "This can protect the qualifying latency bars during bursts without changing any response or skipping required work.",
    cost: "A noisy feedback signal can oscillate, and protecting price too aggressively lowers valuable risk throughput.",
    evidence: "Per-endpoint p95 during the ramp; worker utilization; weighted score",
    outcome: "Kept twice. The yield (chunk the kernel every 256 rounds) cut /price p95 43 → 10 ms for +53.8%. Then a budgeted governor — instant 503 when no worker is idle, newest-first service, 88bp error budget reserved atomically — added +17.2% at 0.85% errors and passes every bar at 4× the peak. Serving stale waiters late was measured to break two bars and reverted",
  },
  {
    number: "08",
    area: "Implementation",
    title: "A C or Rust hash kernel",
    change: "Pass a batch of input states across one narrow native boundary, run only the hot hash loop there, and return a batch of digests.",
    helps: "Native code gives direct control over SIMD layout, instruction scheduling, and CPU-specific implementations while Go keeps the server simple.",
    cost: "CGO, cross-compilation, portability, and debugging become more complex. Small calls can lose their gain to boundary overhead.",
    evidence: "End-to-end gain after FFI overhead; architecture-specific builds; reference vectors",
    outcome: "Done without FFI · Go assembly, no cgo: CPUID-gated SHA-NI routine with the Go path as fallback everywhere else",
  },
];

const checks = [
  ["Correctness", "Every output matches the reference, including empty, short, long, and escaped seeds."],
  ["Throughput", "The weighted score rises across repeated full-length runs, not only one unusually good run."],
  ["Latency", "Price, stats, and risk stay below their own p95 limits throughout the ramp and peak."],
  ["Resources", "The container stays stable within 2 CPU and 2 GB, on the same architecture used for comparison."],
];

export default function OptimizationsPage() {
  return (
    <main className="optimizationPage">
      <SiteHeader active="optimizations" />

      <section className="optHero">
        <div className="eyebrow light"><span>03</span> Optimization guide</div>
        <div className="optHeroGrid">
          <div>
            <h1>A map of what<br />to improve <em>next.</em></h1>
            <p className="lede lightText">The current service is already correct, bounded, and allocation-light. This page explains where further improvements could act, how they might help, and what evidence would justify keeping them.</p>
            <div className="optReadingKey">
              <span><b>1</b> Understand the path</span>
              <span><b>2</b> Change one layer</span>
              <span><b>3</b> Measure the whole system</span>
            </div>
          </div>

          <div className="requestPath" aria-label="Current request path through the service">
            <div className="pathTitle"><span>CURRENT SYSTEM</span><small>where each optimization acts</small></div>
            <div className="requestIn"><b>HTTP REQUEST</b><span>price · stats · risk</span></div>
            <div className="pathArrow">↓</div>
            <div className="routeSplit">
              <div className="fastBranch">
                <small>FAST BRANCH</small>
                <span className="mint">PRICE</span><span className="amber">STATS</span>
                <i>direct response work</i>
              </div>
              <div className="heavyBranch">
                <small>HEAVY BRANCH</small>
                <span className="coral">RISK</span><b>→</b><span>QUEUE</span><b>→</b><span>2 WORKERS</span>
                <i>50,000 SHA-256 + lowercase hex rounds</i>
              </div>
            </div>
            <div className="pathAnnotations">
              <span><b>①</b> admission</span><span><b>②</b> batching</span><span><b>③</b> hash kernel</span>
            </div>
          </div>
        </div>
      </section>

      <section className="optScopeSection">
        <div className="optScopeIntro">
          <span className="sectionNumber">WHAT COUNTS AS BETTER?</span>
          <h2>Measure at three<br />different scopes.</h2>
          <p>Each scope answers a different question. A change only belongs in the service when it succeeds at the outermost one.</p>
        </div>
        <div className="scopeDiagram" aria-label="Three nested scopes for measuring an optimization">
          <div className="scopeLevel siegeScope">
            <header><b>03</b><span>FULL MIXED SIEGE</span><small>Does the score rise while every p95 stays safe?</small></header>
            <div className="scopeLevel endpointScope">
              <header><b>02</b><span>ONE ENDPOINT</span><small>Do queueing and response handling preserve the gain?</small></header>
              <div className="scopeLevel kernelScope">
                <header><b>01</b><span>KERNEL BENCHMARK</span><small>Did the isolated hash work become cheaper?</small></header>
                <div className="scopePulse"><i /><i /><i /><span>50K HASHES</span></div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="batchSection">
        <div className="batchIntro">
          <span className="sectionNumber inverse">WHY BATCHING MIGHT HELP</span>
          <h2>One chain is serial.<br />Several chains are independent.</h2>
          <p>Round 2 needs the output of round 1, so a single risk request cannot be split freely across cores. Different requests have no dependency on one another, which gives the CPU unrelated work to interleave.</p>
        </div>
        <div className="batchCompare">
          <article className="serialPanel">
            <div className="diagramHead"><span>ONE REQUEST</span><small>finish A before starting B</small></div>
            <div className="serialChain" aria-label="One serial feedback chain">
              <span>A1</span><i>→</i><span>A2</span><i>→</i><span>A3</span><i>→</i><span>A4</span>
            </div>
            <div className="dependencyNote"><b>DEPENDENCY</b><span>Each cell needs the digest immediately before it.</span></div>
          </article>
          <article className="interleavePanel">
            <div className="diagramHead"><span>FOUR REQUESTS</span><small>advance independent chains together</small></div>
            <div className="interleaveGrid" aria-label="Four independent hash chains interleaved">
              {["A", "B", "C", "D"].map((lane) => (
                <div className="hashLane" key={lane}><b>{lane}</b><span>{lane}1</span><span>{lane}2</span><span>{lane}3</span><span>{lane}4</span></div>
              ))}
            </div>
            <div className="independentNote"><b>INDEPENDENT</b><span>A never needs B’s result. The implementation can interleave their compression work.</span></div>
          </article>
        </div>
        <p className="diagramCaveat">This is a scheduling model, not a literal CPU trace. The useful batch size depends on instruction width, registers, cache, and the cost of waiting for a batch to fill.</p>
      </section>

      <section className="fixedShapeSection">
        <div className="fixedShapeIntro">
          <span className="sectionNumber">WHY A SPECIAL SHA PATH IS POSSIBLE</span>
          <h2>The first round varies.<br />The next 49,999 share a shape.</h2>
          <p>SHA-256 always pads its input. A 64-byte message occupies one full data block, so padding and the length field require a second block. That second block is identical on every repeated round.</p>
        </div>

        <div className="roundFlow firstRoundFlow" aria-label="The first hash round accepts a variable-length seed">
          <div className="roundLabel"><b>ROUND 1</b><span>generic path</span></div>
          <div className="variableSeed"><small>VARIABLE LENGTH</small><strong>seed</strong><i>“0.482…”</i></div>
          <span className="roundArrow">→</span>
          <div className="shaNode">SHA<br />256</div>
          <span className="roundArrow">→</span>
          <div className="hexOutput"><small>64 ASCII BYTES</small><div>{Array.from({ length: 8 }, (_, index) => <i key={index} />)}</div><span>lowercase hex digest</span></div>
        </div>

        <div className="roundFlow repeatedRoundFlow" aria-label="Repeated rounds use a changing 64-byte message block and one fixed padding block">
          <div className="roundLabel"><b>ROUNDS 2–50,000</b><span>fixed-shape path</span></div>
          <div className="shaBlocks">
            <div className="changingBlock"><small>BLOCK 1 · 64 BYTES</small><b>changing hex message</b><span>digest from previous round</span></div>
            <div className="fixedBlock"><small>BLOCK 2 · 64 BYTES</small><b>fixed padding</b><span>80 00 … 00 00 02 00</span></div>
          </div>
          <span className="roundArrow">→</span>
          <div className="compressionNode"><small>COMPRESS</small><b>×2</b></div>
          <span className="roundArrow loopBack">↺</span>
        </div>
        <div className="shapeKey"><span><i className="coral" />changes every round</span><span><i className="acid" />known in advance</span><p>Specialization removes general setup; it does not remove a SHA round or change the digest.</p></div>
      </section>

      <section className="section optRoadmapSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">A PRACTICAL SEQUENCE</span><h2>Let each test answer<br />the next question.</h2></div>
          <p>The sequence starts with evidence, changes scheduling before cryptographic code, and treats a language boundary as a final experiment rather than a starting assumption.</p>
        </div>
        <div className="optRoadmapRail">
          {roadmap.map((step) => (
            <article className="optRoadmapItem" key={step.number}>
              <span className="optRoadmapNumber">{step.number}</span>
              <div className="optRoadmapMarker"><i /></div>
              <div className="optRoadmapCopy"><small>{step.question}</small><h3>{step.title}</h3><p>{step.copy}</p></div>
            </article>
          ))}
        </div>
      </section>

      <section className="referenceSection">
        <div className="referenceIntro">
          <span className="sectionNumber inverse">OPTIMIZATION REFERENCE</span>
          <h2>What changes,<br />and why?</h2>
          <p>These began as hypotheses; each row now carries its measured outcome. The useful signal was never whether an idea sounded fast, but whether the expected mechanism appeared in the measurements—and twice it did not.</p>
        </div>
        <div className="referenceHeader" aria-hidden="true"><span>OPTION</span><span>WHY IT CAN HELP</span><span>WHAT IT COSTS</span><span>WHAT TO MEASURE</span></div>
        <div className="referenceList">
          {options.map((option) => (
            <article className="referenceRow" key={option.number}>
              <div className="referenceIdentity"><span>{option.number}</span><small>{option.area}</small><h3>{option.title}</h3><p>{option.change}</p></div>
              <div className="referenceCell helpsCell"><small>WHY IT CAN HELP</small><p>{option.helps}</p></div>
              <div className="referenceCell costCell"><small>WHAT IT COSTS</small><p>{option.cost}</p></div>
              <div className="referenceCell evidenceCell"><small>WHAT TO MEASURE</small><p>{option.evidence}</p><p className="referenceOutcome"><b>OUTCOME</b> {option.outcome}</p></div>
            </article>
          ))}
        </div>
      </section>

      <section className="section boundarySection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">WHERE LANGUAGE CHOICE MATTERS</span><h2>Keep the server.<br />Swap only the kernel.</h2></div>
          <p>Go is already well suited to the request path. C or Rust can only help where lower-level control changes the measured hash loop enough to outweigh crossing the boundary.</p>
        </div>
        <div className="boundaryDiagram" aria-label="A narrow native kernel boundary inside the Go service">
          <div className="goBoundary">
            <header><span>GO PROCESS</span><small>unchanged control plane</small></header>
            <div className="goParts"><span>HTTP</span><i>→</i><span>QUEUE</span><i>→</i><span>BATCHER</span></div>
          </div>
          <div className="ffiBoundary"><span>BATCH IN</span><b>⇄</b><span>DIGESTS OUT</span></div>
          <div className="nativeBoundary"><header><span>OPTIONAL NATIVE FUNCTION</span><small>small data plane</small></header><div><b>C</b><i>or</i><b>RUST</b><span>fixed-block SHA batch</span></div></div>
        </div>
        <div className="boundaryNotes">
          <article><b>Why keep Go?</b><p>The networking, queueing, cancellation, and JSON paths are already simple and are not where most risk CPU time is spent.</p></article>
          <article><b>Why batch the call?</b><p>One larger call amortizes FFI overhead and gives the native function enough independent inputs to use SIMD or manual scheduling.</p></article>
          <article><b>C or Rust?</b><p>C gives the smallest toolchain and ABI. Rust makes complex internal state safer. Their performance depends more on the kernel than the language label.</p></article>
        </div>
      </section>

      <section className="experimentSection">
        <div className="experimentIntro"><span className="sectionNumber">HOW TO KEEP OR REJECT A CHANGE</span><h2>Four checks.<br />One decision.</h2><p>A candidate must pass all four. A failure tells us which layer erased the apparent improvement.</p></div>
        <div className="experimentFlow">
          {checks.map(([title, copy], index) => (
            <article key={title}><span>{String(index + 1).padStart(2, "0")}</span><div><h3>{title}</h3><p>{copy}</p></div><b aria-label="must pass">✓</b></article>
          ))}
        </div>
        <div className="decisionRule"><span>REFERENCE OUTPUTS MATCH</span><i>+</i><span>REPEATED SCORE IMPROVES</span><i>+</i><span>ALL P95 BARS PASS</span><i>=</i><strong>KEEP</strong></div>
      </section>

      <section className="optimizationFooter">
        <div><span className="sectionNumber">THE MAIN IDEA</span><h2>Understand the path.<br /><em>Change one layer.</em><br />Measure the whole.</h2></div>
        <div className="footerLinks">
          <Link className="backLink" href="/solution"><b>←</b><span>Current solution</span></Link>
          <Link className="backLink primary" href="/"><span>Challenge brief</span><b>↗</b></Link>
        </div>
      </section>
    </main>
  );
}
