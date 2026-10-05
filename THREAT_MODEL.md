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
| The vault leaks values through logs or errors | Values encrypted in memory, short expiry, a test that scans logs for planted values | D4 |
| The model invents placeholders to extract other values | Only placeholders issued for that request are rehydrated | D4 |
| Someone edits the audit log to hide a request | SHA-256 hash chain; `audit verify` names the first broken record | D7 |
| Short identifiers are guessed from audit hashes | HMAC-SHA-256 with a server secret instead of plain hashes | D7 |
| The public demo is used to spend money | Mock model unless a valid access token is present; daily budget; rate limit | D6, D10 |
| A compromised dependency or Action | Standard library only (ADR 0001), SHA-pinned Actions, Dependabot, `govulncheck`, CodeQL | D1 |
