import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

async function render(pathname = "/") {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}-${pathname}`);
  const { default: worker } = await import(workerUrl.href);

  return worker.fetch(
    new Request(`http://localhost${pathname}`, {
      headers: { accept: "text/html" },
    }),
    { ASSETS: { fetch: async () => new Response("Not found", { status: 404 }) } },
    { waitUntil() {}, passThroughOnException() {} },
  );
}

test("renders the visual challenge page and social metadata", async () => {
  const response = await render("/");
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /<title>Obsidio — Engineering Under Siege<\/title>/i);
  assert.match(html, /Keep the gate fast/);
  assert.match(html, /Three jobs/);
  assert.match(html, /THE SIEGE/);
  assert.match(html, /THE SCORE/);
  assert.match(html, /og:image/);
  assert.match(html, /og\.png/);
  assert.match(html, /https:\/\/obsidio-engineering-under-siege\.timothymathews\.chatgpt\.site\/og\.png/);
  assert.doesNotMatch(html, /codex-preview|SkeletonPreview|react-loading-skeleton/);
});

test("renders the visual solution page", async () => {
  const response = await render("/solution");
  assert.equal(response.status, 200);

  const html = await response.text();
  assert.match(html, /<title>Our Solution — Obsidio<\/title>/i);
  assert.match(html, /Protect the cheap path/);
  assert.match(html, /THE ARCHITECTURE/);
  assert.match(html, /150,000/);
  assert.match(html, /3,245,586/);
  assert.match(html, /0\.00%/);
});

test("removes starter artifacts and keeps branded assets", async () => {
  const [packageJson, socialCard, favicon] = await Promise.all([
    readFile(new URL("../package.json", import.meta.url), "utf8"),
    access(new URL("../public/og.png", import.meta.url)),
    readFile(new URL("../public/favicon.svg", import.meta.url), "utf8"),
  ]);

  assert.doesNotMatch(packageJson, /react-loading-skeleton/);
  assert.match(favicon, /#D7FF43/i);
  assert.equal(socialCard, undefined);
  await assert.rejects(access(new URL("../app/_sites-preview", import.meta.url)));
});
