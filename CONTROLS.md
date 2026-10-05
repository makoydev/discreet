# Controls

Each Discreet feature, the principle or obligation it supports, and the source. **Draft v0:** features are listed as they are planned, and each row is mapped to a primary source and graded (primary, secondary or unverified) in issue D11. No row claims compliance with anything.

| Feature | Supports | Source | Status |
| --- | --- | --- | --- |
| Personal data detected with the shared, checksummed sg-pii-rules release (NRIC/FIN, phone, email, card, postal code, unit number, date of birth) | Data minimisation; protecting national identification numbers | PDPA; PDPC Advisory Guidelines on NRIC and other national identification numbers | Detection (D3, 369/369 shared cases) and replacement with placeholders (D4) in place; mapping pending |
| Purpose declared in a header; automated eligibility decisions refused | Human accountability for decisions about people | IMDA Model AI Governance Framework for Agentic AI; PDPC guidelines on AI recommendation and decision systems | Planned (D5); mapping pending |
| Hash-chained audit log with `verify` and CSV export; prompts and responses kept only as keyed hashes | Accountability, traceability, incident investigation | IMDA Model AI Governance Framework for Generative AI | In place (D7); mapping pending |
| Daily budget, access token, rate limit | Bounding risk and cost | OWASP Top 10 for LLM Applications 2025 (unbounded consumption) | Planned (D6); mapping pending |
| Pinned Actions, `govulncheck`, CodeQL, reviewed pull requests | Secure development | OWASP Top 10 for LLM Applications 2025 (supply chain) | In place (D1); mapping pending |
