import type { Metadata } from "next";
import Link from "next/link";
import { SiteHeader } from "../components/SiteHeader";

export const metadata: Metadata = {
  title: "Optimization Roadmap — Obsidio",
  description: "A visual, evidence-led roadmap for taking the Obsidio Go solution beyond its current baseline without sacrificing correctness.",
};

const roadmap = [
  {
    number: "01",
    title: "Lock the baseline",
    label: "MEASURE",
    copy: "Freeze correctness vectors, capture CPU and allocation profiles, then record repeated full-siege results.",
  },
  {
    number: "02",
    title: "Own the risk queue",
    label: "ARCHITECTURE",
    copy: "Replace per-request hash goroutines with two permanent workers and one small bounded FIFO.",
  },
  {
    number: "03",
    title: "Batch the hot work",
    label: "BIG EXPERIMENT",
    copy: "Try batches of 2, 4, then 8 independent risk jobs, with a strict cap on how long a batch may wait to fill.",
  },
  {
    number: "04",
    title: "Specialize the kernel",
    label: "DEEP OPTIMIZATION",
    copy: "Exploit the fixed 64-byte ASCII-hex input used by rounds 2–50,000 while preserving every SHA-256 round.",
  },
  {
    number: "05",
    title: "Take the cheap wins",
    label: "LOW RISK",
    copy: "Apply Go PGO from a representative profile, then tune GC, queue depth, and worker count one variable at a time.",
  },
  {
    number: "06",
    title: "Trim the perimeter",
    label: "ONLY IF VISIBLE",
    copy: "Optimize HTTP parsing, JSON assembly, symbol lookup, and response buffers only after profiles show they matter.",
  },
  {
    number: "07",
    title: "Cross the language boundary",
    label: "LAST RESORT",
    copy: "Try a narrow C or Rust hash kernel behind the Go server only if the pure-Go kernel still dominates the siege.",
  },
];

const options = [
  {
    order: "01",
    title: "Permanent risk workers",
    tag: "DO FIRST",
    tone: "acid",
    upside: "High",
    effort: "Medium",
    risk: "Low",
    summary: "Two long-lived workers pull heavy jobs from a bounded queue; price and stats still bypass it.",
    pros: ["Creates the foundation for batching", "Predictable CPU competition", "Fewer transient goroutines"],
    cons: ["Adds queue and hand-off overhead", "Queue depth needs siege testing"],
  },
  {
    order: "02",
    title: "Multi-request batching",
    tag: "BEST UPSIDE",
    tone: "coral",
    upside: "High",
    effort: "High",
    risk: "Medium",
    summary: "Advance several independent SHA chains together so the CPU can overlap work and use its execution units more fully.",
    pros: ["Attacks the dominant workload", "Better instruction-level parallelism", "Can improve cache behaviour"],
    cons: ["The first job may wait for a batch", "Best size depends on grading CPU", "More complex kernel and tests"],
  },
  {
    order: "03",
    title: "Fixed-block SHA path",
    tag: "KERNEL",
    tone: "coral",
    upside: "Med–high",
    effort: "High",
    risk: "High",
    summary: "After the first round, every input is exactly 64 lowercase hex bytes. Prebuild the invariant padding path and specialize that case.",
    pros: ["49,999 rounds share one shape", "Removes general-purpose setup", "Keeps the response identical"],
    cons: ["Cryptographic code is easy to get subtly wrong", "May duplicate work already optimized in assembly"],
  },
  {
    order: "04",
    title: "Go PGO",
    tag: "LOW FRICTION",
    tone: "mint",
    upside: "Low–med",
    effort: "Low",
    risk: "Low",
    summary: "Build with a CPU profile captured from the representative mixed workload, not a synthetic one-endpoint run.",
    pros: ["Compiler-guided inlining and layout", "Small code change", "Easy to revert"],
    cons: ["Usually a few-percent tool, not a breakthrough", "A stale profile can mislead the build"],
  },
  {
    order: "05",
    title: "Runtime tuning",
    tag: "MEASURE",
    tone: "amber",
    upside: "Low–med",
    effort: "Low",
    risk: "Medium",
    summary: "Sweep worker count, queue depth, batch size, and GOGC under the real 2 CPU / 2 GB cap.",
    pros: ["Cheap experiments", "Can cut scheduler or GC tax", "Makes resource use explicit"],
    cons: ["Knobs interact", "Higher GOGC trades CPU for memory", "Local laptop results may not transfer"],
  },
  {
    order: "06",
    title: "HTTP + serialization",
    tag: "SECONDARY",
    tone: "mint",
    upside: "Low",
    effort: "Medium",
    risk: "Medium",
    summary: "Benchmark a leaner HTTP path, prebuilt response fragments, direct query parsing, and tighter buffer reuse.",
    pros: ["Benefits every request", "Can reduce allocations", "Fast path gets slightly cheaper"],
    cons: ["Hashing still dominates total CPU", "Custom protocol code expands the bug surface"],
  },
  {
    order: "07",
    title: "Priority-aware admission",
    tag: "SAFETY",
    tone: "amber",
    upside: "Indirect",
    effort: "Medium",
    risk: "Medium",
    summary: "Reduce risk pressure when cheap-request latency or runnable work climbs, then restore it when the gate clears.",
    pros: ["Protects the qualifying p95 bars", "Adapts to bursty traffic", "Keeps all endpoints correct"],
    cons: ["Can reduce risk throughput", "Feedback can oscillate", "Needs a reliable pressure signal"],
  },
  {
    order: "08",
    title: "Native C / Rust kernel",
    tag: "PROVE IT",
    tone: "ink",
    upside: "Uncertain",
    effort: "Highest",
    risk: "High",
    summary: "Keep Go for networking and scheduling; replace only the measured hash kernel through a very small native boundary.",
    pros: ["Maximum control over SIMD and batching", "Rust can isolate unsafe code", "C has the smallest ABI surface"],
    cons: ["CGO and cross-build complexity", "Call overhead punishes tiny batches", "Harder debugging and portability"],
  },
];

