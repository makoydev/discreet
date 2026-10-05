# 0005. A hash-chained JSON-lines audit log with keyed hashes of prompts and responses

Status: accepted. Drafted by Claude Code; reviewed and accepted by Michael Mendoza on 2026-10-06.
Date: 2026-10-05

## Context

An operator must be able to show afterwards what Discreet did with each request (allowed, refused, blocked, what kinds of personal data, which model, what it cost) without the log itself becoming a store of personal data, and to prove the log hasn't been edited (issue D7).

## Options

1. **Append-only JSON lines, each record carrying the SHA-256 of the previous one.** Anyone can open the file, read it, edit a byte and watch `discreet audit verify` fail, which is the clearest tamper demonstration there is.
2. **A SQLite table** with the same chain. Better for queries, but harder for a reviewer to inspect, and the tamper demo needs a database tool.
3. **An external append-only service** (a transparency log, cloud object lock). Stronger against an attacker with full server access, but adds cost and an account, against the US$20/month cap.

## Decision

Option 1.

- Fields: sequence number, UTC time, request ID, tenant, purpose, decision, reason, upstream, model, counts per entity, HMAC-SHA-256 of prompt and response, cost, latency, previous hash, hash. The hash covers the record's JSON with the hash field empty; field order is fixed, so it's deterministic.
- Prompts and responses are stored only as **HMAC-SHA-256 with a server secret** (`DISCREET_HMAC_KEY`, at least 32 characters; Discreet refuses to start without it). Plain hashes of short identifiers such as NRICs could be reversed by hashing every possible number. With the key, an investigator holding a disputed text can prove whether it was the one sent.
- `discreet audit verify` checks sequence, links and hashes and names the first broken record. It needs no key. Discreet also verifies an existing log at start-up and **refuses to append to a broken chain**.
- `discreet audit export -csv` exports only a verified log, with spreadsheet formulas neutralised.
- The daily budget (issue D6) is recomputed from the log at start-up, so a restart can't reset spending.

## Consequences

- **Limits, stated plainly:** someone with write access who rewrites the entire chain from some point on, recomputing every hash, produces a log that verifies. So does cutting records off the end. `verify` prints the head hash so it can be recorded somewhere else (a ticket, a daily email) and compared later; anchoring it automatically is Milestone 3 work.
- Writes are flushed to disk (`fsync`) before the response is sent, which costs a little latency per request (measured in issue D9).
- The purpose header is recorded as given; the gateway scrubs personal data from it first (issue D5).
