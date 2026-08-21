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
  assert.match(html, /https:\/\/timothy-unimelb\.github\.io\/Obsidio\/og\.png/);
  assert.doesNotMatch(html, /chatgpt\.site|codex-preview|SkeletonPreview/);
});

test("exports the solution page", async () => {
  const html = await readFile(new URL("solution/index.html", output), "utf8");

  assert.match(html, /<title>Our Solution — Obsidio<\/title>/i);
  assert.match(html, /Protect the cheap path/);
  assert.match(html, /THE ARCHITECTURE/);
  assert.match(html, /150,000/);
  assert.match(html, /3,245,586/);
  assert.match(html, /0\.00%/);
  assert.match(html, /href="\/Obsidio\/?"/);
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
