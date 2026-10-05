# Evaluations

**Nothing is measured yet.** This file will only ever hold numbers that were actually measured, each with its method, sample size and limitations.

| What | Planned in | Status |
| --- | --- | --- |
| Precision and recall per entity on about 5,000 synthetic samples from a generator written independently of the detectors | Issue D9 | Not started |
| Added latency (p50, p95) against the mock model | Issue D9 | Not started |
| Detector conformance: every sg-pii-rules fixture passes in Go | Issue D3 | **Done: 369 / 369** (see below) |
| Data-disclosure and adversarial-prompt run with Project Moonshot | Milestone 3 | Not started |

Known limitation, stated in advance: synthetic data flatters rule-based detectors, because real text is messier than any generator.

Vetted's review of this repository's pull requests is measured in Vetted's own `EVALS.md`.

## Cost bound (issue D6, computed 2026-10-05)

With `gpt-6-luna` at US$0.10 / US$0.50 per million input / output tokens (OpenAI's model page, primary source), the 64 KB request limit and the default 2,048-token output cap, one request can cost at most **US$0.0076**: (65,536 bytes + 16) × 0.10 / 10⁶ + 2,048 × 0.50 / 10⁶. The default daily budget of US$0.20 therefore allows at least 26 worst-case requests, and at most about US$6 a month. This is an upper bound by construction, not a measurement; no real request has been sent yet, because `OPENAI_API_KEY` is not configured.

## Detector conformance (issue D3, measured 2026-10-05)

`go test ./internal/detect` runs every case in the vendored sg-pii-rules `v0.2.0-rc.1` fixtures through Discreet's Go engine: **369 of 369 pass**, across NRIC, NRIC look-alikes, phone, email, card, postal code, unit number and date of birth. These cases are a behaviour specification shared with Vetted, not a measure of real-world accuracy (that is the benchmark above).

Deliberate breaks, each restored from a backup afterwards:

| Break | Tests that failed |
| --- | --- |
| Wrong M-series check-letter table | 2 |
| Luhn check always passes | 3 |
| `value` groups ignored | 52 |
| Overlap tie-break reversed | 0 at first, because no shared case has two detectors matching the same span; 3 after adding toy-detector overlap tests (tracked for the shared suite in vetted#44) |
| A vendored fixture edited by hand | 1 (the checksum test) |

Speed, for scale only: on a dense synthetic text with every kind of personal data in each sentence, the engine scans about **3.9 MB/s** on an Apple-silicon Mac (`go test -bench BenchmarkDetect ./internal/detect`), roughly half a millisecond for a 2 KB chat message. Latency through the whole gateway is measured in issue D9.

