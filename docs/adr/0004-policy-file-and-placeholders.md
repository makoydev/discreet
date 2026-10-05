# 0004. Placeholders per request, an encrypted in-memory vault, and a YAML policy

Status: accepted. Drafted by Claude Code; reviewed and accepted by Michael Mendoza on 2026-10-06.
Date: 2026-10-05

## Context

Discreet replaces personal data before a request reaches the model and puts it back in the answer (issue D4). Plain redaction ("call [REDACTED] about [REDACTED]") makes answers useless; but anything that can put values back must store them somewhere, and that store becomes the most sensitive thing in the system. The brief asks for policy as YAML, with `tokenise`, `redact` or `block` per entity.

## Options

1. **Numbered placeholders per request (`<NRIC_1>`), values encrypted in memory for minutes.** The model can still tell two people apart and refer back to them.
2. **Format-preserving fake values** (a different valid-looking NRIC). Reads more naturally, but a fake that looks real can be mistaken for real data downstream, and mapping back is ambiguous if the model rewrites it.
3. **A persistent vault (SQLite).** Survives restarts, but stores encrypted personal data on disk for no benefit to non-streaming requests that last seconds.

## Decision

Option 1.

- A **session** per request numbers placeholders per entity and reuses one for the same value anywhere in the request. It remembers values only by an HMAC-SHA-256 with a per-session random key, never in plain text.
- The **vault** keeps values encrypted with AES-256-GCM under a random key that exists only in this process, binds each value to its key (as additional data), deletes them when the request ends, and expires anything left after 10 minutes. Nothing in it formats or logs a value.
- **Restore** only replaces placeholders the same session issued, so a model that invents `<NRIC_7>` can't fish for another request's values.
- The **policy** is YAML, parsed strictly: an unknown key or action (`tokenize`) is an error, so a typo can't weaken protection. The parser is `go.yaml.in/yaml/v3` v3.0.5, the YAML organisation's maintained fork; the original `gopkg.in/yaml.v3` has been archived since April 2025 (checked 2026-10-05). It is the first third-party module (ADR 0001 allowed exactly this one).
- Default policy: everything `tokenise`, except `CARD: redact` (a model never needs a card number back).

## Consequences

- A restart loses in-flight values; for non-streaming requests that only fails requests already running.
- The model sees placeholders, which can make answers slightly stilted ("Dear <NRIC_1>"); the demo page shows this honestly.
- `block` refuses the whole request on the first blocked entity; values already placed in the vault for that request are deleted when the gateway closes the session.
