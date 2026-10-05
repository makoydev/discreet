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
| Personal data travels in request fields the gateway doesn't inspect (`tools`, `user`, metadata) | Requests are rebuilt from model, messages, token limit and temperature only; tools, images and tool-role messages are rejected (ADR 0006, test) | D5 (done) |
| Personal data ends up in server logs | Logs carry counts and decisions, never text; a test captures every log line and searches it for planted values | D5 (done) |
| A caller uses Discreet for automated eligibility decisions | Required purpose header; denied purposes refused with 403 before the model is called, and recorded (ADR 0006) | D5 (done) |
| The public demo is used to spend money | Mock model unless a valid access token is present (a wrong or missing token gets the mock, test); worst-case cost reserved against a hard daily budget that survives restarts (tests, including 100 parallel requests); 30 requests a minute per address (ADR 0007) | D6 (done), D10 |
| An access token leaks from memory or logs | Only SHA-256 hashes kept, compared in constant time; tokens and provider keys never logged; the caller's token is never forwarded | D6 (done) |
| The demo page is used to inject script or exfiltrate what people type | Strict Content-Security-Policy (own scripts only, connections only to this server), user text inserted as text never HTML, no third-party scripts; always the mock model (test) | D8 (done) |
| The public demo is reconfigured to reach a paid model | `fly.toml` never sets an upstream; `docs/DEPLOY.md` forbids setting one as a secret; without an upstream every request gets the mock (ADR 0007, ADR 0009) | D10 (done) |
| The container is used to escalate on the host | Distroless image (no shell), non-root user, static binary, images pinned by digest, built and smoke-tested in CI | D10 (done) |
| A compromised dependency or Action | Standard library only (ADR 0001), SHA-pinned Actions, Dependabot, `govulncheck`, CodeQL | D1 |
