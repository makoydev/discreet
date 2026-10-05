# 0007. The real model only for access tokens, under a hard daily budget

Status: accepted. Drafted by Claude Code; reviewed and accepted by Michael Mendoza on 2026-10-06.
Date: 2026-10-05

## Context

The brief: anything that calls a paid model respects a hard daily budget, defaults to a mock in public demos, and public endpoints never relay to a paid model without an access token, with per-IP rate limits. The whole toolkit must stay under US$20 a month. Discreet's demo will be public (issue D10).

## Options

For access: **a valid token gets the real model; anyone else gets the mock**, or reject requests without a token (401). For the budget: **reserve each request's worst case before calling**, or count real costs after the fact.

## Decision

- **Tokens.** `DISCREET_ACCESS_TOKENS` holds `tenant:token` pairs (tokens of 24+ characters). Only SHA-256 hashes are kept in memory, compared in constant time. A missing or wrong token is not an error: the request goes to the free mock, and the `X-Discreet-Upstream` header says so. OpenAI's client libraries always send some key, so this keeps the public demo usable while making it impossible for it to spend money. The caller's token is never forwarded; the provider gets Discreet's own key.
- **One model.** The real upstream is pinned to one model (default `gpt-6-luna`, US$0.10 / US$0.50 per million input / output tokens, from OpenAI's model page on 2026-10-05). A request for another model is refused (400). An unpriced model needs explicit prices, or Discreet won't start.
- **Worst case first.** Before each real call, Discreet computes the most it could cost: the protected prompt's bytes (a token is at least one byte) plus 16 tokens per message, at the input price, plus the output cap (`DISCREET_MAX_OUTPUT_TOKENS`, default 2048) at the output price. With the 64 KB body limit that's at most **US$0.0076 per request**. The request is refused (429, recorded as `refused_budget`) unless today's spending plus all in-flight reservations plus this worst case fits the daily limit (`DISCREET_DAILY_BUDGET_USD`, default US$0.20, so at least 26 worst-case requests a day). The real cost from the provider's usage figures is recorded in the audit log.
- **Restarts.** Today's spending is summed from the audit log at start-up (UTC days), so a restart can't reset it.
- **Rate limit.** 30 requests a minute per client address, in bursts of up to 10. Behind a proxy, the client address comes only from a header the operator names (`DISCREET_CLIENT_IP_HEADER`, e.g. `Fly-Client-IP`), never a header a client could forge by default.

## Consequences

- US$0.20 a day is at most about US$6 a month for Discreet's real model, inside the toolkit's US$20 cap with Vetted's US$6 and hosting.
- The worst-case bound is deliberately pessimistic: real requests cost far less, so the cap may refuse earlier than strictly necessary.
- The OpenAI-compatible client supports Ollama through `DISCREET_UPSTREAM_TOKEN_PARAM=max_tokens` and zero prices. That path is tested only against a fake server, not a real Ollama.
