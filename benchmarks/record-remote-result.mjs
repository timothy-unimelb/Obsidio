import { createHash } from "node:crypto";
import { appendFile, readFile } from "node:fs/promises";
import process from "node:process";

const [stage, profile, runId, summaryPath, sourceSha, imageId, exitCodeText, scriptPath] = process.argv.slice(2);
if (!stage || !profile || !runId || !summaryPath || !sourceSha || !imageId || exitCodeText === undefined || !scriptPath) {
  console.error("usage: node benchmarks/record-remote-result.mjs <stage> <profile> <run-id> <summary> <source-sha> <image-id> <k6-exit-code> <script>");
  process.exit(64);
}

const summary = JSON.parse(await readFile(summaryPath, "utf8"));
const script = await readFile(scriptPath);
const metrics = summary.metrics ?? {};
const metric = (name, field) => metrics[name]?.values?.[field] ?? metrics[name]?.[field];
const errorRate = metrics.http_req_failed?.values?.rate
  ?? metrics.http_req_failed?.rate
  ?? metrics.http_req_failed?.value;

const requiredEnvironment = [
  "BENCH_COMPARISON_SET",
  "BENCH_ENVIRONMENT_ID",
  "BENCH_TARGET_INSTANCE_ID",
  "BENCH_LOAD_INSTANCE_ID",
  "BENCH_K6_IMAGE",
  "BENCH_K6_VERSION",
];
for (const name of requiredEnvironment) {
  if (!process.env[name]) {
    throw new Error(`missing required environment metadata: ${name}`);
  }
}

const entry = {
  recorded_at: new Date().toISOString(),
  comparison_set: process.env.BENCH_COMPARISON_SET,
  environment_id: process.env.BENCH_ENVIRONMENT_ID,
  target_relationship: "separate-private-network",
  stage,
  profile,
  run_id: runId,
  verdict: "pending",
  git_sha: sourceSha,
  source_dirty: false,
  meaningful_changes: [],
  image_name: process.env.BENCH_REMOTE_IMAGE_NAME,
  image_id: imageId,
  cpu_mode: process.env.BENCH_CPU_MODE ?? "default",
  target_instance_id: process.env.BENCH_TARGET_INSTANCE_ID,
  load_instance_id: process.env.BENCH_LOAD_INSTANCE_ID,
  target_limits: {
    nano_cpus: 2_000_000_000,
    memory_bytes: 2_147_483_648,
    memory_swap_bytes: 2_147_483_648,
    cpuset: null,
  },
  k6_image: process.env.BENCH_K6_IMAGE,
  k6_version: process.env.BENCH_K6_VERSION,
  workload_script: scriptPath,
  workload_sha256: createHash("sha256").update(script).digest("hex"),
  k6_exit_code: Number(exitCodeText),
  raw_summary: summaryPath,
  result: {
    work_score: metric("work_score", "count"),
    http_reqs: metric("http_reqs", "count"),
    error_rate: errorRate,
    price_p95_ms: metric("http_req_duration{tier:price}", "p(95)"),
    stats_p95_ms: metric("http_req_duration{tier:stats}", "p(95)"),
    risk_p95_ms: metric("http_req_duration{tier:risk}", "p(95)"),
  },
};

await appendFile("benchmarks/history.jsonl", `${JSON.stringify(entry)}\n`);
console.log(`recorded remote ${profile} run ${runId} in benchmarks/history.jsonl`);

