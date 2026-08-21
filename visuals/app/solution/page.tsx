import type { Metadata } from "next";
import Link from "next/link";
import { SiteHeader } from "../components/SiteHeader";

export const metadata: Metadata = {
  title: "Our Solution — Obsidio",
  description: "A visual walkthrough of the bounded-concurrency, allocation-free Go solution built for Obsidio.",
};

const results = [
  { metric: "/price p95", value: "41.90", unit: "ms", bar: "21%", tone: "mint", limit: "200 ms bar" },
  { metric: "/stats p95", value: "41.86", unit: "ms", bar: "8%", tone: "amber", limit: "500 ms bar" },
  { metric: "/risk p95", value: "62.24", unit: "ms", bar: "4%", tone: "coral", limit: "1,500 ms bar" },
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

      <section className="section architectureSection">
        <div className="sectionHead">
          <div><span className="sectionNumber">THE ARCHITECTURE</span><h2>Route first.<br />Queue only the cost.</h2></div>
          <p>The HTTP front door stays concurrent. Only risk work crosses the semaphore, so queue depth never becomes active CPU contention.</p>
        </div>

        <div className="architectureMap" aria-label="Request architecture diagram">
          <div className="incomingNode"><span>IN</span><b>HTTP</b><small>:8080</small></div>
          <div className="mapConnector connectorIn"><i /><i /><i /></div>
          <div className="routerNode"><small>ROUTE BY</small><b>PATH</b></div>
          <div className="branchLines" aria-hidden="true"><i /><i /><i /></div>
          <div className="pathNodes">
            <div className="pathNode mint"><span>/price</span><b>direct</b><small>pre-serialized response</small><i>→</i></div>
            <div className="pathNode amber"><span>/stats</span><b>direct</b><small>500 values · two passes</small><i>→</i></div>
            <div className="pathNode coral riskPath"><span>/risk</span><b>2-slot gate</b><small>wait without consuming CPU</small><i>→</i></div>
          </div>
          <div className="semaphore" aria-label="Two risk permits">
            <span className="permit used">01</span><span className="permit used">02</span><span className="permit blocked">WAIT</span>
          </div>
          <div className="cpuPool"><div><small>CPU</small><b>1</b></div><div><small>CPU</small><b>2</b></div></div>
          <div className="mapCaption"><span>Cheap work bypasses the queue</span><span>Heavy work has exactly two active permits</span></div>
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
        <div className="resultsIntro"><span className="sectionNumber inverse">THE FULL SIEGE</span><h2>Correct work,<br />at speed.</h2><p>Full 4m30s published k6 run · 200 VUs · local Apple M4 development result.</p></div>
        <div className="heroScore"><small>WORK SCORE</small><strong>3,245,586</strong><span>12,020 weighted points / sec</span></div>
        <div className="resultBars">
          {results.map((result) => (
            <div className="resultRow" key={result.metric}>
              <code>{result.metric}</code><strong>{result.value}<small>{result.unit}</small></strong>
              <div className="resultTrack"><i className={result.tone} style={{ width: result.bar }} /><span>{result.limit}</span></div>
            </div>
          ))}
        </div>
        <div className="proofStrip"><span><strong>1,298,614</strong> successful requests</span><span><strong>0.00%</strong> HTTP errors</span><span><strong>2.2 MB</strong> final image</span></div>
      </section>

      <section className="solutionFooter">
        <div><span className="sectionNumber">THE PRINCIPLE</span><h2>Do all the work.<br /><em>Control when it runs.</em></h2></div>
        <Link className="backLink" href="/"><b>←</b><span>Revisit the challenge</span></Link>
      </section>
    </main>
  );
}
