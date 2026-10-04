# Evaluations

**Nothing is measured yet.** This file will only ever hold numbers that were actually measured, each with its method, sample size and limitations.

| What | Planned in | Status |
| --- | --- | --- |
| Precision and recall per entity on about 5,000 synthetic samples from a generator written independently of the detectors | Issue D9 | Not started |
| Added latency (p50, p95) against the mock model | Issue D9 | Not started |
| Detector conformance: every sg-pii-rules fixture passes in Go | Issue D3 | Not started |
| Data-disclosure and adversarial-prompt run with Project Moonshot | Milestone 3 | Not started |

Known limitation, stated in advance: synthetic data flatters rule-based detectors, because real text is messier than any generator.

Vetted's review of this repository's pull requests is measured in Vetted's own `EVALS.md`.
