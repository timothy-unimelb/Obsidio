import type { Metadata } from "next";
import Link from "next/link";
import { SiteHeader } from "../components/SiteHeader";

export const metadata: Metadata = {
  title: "Our Solution — Obsidio",
  description: "A visual walkthrough of the bounded-concurrency, allocation-free Go solution built for Obsidio.",
};

const results = [
  { metric: "/price p95", value: "50.82", unit: "ms", bar: "25.4%", tone: "mint", limit: "200 ms bar" },
  { metric: "/stats p95", value: "50.85", unit: "ms", bar: "10.2%", tone: "amber", limit: "500 ms bar" },
  { metric: "/risk p95", value: "55.84", unit: "ms", bar: "3.7%", tone: "coral", limit: "1,500 ms bar" },
];

export default function SolutionPage() {
  return (
    <main className="solutionPage">
      <SiteHeader active="solution" />

      <section className="solutionHero">
        <div className="eyebrow light"><span>02</span> Our solution</div>
        <div className="solutionHeroGrid">
          <div>
            <h1>Protect the cheap path.<br /><em>Bound the expensive one.</em></h1>
            <p className="lede lightText">A compact Go server turns uncontrolled CPU contention into two deliberate lanes of heavy work—while cheap traffic walks straight past.</p>
          </div>
          <div className="coreVisual" aria-label="Two CPU cores process bounded risk work">
            <div className="coreRing outer"><span>GOMAXPROCS</span><b>2</b></div>
            <div className="core coreA"><small>CORE</small><b>01</b><i /></div>
            <div className="core coreB"><small>CORE</small><b>02</b><i /></div>
            <div className="orbitLabel l1">PRICE</div><div className="orbitLabel l2">STATS</div><div className="orbitLabel l3">RISK</div>
          </div>
        </div>
      </section>

      <section className="section controlsSection">
        <div className="sectionHead">
          <div><span className="sectionNumber">THE TWO CONTROLS</span><h2>Same number.<br />Different jobs.</h2></div>
          <p><code>riskSlots=2</code> and <code>GOMAXPROCS=2</code> are separate limits. One controls admission to heavy work; the other controls execution across the whole Go process.</p>
        </div>

        <div className="controlDiagram" aria-label="The semaphore and Go scheduler are two separate controls">
          <article className="controlLayer admissionLayer">
            <header><span>01 · OUR CODE</span><code>riskSlots = 2</code></header>
            <div className="admissionBoard">
              <div className="routeLabel mint"><b>/price</b><small>BYPASS</small></div>
              <div className="routeLine"><i /><span>→</span></div>
              <div className="admissionResult directResult">RUNNABLE</div>

              <div className="routeLabel amber"><b>/stats</b><small>BYPASS</small></div>
              <div className="routeLine"><i /><span>→</span></div>
              <div className="admissionResult directResult">RUNNABLE</div>

              <div className="routeLabel coral"><b>/risk</b><small>MUST ENTER</small></div>
              <div className="permitGate"><span>01</span><span>02</span></div>
              <div className="admissionResult riskResult"><b>2 RUNNABLE</b><small>REST PARKED</small></div>
            </div>
            <p>Only risk requests pass through this gate. Once both permits are occupied, later risk handlers sleep instead of joining the CPU competition.</p>
          </article>

          <div className="controlHandoff" aria-hidden="true"><span>THEN</span><b>→</b></div>

          <article className="controlLayer runtimeLayer">
            <header><span>02 · GO RUNTIME</span><code>GOMAXPROCS = 2</code></header>
            <div className="runtimeBoard">
              <div className="runnableTray">
                <small>RUNNABLE GOROUTINES</small>
                <div><span className="mint">PRICE</span><span className="amber">STATS</span><span className="coral">RISK 01</span><span className="coral">RISK 02</span></div>
              </div>
              <div className="schedulerStep"><span>GO SCHEDULER PICKS ANY TWO</span><b>↓</b></div>
              <div className="processorPair">
                <div><small>EXECUTING</small><b>CPU SLOT 1</b></div>
                <div><small>EXECUTING</small><b>CPU SLOT 2</b></div>
              </div>
            </div>
            <p>This limit applies to every endpoint. The two executing goroutines can be risk + risk, risk + price, price + stats—or any other ready pair.</p>
          </article>
        </div>

        <div className="priorityTruth">
          <strong>NO SECRET PRIORITY</strong>
          <p>Go does not know that <code>/price</code> matters most. Cheap requests stay responsive because they bypass the risk gate and compete with at most two active hash loops—not hundreds.</p>
        </div>
      </section>

      <section className="schedulerSection">
        <div className="schedulerIntro">
          <span className="sectionNumber inverse">A SCHEDULING MOMENT</span>
          <h2>Price arrives.<br />Who moves?</h2>
          <p>A request becomes runnable immediately. If both CPU slots are occupied, it waits for the next scheduling opportunity rather than entering the risk queue.</p>
        </div>

        <div className="scheduleExplainer">
          <div className="timelineCard" aria-label="Illustrative timeline of a price request being scheduled between risk requests">
            <div className="timelineScale"><span>NOW</span><span>NEXT OPPORTUNITY</span><span>CONTINUE</span></div>
            <div className="cpuTimeline">
              <b>CPU 01</b>
              <div className="timelineTrack">
                <span className="segment coral long">RISK A</span>
                <span className="segment mint quick">PRICE</span>
                <span className="segment coral rest">RISK C</span>
              </div>
            </div>
            <div className="cpuTimeline">
              <b>CPU 02</b>
              <div className="timelineTrack secondTrack">
                <span className="segment coral longer">RISK B</span>
                <span className="segment amber quick">STATS</span>
                <span className="segment coral remainder">RISK D</span>
              </div>
            </div>
            <div className="arrivalMarker"><i /><span>PRICE BECOMES RUNNABLE</span></div>
            <p>Illustrative sequence—not an exact trace or a priority guarantee.</p>
          </div>

          <div className="switchCard">
            <div className="switchEvent"><span>01</span><p><b>Natural hand-off</b>A handler finishes, blocks on I/O, or waits on synchronization.</p></div>
            <div className="switchEvent"><span>02</span><p><b>Runtime preemption</b>Go can pause CPU-bound work so another runnable goroutine is not starved.</p></div>
            <div className="timingPair">
              <div><strong>3.68<small>ms</small></strong><span>local risk kernel</span></div>
              <div><strong>~10<small>ms</small></strong><span>preemption target</span></div>
            </div>
            <p className="timingCaveat">Usually our risk calculation finishes before forced preemption is needed. The ~10 ms target is a backstop, not a metronome.</p>
          </div>
        </div>
      </section>

      <section className="section permitSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">WHY TWO PERMITS?</span><h2>Match the box.<br />Protect the score.</h2></div>
          <p>Two is the strongest measured default, not a universal mathematical proof. It matches two CPUs and preserves headroom for the fast path on our full siege.</p>
        </div>

        <div className="permitChoices">
          <article><span className="choiceNumber">1</span><small>RISK PERMIT</small><h3>Leaves throughput behind</h3><p>Maximum fast-path protection, but only one core can advance the most valuable request type at a time.</p></article>
          <article className="chosen"><div className="choiceFlag">OUR CHOICE</div><span className="choiceNumber">2</span><small>RISK PERMITS</small><h3>Useful parallelism</h3><p>Both CPUs can hash while Go still gives short turns to price and stats. Local p95 remained far below every bar.</p></article>
          <article><span className="choiceNumber">3+</span><small>RISK PERMITS</small><h3>More competition, not more CPUs</h3><p>Pure CPU work has no I/O to hide. Extra runnable hash loops add contention without adding execution capacity.</p></article>
        </div>

        <div className="scoreMotive">
          <div><span className="sectionNumber">EXPECTED POINTS PER RANDOM REQUEST</span><p>Traffic share × endpoint weight</p></div>
          <div className="motiveBars">
            <div><code>/price</code><i className="mint" style={{ width: "60%" }} /><strong>0.60</strong></div>
            <div><code>/stats</code><i className="amber" style={{ width: "90%" }} /><strong>0.90</strong></div>
            <div><code>/risk</code><i className="coral" style={{ width: "100%" }} /><strong>1.00</strong></div>
          </div>
        </div>
      </section>

      <section className="bufferSection">
        <div className="bufferIntro">
          <span className="sectionNumber inverse">THE HOT LOOP</span>
          <h2>Same 50,000 hashes.<br />Almost none of the garbage.</h2>
          <p>The starter converts bytes → string → bytes on every round. We keep the 64 hexadecimal characters in one fixed buffer and feed it straight back into SHA-256.</p>
        </div>
        <div className="bufferCompare">
          <div className="beforePanel">
            <div className="compareTop"><span>BEFORE</span><b>starter strings</b></div>
            <div className="garbageField" aria-hidden="true">
              {Array.from({ length: 18 }, (_, index) => <i key={index}>64B</i>)}
            </div>
            <div className="compareMetric"><strong>150,000</strong><span>allocations / request</span></div>
            <div className="compareMetric"><strong>9.6 MB</strong><span>temporary memory</span></div>
          </div>
          <div className="afterPanel">
            <div className="compareTop"><span>AFTER</span><b>fixed buffer</b></div>
            <div className="hashLoop">
              <div className="hashBox">SHA<br />256</div><span>→</span><div className="hexBuffer"><i /><i /><i /><i /><i /><i /><i /><i /></div><b className="loopArrow">↺</b>
            </div>
            <div className="bufferLabel"><span>64 BYTES</span><em>reused 49,999 times</em></div>
            <div className="compareMetric after"><strong>0</strong><span>loop allocations</span></div>
            <div className="speedBadge"><span>5.00 ms</span><b>→</b><strong>3.68 ms</strong></div>
          </div>
        </div>
      </section>

      <section className="section decisionsSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber">THE CONTROL PLANE</span><h2>Four deliberate<br />constraints.</h2></div>
          <p>Resilience comes from limiting choices at runtime. Every knob mirrors the grading box.</p>
        </div>
        <div className="decisionGrid">
          <article><span className="decisionIcon">2</span><h3>Two schedulers</h3><p><code>GOMAXPROCS=2</code> prevents the host core count from creating runnable work the quota cannot execute.</p></article>
          <article><span className="decisionIcon">Ⅱ</span><h3>Two risk permits</h3><p>The semaphore turns excess heavy requests into sleeping waiters, not competing hash loops.</p></article>
          <article><span className="decisionIcon">↯</span><h3>Direct fast path</h3><p>Price and stats handlers never touch the risk queue. The lightweight work remains immediately schedulable.</p></article>
          <article><span className="decisionIcon">□</span><h3>Tiny artifact</h3><p>A static binary in a <code>scratch</code> image: no runtime, package manager, shell, or incidental memory cost.</p></article>
        </div>
      </section>

      <section className="resultsSection">
        <div className="resultsIntro"><span className="sectionNumber inverse">THE FULL SIEGE</span><h2>Correct work,<br />at speed.</h2><p>Full 4m30s published k6 run · 200 VUs · 2 CPU / 2 GB local grader-shaped result.</p></div>
        <div className="heroScore"><small>WORK SCORE</small><strong>2,790,902</strong><span>10,336.65 weighted points / sec</span></div>
        <div className="resultBars">
          {results.map((result) => (
            <div className="resultRow" key={result.metric}>
              <code>{result.metric}</code><strong>{result.value}<small>{result.unit}</small></strong>
              <div className="resultTrack"><i className={result.tone} style={{ width: result.bar }} /><span>{result.limit}</span></div>
            </div>
          ))}
        </div>
        <div className="proofStrip"><span><strong>1,115,212</strong> successful requests</span><span><strong>0.00%</strong> HTTP errors</span><span><strong>2.2 MB</strong> final image</span></div>
      </section>

      <section className="solutionFooter">
        <div><span className="sectionNumber">THE PRINCIPLE</span><h2>Do all the work.<br /><em>Control when it runs.</em></h2></div>
        <Link className="backLink" href="/performance"><span>See the performance record</span><b>→</b></Link>
      </section>
    </main>
  );
}
