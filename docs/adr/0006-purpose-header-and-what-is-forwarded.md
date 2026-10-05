# 0006. Callers declare a purpose; eligibility decisions are refused; only known fields are forwarded

Status: accepted. Drafted by Claude Code; reviewed and accepted by Michael Mendoza on 2026-10-06.
Date: 2026-10-05

## Context

The brief's non-negotiable: Discreet never takes part in eligibility or benefits decisions; a person makes those. Discreet can't see what a caller does with an answer, but it can make the caller say what a request is for, refuse some purposes outright, and keep a record. Separately, OpenAI's request format has many fields beyond the messages (`tools`, `user`, `metadata`…), and any of them could carry personal data past a gateway that only inspects messages.

## Options

1. **A required `X-Discreet-Purpose` header**, checked against denied phrases in the policy plus a rule for variants ("eligibility" together with decide, determine, approve, reject or deny), refused with HTTP 403 and an audit record.
2. **Classify the prompt's intent with a model.** Catches undeclared misuse, but it's probabilistic, costs money per request, and sends the prompt to yet another model.
3. **No purpose at all**, relying on terms of use.

For fields: **forward only the model, messages, token limit and temperature**, or forward everything and scrub every string.

## Decision

Option 1, and forward only the known fields.

- Missing purpose: 400. Denied purpose: 403 with `purpose_refused`, recorded as `refused_purpose`; the model is never called. The purpose is stored in the audit log with any personal data replaced by the entity name and capped at 200 characters.
- Requests are rebuilt from model, messages (string or text parts), token limit and temperature. Other fields are ignored and never forwarded. Tools, images and tool-role messages are rejected (400), because v0.1 can't protect them. Streaming is rejected until it's supported.
- Every answer ends with a disclosure footer naming the Discreet request ID, so readers know personal data was replaced.

## Consequences

- A caller can lie about its purpose. The header makes the claim explicit and recorded, which is what accountability needs; it isn't a technical barrier. "Explain the eligibility criteria" is allowed; "decide eligibility" is not.
- Dropping fields means some OpenAI features (tools, JSON mode, images) don't work through Discreet v0.1. That's the price of not letting personal data slip through fields Discreet doesn't inspect.
- The footer changes the answer text, which can break callers that expect strict JSON output; v0.1 accepts that.
