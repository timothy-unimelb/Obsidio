import { createHash } from "node:crypto";
import { appendFile, readFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import process from "node:process";

const [stage, profile, runId, summaryPath, imageName, exitCodeText, scriptPath] = process.argv.slice(2);

if (!stage || !profile || !runId || !summaryPath || !imageName || exitCodeText === undefined || !scriptPath) {
  console.error("usage: node benchmarks/record-result.mjs <stage> <profile> <run-id> <summary> <image> <k6-exit-code> <script>");
  process.exit(64);
}

const command = (program, args, fallback = "unknown") => {
  try {
    return execFileSync(program, args, { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
  } catch {
    return fallback;
  }
};

const summary = JSON.parse(await readFile(summaryPath, "utf8"));
const script = await readFile(scriptPath);
const metrics = summary.metrics ?? {};
const metric = (name, field) => metrics[name]?.values?.[field] ?? metrics[name]?.[field];
const errorRate = metrics.http_req_failed?.values?.rate
  ?? metrics.http_req_failed?.rate
  ?? metrics.http_req_failed?.value;

const ignoredEvidencePaths = /^(benchmarks\/results\/|benchmarks\/history\.jsonl$)/;
const statusLines = command("git", ["status", "--porcelain", "--untracked-files=all"], "")
  .split("\n")
  .filter(Boolean);
const meaningfulChanges = statusLines.filter((line) => {
  const path = line.slice(3).replace(/^"|"$/g, "");
  return !ignoredEvidencePaths.test(path);
});

const entry = {
  recorded_at: new Date().toISOString(),
  comparison_set: process.env.BENCH_COMPARISON_SET ?? "unassigned",
  environment_id: process.env.BENCH_ENVIRONMENT_ID ?? "local",
  target_relationship: process.env.BENCH_TARGET_RELATIONSHIP ?? "same-host",
  stage,
  profile,
  run_id: runId,
  verdict: "pending",
  git_sha: command("git", ["rev-parse", "HEAD"]),
  source_dirty: meaningfulChanges.length > 0,
  meaningful_changes: meaningfulChanges,
  image_name: imageName,
  image_id: command("docker", ["image", "inspect", "--format", "{{.Id}}", imageName]),
  cpu_mode: process.env.BENCH_CPU_MODE ?? "default",
  k6_version: command("k6", ["version"]),
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
console.log(`recorded ${profile} run ${runId} in benchmarks/history.jsonl`);
