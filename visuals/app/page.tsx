import Link from "next/link";
import { SiteHeader } from "./components/SiteHeader";

const lanes = [
  { name: "/price", share: "60%", weight: "×1", tone: "mint" },
  { name: "/stats", share: "30%", weight: "×3", tone: "amber" },
  { name: "/risk", share: "10%", weight: "×10", tone: "coral" },
];

const endpoints = [
  {
    number: "01",
    name: "/price",
    tone: "mint",
    kind: "LOOKUP",
    share: "60%",
    weight: "1 point",
    work: "Find one price in memory",
    visual: <><i /><i className="lit" /><i /><i /></>,
  },
  {
    number: "02",
    name: "/stats",
    tone: "amber",
    kind: "AGGREGATE",
    share: "30%",
    weight: "3 points",
    work: "Scan 500 values, twice",
    visual: <><b /><b /><b /><b /><b /><b /><b /><b /></>,
  },
  {
    number: "03",
    name: "/risk",
    tone: "coral",
    kind: "COMPUTE",
    share: "10%",
    weight: "10 points",
    work: "SHA-256 → hex, 50,000×",
    visual: <span>50K</span>,
  },
];

const thresholds = [
  { name: "/price", value: "200", unit: "ms", width: "22%", tone: "mint" },
  { name: "/stats", value: "500", unit: "ms", width: "43%", tone: "amber" },
  { name: "/risk", value: "1500", unit: "ms", width: "100%", tone: "coral" },
];

