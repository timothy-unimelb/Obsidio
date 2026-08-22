import { appendFile, readFile } from "node:fs/promises";
import process from "node:process";

const [decisionPath] = process.argv.slice(2);
if (!decisionPath) {
  console.error("usage: node benchmarks/record-decision.mjs <decision.json>");
  process.exit(64);
}

const decision = JSON.parse(await readFile(decisionPath, "utf8"));
if (decision.type !== "comparison_decision" || !decision.comparison_set || !decision.verdict) {
  throw new Error("decision must include type=comparison_decision, comparison_set, and verdict");
}

const historyPath = "benchmarks/history.jsonl";
const history = await readFile(historyPath, "utf8");
const duplicate = history
  .split("\n")
  .filter(Boolean)
  .map((line) => JSON.parse(line))
  .some((entry) => entry.type === decision.type && entry.comparison_set === decision.comparison_set);

if (duplicate) {
  throw new Error(`decision already recorded for ${decision.comparison_set}`);
}

await appendFile(historyPath, `${JSON.stringify(decision)}\n`);
console.log(`recorded decision for ${decision.comparison_set}`);
