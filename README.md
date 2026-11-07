# pheron-relay

The protocol and deployment contract for Pheron's outbound relay. It connects
paired mobile clients to Agentflow's dedicated mobile gateway without exposing
the desktop loopback API or privileged process token.

`relay` is the Go data-plane package. Mount `relay.NewServer(config, store,
auth).Handler()` in a TLS-enabled HTTP server. The caller supplies the
control-plane ticket/key-proof verifier and a shared, atomic `relay.Store` for
production; `NewMemoryStore` is intended for single-process development and
tests only.

- Normative protocol: [relay tunnel v1](docs/contracts/relay-tunnel-v1.md)
- Runtime decision: [ADR 0001](docs/adr/0001-relay-runtime-and-deployment.md)
- Contract fixtures: [`fixtures/relay-tunnel-v1`](fixtures/relay-tunnel-v1)

Validate the documentation and fixtures with:

```sh
python3 scripts/validate_docs.py
python3 scripts/validate_fixtures.py
env -u GOROOT go test ./...
env -u GOROOT go vet ./...
```
# pheron-relay
