# Obsidio visual story

Three responsive, visual-first pages explaining the challenge, the submitted Go solution, and the next optimization experiments:

- `/Obsidio/` — the traffic siege, scoring model, endpoint mix, ramp, and latency target
- `/Obsidio/solution/` — the architecture, bounded concurrency, allocation fix, and measured result
- `/Obsidio/optimizations/` — the evidence-led optimization order, tradeoffs, and native-kernel decision

## Run locally

```bash
npm ci
npm run dev
```

The local route is `http://localhost:3000/Obsidio/`. Use `npm test` to create and verify the static GitHub Pages export in `out/`.
