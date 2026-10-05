# Threat model

**Draft.** Filled in as each feature lands; reviewed in issue D11.

## What Discreet protects

- Personal data in requests: NRIC/FIN numbers, phone numbers, emails, payment cards, addresses and dates of birth.
- The vault that maps placeholders back to real values.
- The audit log's integrity.
- The operator's model budget.

## Trust boundaries

1. **Calling application → Discreet.** Requests may contain anything, including prompt-injection text and real personal data.
2. **Discreet → model provider.** Everything crossing this line must already be tokenised. This is the boundary Discreet exists to enforce.
3. **Discreet → disk.** Only the audit log is written, holding keyed hashes and counts, never raw text.

## Threats to address in Milestone 2

| Threat | Planned mitigation | Issue |
| --- | --- | --- |
| Personal data the detectors miss reaches the model | Shared conformance suite with hard negatives (369/369 pass in Go); measured recall published in EVALS | D3 (done), D9 |
| Rules silently changed or swapped | Vendored release checked against `SHA256SUMS` on every test run; embedded in the binary (ADR 0003) | D3 (done) |
| The vault leaks values through logs or errors | Values encrypted in memory (AES-256-GCM, per-process key), bound to their key, deleted when the request ends, expired after 10 minutes; vault and session print as redacted (tests) | D4 (done); log scan in D5 |
| The model invents placeholders to extract other values | Only placeholders issued for that request are rehydrated (test) | D4 (done) |
| Someone edits the audit log to hide a request | SHA-256 hash chain; `audit verify` names the first broken record; a broken log is refused at start-up (tests for a changed byte, a deleted record, swapped records, a rewritten record) | D7 (done) |
| Someone with write access rewrites the whole chain, or cuts records off the end | Not prevented: `verify` prints the head hash to record elsewhere; automatic anchoring is Milestone 3 work (ADR 0005) | Accepted for v0.1 |
| Short identifiers are guessed from audit hashes | HMAC-SHA-256 with a server secret of at least 32 characters instead of plain hashes | D7 (done) |
| The public demo is used to spend money | Mock model unless a valid access token is present; daily budget; rate limit | D6, D10 |
| A compromised dependency or Action | Standard library only (ADR 0001), SHA-pinned Actions, Dependabot, `govulncheck`, CodeQL | D1 |
