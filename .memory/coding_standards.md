# Coding standards

- Go code is formatted with `gofmt`; run `go test ./...` and `go vet ./...`
  (use `env -u GOROOT` when the shell has a stale toolchain override).
- Keep data-plane dependencies injected: implementations of `TicketAuthenticator`
  validate tickets/key proofs and implementations of `Store` provide atomic
  multi-replica routing. Do not add credentials or a local-only production store.
- Write concise Markdown contracts and ADRs; use JSON for deterministic,
  non-secret fixture scenarios.
- Version every wire message and use stable machine-readable error codes;
  diagnostic wording is never contractual.
- Examples and fixtures use opaque placeholders only. Never write payloads,
  proofs, tickets, tokens, query strings, or Agentflow's process secret to logs
  or repository fixtures.
- Keep the relay byte-transparent after admission. A change must not make it a
  generic proxy for desktop API routes.
- Python validators use only the standard library and run from repository root
  with `python3 scripts/validate_docs.py` and `python3 scripts/validate_fixtures.py`.
- Use Conventional Commits, with focused `docs:` and `test:` commits.
- Runtime configuration comes from `RELAY_*` environment variables; examples
  contain names and safe development values only. Secret paths may be supplied
  at runtime, but values must never be logged, emitted as metrics labels, or
  committed.
- Operational metrics use bounded, aggregate event names only. Health and
  readiness responses contain only status; do not add routing or authentication
  diagnostics to them.
- Exercise socket changes with `go test -race ./...`; keep concurrency,
  disconnect, quota, and tenant-isolation coverage local and deterministic.

See [architecture](architecture.md) for boundaries.
