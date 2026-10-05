# Evaluations

This file only holds numbers that were actually measured, each with its method, sample size and limitations.

| What | Planned in | Status |
| --- | --- | --- |
| Precision and recall per entity on about 5,000 synthetic samples from a generator written independently of the detectors | Issue D9 | **Done** (below) |
| Added latency (p50, p95) against the mock model | Issue D9 | **Done** (below) |
| Detector conformance: every sg-pii-rules fixture passes in Go | Issue D3 | **Done: 369 / 369** (see below) |
| Data-disclosure and adversarial-prompt run with Project Moonshot | Milestone 3 | Not started |

Known limitation, stated in advance: synthetic data flatters rule-based detectors, because real text is messier than any generator.

Vetted's review of this repository's pull requests is measured in Vetted's own `EVALS.md`.

## Benchmark: detection on 5,000 synthetic samples (issue D9, measured 2026-10-05)

Reproduce with `make bench` (`go run ./cmd/benchmark`). The generator (`internal/bench`) is written independently of the detection rules: its own NRIC checksum and Luhn code, its own sentence templates, and on purpose it writes personal data in styles a rule-based detector may not handle (an NRIC with spaces, a phone in 2-2-2-2 groups, a date of birth after "Birthday"). Every sample mixes 0 to 3 pieces of personal data with 0 to 2 look-alike decoys (order numbers, OTPs, amounts, timestamps, tracking numbers). A value counts as **caught** only if a detection of the same kind covers it completely; partly covered still leaks, so it counts as missed.

| Entity | True values | Caught as the right kind (recall) | Protected by any detection | Detections | False alarms | Precision |
| --- | --- | --- | --- | --- | --- | --- |
| `CARD` | 1075 | 933 (86.8%) | 933 (86.8%) | 933 | 0 | 100.0% |
| `DOB` | 1044 | 614 (58.8%) | 614 (58.8%) | 614 | 0 | 100.0% |
| `EMAIL` | 1083 | 996 (92.0%) | 996 (92.0%) | 996 | 0 | 100.0% |
| `NRIC` | 1105 | 880 (79.6%) | 880 (79.6%) | 880 | 0 | 100.0% |
| `PHONE` | 1112 | 856 (77.0%) | 856 (77.0%) | 1338 | 482 | 64.0% |
| `POSTAL` | 1058 | 779 (73.6%) | 779 (73.6%) | 779 | 0 | 100.0% |
| `UNIT` | 1054 | 702 (66.6%) | 702 (66.6%) | 702 | 0 | 100.0% |
| **All** | 7531 | 5760 (76.5%) | 5760 (76.5%) | 6242 | 482 | 92.3% |

Recall by writing style:

