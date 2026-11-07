# pheron-relay

The protocol and deployment contract for Pheron's outbound relay. It connects
paired mobile clients to Agentflow's dedicated mobile gateway without exposing
the desktop loopback API or privileged process token.

- Normative protocol: [relay tunnel v1](docs/contracts/relay-tunnel-v1.md)
- Runtime decision: [ADR 0001](docs/adr/0001-relay-runtime-and-deployment.md)
- Contract fixtures: [`fixtures/relay-tunnel-v1`](fixtures/relay-tunnel-v1)

Validate the documentation and fixtures with:

```sh
python3 scripts/validate_docs.py
python3 scripts/validate_fixtures.py
```
# pheron-relay
