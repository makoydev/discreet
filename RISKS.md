# Risks

Reviewed at the start of each issue and at each milestone. Likelihood and impact are Low / Medium / High.

| # | Risk | Likelihood | Impact | Mitigation | Owner |
| --- | --- | --- | --- | --- | --- |
| R1 | Go and TypeScript detectors disagree on edge cases | Low | Medium | Both use RE2 and must pass the same sg-pii-rules fixtures; any disagreement becomes a new fixture | Claude Code |
| R2 | Postal code and date-of-birth rules are noisy on real text | Medium | Medium | Context clues, hard negatives, false-positive rates published rather than hidden | Claude Code |
| R3 | The public demo is abused | Medium | Low | Mock-only without a token, per-IP rate limit, nothing kept but keyed hashes and counts | Claude Code |
| R4 | The hosting account is not ready by the week of 2 Nov | Low | Medium | Asked for early; fallback is "run locally in five minutes" with the slip recorded | Michael |
| R5 | Pilot evidence stays mock-only because `OPENAI_API_KEY` is not set | High | Medium | EVALS states reviews run vs. reviews with a real model; setting the key takes about 10 minutes | Michael |
| R6 | M2 slips past 22 Nov | Medium | High | Plan finishes about 10 days early; anything that doesn't fit moves to "Next" in the README with the reason | Claude Code (plan), Michael (review) |
| R7 | Michael can't explain AI-drafted Go code in an interview | Medium | High | Standard library only, small pull requests, an "If asked in an interview" section on each, ADRs he confirms | Michael |