| Entity | Style | Caught / total |
| --- | --- | --- |
| `CARD` | double spaces | 0 / 142 (0%) |
| `CARD` | hyphens | 124 / 124 (100%) |
| `CARD` | plain | 141 / 141 (100%) |
| `CARD` | spaced | 668 / 668 (100%) |
| `DOB` | DOB dd/mm/yyyy | 155 / 155 (100%) |
| `DOB` | JSON ISO | 171 / 171 (100%) |
| `DOB` | birthday clue | 0 / 147 (0%) |
| `DOB` | born on Month d, yyyy | 141 / 141 (100%) |
| `DOB` | date of birth d Month yyyy | 147 / 147 (100%) |
| `DOB` | two-digit year | 0 / 124 (0%) |
| `DOB` | words between clue and date | 0 / 159 (0%) |
| `EMAIL` | obfuscated | 0 / 87 (0%) |
| `EMAIL` | plus tag | 127 / 127 (100%) |
| `EMAIL` | standard | 764 / 764 (100%) |
| `EMAIL` | uppercase | 105 / 105 (100%) |
| `NRIC` | hyphenated | 0 / 123 (0%) |
| `NRIC` | lowercase | 104 / 104 (100%) |
| `NRIC` | mistyped check letter | 102 / 102 (100%) |
| `NRIC` | spaced | 0 / 102 (0%) |
| `NRIC` | standard | 674 / 674 (100%) |
| `PHONE` | (65) | 138 / 138 (100%) |
| `PHONE` | +65 joined | 160 / 160 (100%) |
| `PHONE` | +65 space | 145 / 145 (100%) |
| `PHONE` | 2-2-2-2 | 0 / 124 (0%) |
| `PHONE` | 3-5 | 0 / 132 (0%) |
| `PHONE` | 4-4 hyphen | 152 / 152 (100%) |
| `PHONE` | 4-4 space | 123 / 123 (100%) |
| `PHONE` | plain | 138 / 138 (100%) |
| `POSTAL` | S'pore abbreviation | 0 / 134 (0%) |
| `POSTAL` | S(…) | 118 / 118 (100%) |
| `POSTAL` | after Singapore | 535 / 535 (100%) |
| `POSTAL` | no clue | 0 / 145 (0%) |
| `POSTAL` | postal code | 126 / 126 (100%) |
| `UNIT` | # with space | 0 / 174 (0%) |
| `UNIT` | #floor-unit | 702 / 702 (100%) |
| `UNIT` | Unit without # | 0 / 178 (0%) |

False-alarm examples (synthetic text):
- `PHONE`: "92121752"
- `PHONE`: "60130067"
- `PHONE`: "65478180"

**Reading this honestly.**

- **Every miss is a writing style the rules don't support yet**, and every supported style scored 100%: NRICs with spaces or hyphens, phones in 2-2-2-2 or 3-5 groups, cards with double spaces, obfuscated emails ("name at example dot com"), postal codes with no clue or after "S'pore", unit numbers without `#` or with a space after it, and dates of birth after "Birthday", with words between clue and date, or with a two-digit year. The generator's mix of styles is a choice, so the overall 76.5% depends on it; the per-style table is the more useful result. These gaps are candidates for sg-pii-rules in Milestone 3.
- **False alarms: 482, all `PHONE`**, all bare eight-digit decoys starting 6, 8 or 9 (order and batch numbers). This is the trade-off sg-pii-rules accepted in its ADR 0007: a false alarm replaces a number with a placeholder; a miss leaks a phone number. Every other entity had no false alarms.
- **Synthetic data flatters rule-based detectors.** Real text is messier than any generator, and real personal data can't be used to check. Treat these numbers as an upper bound on recall for the styles covered.

## Latency (issue D9, measured 2026-10-05)

Latency over 2000 sequential requests on loopback, each with one benchmark sample:

| | p50 | p95 | p99 |
| --- | --- | --- | --- |
| Baseline: same HTTP and JSON work, mock model, no Discreet | 0.04 ms | 0.11 ms | 0.17 ms |
| Through Discreet (detect, protect, restore, audit write with fsync) | 3.05 ms | 4.90 ms | 11.74 ms |
| **Added by Discreet** | **3.00 ms** | **4.79 ms** | 11.57 ms |
| of which: detection alone | 0.079 ms | 0.230 ms | 0.578 ms |
| of which: one audit write, flushed to disk | 2.884 ms | 3.628 ms | 4.828 ms |

Measured on an Apple-silicon Mac over loopback, sequential requests, each carrying one benchmark sample, with the mock model. The baseline server does the same HTTP and JSON work and the same mock call without Discreet, so the difference is what Discreet adds.

**Where the time goes:** almost all of the roughly 3 ms is the audit record being flushed to disk before the answer is sent (ADR 0005); detection itself takes under 0.1 ms. That flush is the price of "no answer without a record". Tail latency (p99) varied between runs (4.7 ms and 11.7 ms on the same machine), so treat it as indicative. Against a real model, which takes hundreds of milliseconds to seconds, the added time is small. Disk flush speed differs by machine and hosting; this will be re-measured on the deployment (issue D10).

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

