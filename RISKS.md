# Risks

Reviewed at the start of each issue and at each milestone. Likelihood and impact are Low / Medium / High.

| # | Risk | Likelihood | Impact | Mitigation | Owner |
| --- | --- | --- | --- | --- | --- |
| R1 | Go and TypeScript detectors disagree on edge cases | Low | Medium | Both use RE2 and pass the same 369 fixtures (Go: 369/369). The tie-break rule had no shared case; tested in each implementation, shared cases tracked in vetted#44 | Claude Code |
| R2 | Postal code and date-of-birth rules are noisy on real text | Low | Medium | Clue-gated; 0 false alarms for both in the benchmark and on 36 MB of real code; misses published (EVALS) | Claude Code |
| R3 | The public demo is abused | Medium | Low | Mock-only without a token, per-IP rate limit, nothing kept but keyed hashes and counts | Claude Code |
| R4 | The hosting account is not ready | Medium | Medium | Everything is ready except the account (docs/DEPLOY.md, about 10 minutes); fallback is the Docker quickstart | Michael |
| R5 | Pilot evidence stays mock-only because `OPENAI_API_KEY` is not set | High | Medium | EVALS states reviews run vs. reviews with a real model; setting the key takes about 10 minutes | Michael |
| R6 | M2 slips past 22 Nov | Low | High | All build issues merged into `next` by 2026-10-05; remaining: Michael's review, the Fly.io account, and the pilot switch on 14 Oct | Michael |
| R7 | Michael can't explain AI-drafted Go code in an interview | Medium | High | Standard library only, small pull requests, an "If asked in an interview" section on each, ADRs he confirms | Michael |
| R8 | Benchmark recall is read as real-world accuracy | Medium | Medium | EVALS states that synthetic data flatters rules and gives recall per writing style, not just the total | Michael (in interviews) |
| R9 | Vetted's pilot misses the new entities because it vendors sg-pii-rules v0.1.0 | High until M3 | Low (synthetic test data only) | Upgrade Vetted to v0.2.0 in Milestone 3; recorded in the threat model | Claude Code |