const keepChecks = [
  ["01", "Correct", "Cross-check every digest against the reference implementation, including edge-case seeds."],
  ["02", "Faster", "Beat the baseline across repeated full k6 runs—not just a microbenchmark."],
  ["03", "Qualified", "Keep price, stats, and risk p95 beneath their bars with a 0% application error target."],
  ["04", "Portable", "Build reproducibly and retest on the grader’s CPU architecture under 2 CPU / 2 GB."],
];

export default function OptimizationsPage() {
  return (
    <main className="optimizationPage">
      <SiteHeader active="optimizations" />

      <section className="optimizationHero">
        <div className="eyebrow light"><span>03</span> The next gains</div>
        <div className="optimizationHeroGrid">
          <div>
            <h1>Optimize in layers.<br /><em>Rewrite last.</em></h1>
            <p className="lede lightText">The current Go build already protects the fast path and removes loop allocations. The next score gains come from feeding the two cores better—then specializing only what the profile proves is hot.</p>
            <div className="optimizationCaps">
              <span>✓ EXACT OUTPUTS</span><span>2 CPU AWARE</span><span>FULL-SIEGE TESTED</span>
            </div>
          </div>

          <div className="gainStack" aria-label="Optimization layers build from the current baseline toward an optional native kernel">
            <div className="gainAxis"><span>LOW COMPLEXITY</span><span>HIGH COMPLEXITY</span></div>
            <div className="gainStep stepGo"><span>CURRENT</span><b>GO BASELINE</b><i>bounded + allocation-free</i></div>
            <div className="gainStep stepQueue"><span>01</span><b>WORKERS + QUEUE</b><i>control the feed</i></div>
            <div className="gainStep stepBatch"><span>02</span><b>BATCH + SPECIALIZE</b><i>accelerate the kernel</i></div>
            <div className="gainStep stepNative"><span>03</span><b>NATIVE?</b><i>only if measured</i></div>
          </div>
        </div>
      </section>

      <section className="measureRule">
        <span className="sectionNumber">THE RULE</span>
        <div className="measureRuleGrid">
          <h2>Optimize the <em>siege</em>,<br />not the stopwatch.</h2>
          <div className="evidenceLoop" aria-label="Correctness and performance validation loop">
            <span><b>1</b> CORRECT</span><i>→</i><span><b>2</b> PROFILE</span><i>→</i><span><b>3</b> SIEGE</span><i>→</i><span><b>4</b> KEEP?</span>
          </div>
        </div>
        <p>A faster isolated hash is a loss if its queueing, memory, or language boundary slows the mixed workload. Every change must improve weighted work while all three endpoints remain correct.</p>
      </section>

      <section className="section roadmapSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">RECOMMENDED ORDER</span><h2>The order<br />creates the gain.</h2></div>
          <p>Each phase either establishes evidence or unlocks the next experiment. Stop as soon as the profile says the remaining complexity is not worth it.</p>
        </div>

        <div className="roadmapRail">
          {roadmap.map((step) => (
            <article className="roadmapItem" key={step.number}>
              <span className="roadmapNumber">{step.number}</span>
              <div className="roadmapMarker"><i /></div>
              <div className="roadmapCopy"><small>{step.label}</small><h3>{step.title}</h3><p>{step.copy}</p></div>
            </article>
          ))}
        </div>
      </section>

      <section className="optionSection">
        <div className="optionIntro">
          <span className="sectionNumber inverse">THE OPTIONS</span>
          <h2>Eight levers.<br /><em>Unequal tradeoffs.</em></h2>
          <p>“Upside” is a hypothesis until the grader-shaped benchmark confirms it. The labels compare opportunities with each other; they are not promised speedups.</p>
        </div>

        <div className="optionGrid">
          {options.map((option) => (
            <article className={`optionCard ${option.tone}`} key={option.order}>
              <div className="optionTop"><span>{option.order}</span><b>{option.tag}</b></div>
              <h3>{option.title}</h3>
              <p className="optionSummary">{option.summary}</p>
              <div className="optionMeters">
                <span><small>UPSIDE</small><b>{option.upside}</b></span>
                <span><small>EFFORT</small><b>{option.effort}</b></span>
                <span><small>RISK</small><b>{option.risk}</b></span>
              </div>
              <div className="tradeoffs">
                <div><small>PROS</small>{option.pros.map((pro) => <p key={pro}><i>+</i>{pro}</p>)}</div>
                <div><small>CONS</small>{option.cons.map((con) => <p key={con}><i>−</i>{con}</p>)}</div>
              </div>
            </article>
          ))}
        </div>
      </section>

      <section className="section kernelDecisionSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">THE LANGUAGE DECISION</span><h2>Keep Go around<br />the hot core.</h2></div>
          <p>A whole-server rewrite spends risk on code that is not the bottleneck. If native code wins, make it a scalpel: one batch-in, batch-out kernel behind the existing Go queue.</p>
        </div>

        <div className="kernelDecision" aria-label="Decision flow for adopting a native C or Rust kernel">
          <div className="decisionNode goNode"><small>CONTROL PLANE</small><strong>GO</strong><span>HTTP · queue · scheduling · JSON</span></div>
          <div className="decisionArrow"><span>PROFILE SAYS<br />SHA DOMINATES</span><b>→</b></div>
          <div className="decisionNode nativeNode"><small>OPTIONAL DATA PLANE</small><strong>C / RUST</strong><span>batched fixed-block SHA only</span></div>
          <div className="decisionArrow"><span>FULL SIEGE<br />+ VECTORS</span><b>→</b></div>
          <div className="decisionNode verdictNode"><small>VERDICT</small><strong>KEEP / DROP</strong><span>evidence, not language preference</span></div>
        </div>

        <div className="languageNotes">
          <article><span>C</span><p><b>Smallest boundary.</b> Easy to expose a narrow batch function and tune intrinsics. Memory safety and manual portability remain ours to manage.</p></article>
          <article><span>R</span><p><b>Safer internals.</b> Rust makes a complex batched implementation easier to contain, at the cost of a heavier toolchain and the same FFI boundary.</p></article>
          <article className="recommendation"><span>G</span><p><b>Our recommendation.</b> Start and probably finish in Go. Move only the kernel if a native prototype wins by enough to pay for its build and correctness cost.</p></article>
        </div>
      </section>

      <section className="proofSection">
        <div className="proofIntro"><span className="sectionNumber inverse">THE KEEP / REJECT GATE</span><h2>Every win<br />must survive four tests.</h2></div>
        <div className="proofGrid">
          {keepChecks.map(([number, title, copy]) => (
            <article key={number}><span>{number}</span><h3>{title}</h3><p>{copy}</p></article>
          ))}
        </div>
        <div className="correctnessLine"><strong>NOT ON THE ROADMAP</strong><span>Skipped hashes, cached seeds, approximate answers, or intentional failures. More score only counts when it represents the specified work.</span></div>
      </section>

      <section className="optimizationFooter">
        <div><span className="sectionNumber">THE BET</span><h2>Queue first.<br /><em>Batch second.</em><br />Rewrite last.</h2></div>
        <div className="footerLinks">
          <Link className="backLink" href="/solution"><b>←</b><span>Current solution</span></Link>
          <Link className="backLink primary" href="/"><span>Challenge brief</span><b>↗</b></Link>
        </div>
      </section>
    </main>
  );
}
