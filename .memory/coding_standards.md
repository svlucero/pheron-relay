# Coding standards

- Use concise Markdown for normative specifications and ADRs; use JSON fixtures
  with no real credentials or secrets.
- Use explicit versioned message types and stable machine-readable error codes.
- Keep operational diagnostics free of payloads, proofs, tokens, tickets, and
  Agentflow's privileged process secret.
- Validator scripts should use Python's standard library and be runnable from
  the repository root with `python3`.
- Use Conventional Commits (for example, `docs:` and `test:`).

See [architecture](architecture.md) for repository layout.
