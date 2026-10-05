# Numbers for the CV (after Milestone 2)

Each number says how it was measured and on what sample, so it survives "how do you know?". All data is synthetic.

| Number | How it was measured | Sample |
| --- | --- | --- |
| **0 personal data values reached the model** in gateway tests, including a scan of every log line and the audit log for the planted values | `go test ./internal/gateway`; the model stub records exactly what it received | NRIC, phone, email and card values in every request shape tested |
| **369 / 369 shared conformance cases** pass in Go, from a Go engine written independently of the TypeScript reference | `go test ./internal/detect` against vendored sg-pii-rules v0.2.0-rc.1 | 8 entity types; five deliberate breaks each caught |
| **76.5% recall, 92.3% precision** on synthetic text; **100% on every supported writing style**; all false alarms are bare 8-digit numbers read as phones | `make bench`, generator written independently of the rules, fixed seed | 5,000 samples, 7,531 pieces of personal data |
| **About 3 ms added per request**, of which 2.9 ms is writing the tamper-evident audit record to disk | `make bench`, against a baseline server without Discreet | 2,000 sequential requests, Apple-silicon Mac |
| **At most US$0.0076 per request and US$0.20 a day** for the real model, by construction; US$0 in public (mock-only) | Worst case: (64 KB + 16) × input price + 2,048 × output price (`gpt-6-luna`, OpenAI's model page) | Upper bound, not a measurement; no paid call made yet |

Delivery: 13 issues planned on 2026-10-02; 11 build and evidence pull requests merged into `next` by Claude Code (`ai-merged`) by 2026-10-05, each with CI (tests with the race detector, `govulncheck`, CodeQL, a container smoke test) and reviewed in shadow mode by Vetted; 458 Go tests; 9 decision records.
