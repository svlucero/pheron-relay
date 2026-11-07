# Pheron Relay contributor context

## Purpose

Pheron Relay is the provider-neutral Go data plane for the Pheron mobile
relay. It connects an authenticated desktop channel with a mobile channel and
forwards the inner Agentflow TLS stream as opaque bytes. It must never become a
proxy for Agentflow desktop API or process-token surfaces.

## Architecture

The normative tunnel contract is `docs/contracts/relay-tunnel-v1.md`; runtime
and deployment choices are recorded in `docs/adr/`. The `relay/` package owns
WebSocket admission, locator rendezvous, leases, stream state, and bounded
byte forwarding. A caller injects a ticket authenticator and a routing store;
the in-memory store is only for local development and tests. Contract fixtures
are in `fixtures/relay-tunnel-v1/`, and standard-library validators are in
`scripts/`.

## Commands

```sh
env -u GOROOT go test ./...
env -u GOROOT go test -race ./...
env -u GOROOT go vet ./...
python3 scripts/validate_docs.py
python3 scripts/validate_fixtures.py
```

## Conventions

- Format Go with `gofmt` and keep dependencies injected and provider-neutral.
- Preserve opaque payload forwarding after admission; never log payload bytes,
  locators, tickets, proofs, credentials, query strings, or process tokens.
- Use stable machine-readable transport errors; prose diagnostics are not a
  contract.
- Keep deployment artifacts reproducible and secret-free. Production replicas
  require an atomic shared routing/lease store and distributed rate limits.
- Update `.memory/architecture.md`, `.memory/coding_standards.md`, and
  `.memory/api_contracts.md` with verified, durable knowledge on every task.