export default function ChallengePage() {
  return (
    <main>
      <SiteHeader active="challenge" />

      <section className="hero challengeHero">
        <div className="eyebrow"><span>01</span> The challenge</div>
        <div className="heroGrid">
          <div className="heroCopy">
            <h1>Keep the gate fast<br />under <em>siege.</em></h1>
            <p className="lede">
              Build one correct analytics API. Then hold the line while 200
              virtual users mix tiny lookups with relentless CPU-heavy work.
            </p>
            <div className="caps">
              <span><b>2</b> CPU cores</span>
              <span><b>2</b> GB memory</span>
              <span><b>8080</b> one port</span>
            </div>
          </div>

          <div className="siegeCard" aria-label="Traffic mix entering the constrained API">
            <div className="trafficSource">
              <div className="pulse p1" /><div className="pulse p2" /><div className="pulse p3" />
              <span>200 VUs</span>
            </div>
            <div className="flowLines" aria-hidden="true"><i /><i /><i /></div>
            <div className="gate">
              <span className="gateLabel">THE GATE</span>
              <strong>:8080</strong>
              <small>2 CPU · 2 GB</small>
            </div>
            <div className="laneStack">
              {lanes.map((lane) => (
                <div className={`lane ${lane.tone}`} key={lane.name}>
                  <span className="laneShare">{lane.share}</span>
                  <strong>{lane.name}</strong>
                  <span className="laneWeight">{lane.weight}</span>
                </div>
              ))}
            </div>
            <p className="cardCaption">One entrance. Three radically different costs.</p>
          </div>
        </div>
      </section>

      <section className="firstLesson">
        <span className="sectionNumber">THE CORE TENSION</span>
        <p>The cheap request takes almost no work.</p>
        <h2>If <code>/price</code> is slow, it is waiting behind something expensive.</h2>
        <div className="queueVisual" aria-label="A price request waiting behind risk requests">
          <div className="queueLabel">CPU QUEUE</div>
          <div className="queueTrack">
            <span className="queueBlock riskBlock">RISK</span>
            <span className="queueBlock riskBlock">RISK</span>
            <span className="queueBlock statsBlock">STATS</span>
            <span className="queueBlock priceBlock">PRICE</span>
            <span className="queueArrow">→</span>
            <span className="cpuDoor">CPU</span>
          </div>
          <div className="waitNote"><b>fast work</b><span>slow wait</span></div>
        </div>
      </section>

      <section className="section endpointsSection">
        <div className="sectionHead">
          <div><span className="sectionNumber">THE WORKLOAD</span><h2>Three jobs.<br />One shared pipe.</h2></div>
          <p>Every request must return the right JSON. The work cannot be skipped; the only freedom is how deliberately the box handles it.</p>
        </div>
        <div className="endpointGrid">
          {endpoints.map((endpoint) => (
            <article className={`endpointCard ${endpoint.tone}`} key={endpoint.name}>
              <div className="cardTop"><span>{endpoint.number}</span><small>{endpoint.kind}</small></div>
              <div className={`endpointGlyph ${endpoint.tone}`}>{endpoint.visual}</div>
              <h3>{endpoint.name}</h3>
              <p>{endpoint.work}</p>
              <div className="endpointMeta"><span><small>TRAFFIC</small>{endpoint.share}</span><span><small>SCORE</small>{endpoint.weight}</span></div>
            </article>
          ))}
        </div>
      </section>

      <section className="section rampSection">
        <div className="sectionHead compact">
          <div><span className="sectionNumber inverse">THE SIEGE</span><h2>Pressure climbs.<br />Then it holds.</h2></div>
          <p>k6 keeps every virtual user looping. Faster responses immediately become more requests—and more score.</p>
        </div>
        <div className="rampChart" aria-label="Load ramps from zero to 200 virtual users over four minutes and thirty seconds">
          <div className="axisLabels"><span>200 VUs</span><span>100</span><span>50</span><span>0</span></div>
          <div className="rampPlot">
            <div className="gridLine g1" /><div className="gridLine g2" /><div className="gridLine g3" />
            <div className="rampArea">
              <div className="stage stage1"><span>WARM UP</span></div>
              <div className="stage stage2"><span>CLIMB</span></div>
              <div className="stage stage3"><span>PEAK</span></div>
              <div className="stage stage4"><span>COOL</span></div>
            </div>
            <div className="timeLabels"><span>0:00</span><span>1:00</span><span>3:00</span><span>4:00</span><span>4:30</span></div>
          </div>
        </div>
      </section>

      <section className="section scoringSection">
        <div className="scorePanel">
          <span className="sectionNumber">THE SCORE</span>
          <div className="scoreEquation" aria-label="Price requests times one plus stats requests times three plus risk requests times ten">
            <span><b>price</b><i>×1</i></span><em>+</em>
            <span><b>stats</b><i>×3</i></span><em>+</em>
            <span><b>risk</b><i>×10</i></span>
          </div>
          <p>Only successful work earns points. With the published mix, each tier contributes a surprisingly similar share of the opportunity.</p>
          <div className="balanceBar">
            <span className="mint">0.60</span><span className="amber">0.90</span><span className="coral">1.00</span>
          </div>
          <div className="balanceLegend"><span>60% × 1</span><span>30% × 3</span><span>10% × 10</span></div>
        </div>
        <div className="thresholdPanel">
          <span className="sectionNumber">QUALIFY FIRST</span>
          <h3>Stay beneath every p95 bar.</h3>
          <div className="thresholdList">
            {thresholds.map((item) => (
              <div className="threshold" key={item.name}>
                <div><code>{item.name}</code><strong>{item.value}<small>{item.unit}</small></strong></div>
                <div className="thresholdTrack"><i className={item.tone} style={{ width: item.width }} /></div>
              </div>
            ))}
          </div>
          <p className="errorLimit"><span>ERROR RATE</span><b>&lt; 1%</b></p>
        </div>
      </section>

      <section className="challengeCta">
        <div>
          <span className="sectionNumber inverse">THE QUESTION</span>
          <h2>How do you spend both cores<br /><em>without sacrificing the gate?</em></h2>
        </div>
        <Link className="ctaButton" href="/solution"><span>See our answer</span><b>→</b></Link>
      </section>
    </main>
  );
}
