# Obsidio visual story

Four responsive, visual-first pages explaining the challenge, the submitted Go solution, the next optimization experiments, and their measured progression:

- `/Obsidio/` — the traffic siege, scoring model, endpoint mix, ramp, and latency target
- `/Obsidio/solution/` — the architecture, bounded concurrency, allocation fix, and measured result
- `/Obsidio/optimizations/` — a visual guide to the request path, batching, fixed-shape SHA rounds, tradeoffs, and native-kernel decision
- `/Obsidio/performance/` — the reproducible baseline, measured checkpoints, raw evidence, and pending progression

## Run locally

```bash
npm ci
npm run dev
```

The local route is `http://localhost:3000/Obsidio/`. Use `npm test` to create and verify the static GitHub Pages export in `out/`.
