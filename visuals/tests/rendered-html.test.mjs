import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

const output = new URL("../out/", import.meta.url);

test("exports the challenge page for the GitHub Pages base path", async () => {
  const html = await readFile(new URL("index.html", output), "utf8");

  assert.match(html, /<title>Obsidio — Engineering Under Siege<\/title>/i);
  assert.match(html, /Keep the gate fast/);
  assert.match(html, /Three jobs/);
  assert.match(html, /THE SIEGE/);
  assert.match(html, /THE SCORE/);
  assert.match(html, /href="\/Obsidio\/solution\/?"/);
  assert.match(html, /href="\/Obsidio\/optimizations\/?"/);
  assert.match(html, /https:\/\/timothy-unimelb\.github\.io\/Obsidio\/og\.png/);
  assert.doesNotMatch(html, /chatgpt\.site|codex-preview|SkeletonPreview/);
});

test("exports the solution page", async () => {
  const html = await readFile(new URL("solution/index.html", output), "utf8");

  assert.match(html, /<title>Our Solution — Obsidio<\/title>/i);
  assert.match(html, /Protect the cheap path/);
  assert.match(html, /THE TWO CONTROLS/);
  assert.match(html, /workers = 2 · FIFO = 32/);
  assert.match(html, /GOMAXPROCS = 2/);
  assert.match(html, /EXPLICIT ADMISSION POLICY/);
  assert.match(html, /WHY TWO WORKERS/);
  assert.match(html, /preemption target/);
  assert.match(html, /150,000/);
  assert.match(html, /2,874,253/);
  assert.match(html, /0\.00%/);
  assert.match(html, /href="\/Obsidio\/?"/);
  assert.match(html, /href="\/Obsidio\/optimizations\/?"/);
  assert.match(html, /href="\/Obsidio\/performance\/?"/);
});

test("exports the optimization roadmap", async () => {
  const html = await readFile(new URL("optimizations/index.html", output), "utf8");

  assert.match(html, /<title>Optimization Guide — Obsidio<\/title>/i);
  assert.match(html, /A map of what/);
  assert.match(html, /CURRENT SYSTEM/);
  assert.match(html, /Measure at three/);
  assert.match(html, /ONE REQUEST/);
  assert.match(html, /FOUR REQUESTS/);
  assert.match(html, /ROUNDS 2–50,000/);
  assert.match(html, /A PRACTICAL SEQUENCE/);
  assert.match(html, /OPTIMIZATION REFERENCE/);
  assert.match(html, /Permanent risk workers/);
  assert.match(html, /Batch independent requests/);
  assert.match(html, /Specialize the 64-byte rounds/);
  assert.match(html, /Go profile-guided optimization/);
  assert.match(html, /A C or Rust hash kernel/);
  assert.match(html, /BATCH IN/);
  assert.match(html, /Four checks/);
  assert.match(html, /href="\/Obsidio\/solution\/?"/);
  assert.match(html, /href="\/Obsidio\/performance\/?"/);
});

test("exports the performance record", async () => {
  const html = await readFile(new URL("performance/index.html", output), "utf8");

  assert.match(html, /<title>Performance Record — Obsidio<\/title>/i);
  assert.match(html, /Performance,/);
  assert.match(html, /LOCAL GRADER-SHAPED RUNS/);
  assert.match(html, /The protocol matches/);
  assert.match(html, /1,425,795/);
  assert.match(html, /2,874,253/);
  assert.match(html, /\+101\.6%/);
  assert.match(html, /Permanent workers \+ queue/);
  assert.match(html, /Measured · kept · median of 3 runs/);
  assert.match(html, /INSIDE A PEAK \/RISK REQUEST/);
  assert.match(html, /320\.70 ms/);
  assert.match(html, /BASELINE → CURRENT/);
  assert.match(html, /GRADING SCRIPT SHA-256/);
  assert.match(html, /href="\/Obsidio\/optimizations\/?"/);
});

test("exports branded assets without hosting-specific files", async () => {
  const [packageJson, socialCard, favicon] = await Promise.all([
    readFile(new URL("../package.json", import.meta.url), "utf8"),
    access(new URL("og.png", output)),
    readFile(new URL("favicon.svg", output), "utf8"),
  ]);

  assert.doesNotMatch(packageJson, /vinext|wrangler|@openai\/sites-vite-plugin/);
  assert.match(favicon, /#D7FF43/i);
  assert.equal(socialCard, undefined);
  await assert.rejects(access(new URL("../.openai/hosting.json", import.meta.url)));
});
