# Pheron Relay context

## Purpose

`pheron-relay` contains the transport contract and deployment decisions for the
Pheron relay. The relay lets a paired mobile device reach the isolated mobile
gateway implemented by Agentflow without exposing Agentflow's loopback desktop
API or its per-process token.

## Current architecture

This repository is documentation- and fixture-first. Protocol specifications
live under `docs/`, executable, non-secret fixture data lives under `fixtures/`,
and small standard-library scripts in `scripts/` validate documentation and
fixtures. Agentflow owns the mobile gateway; Pheron App owns the mobile client.

## Commands

```sh
python3 scripts/validate_docs.py
python3 scripts/validate_fixtures.py
```

## Conventions

- Treat `docs/contracts/relay-tunnel-v1.md` as the normative relay contract.
- Use opaque placeholder values only in fixtures and examples; never commit
  payloads, credentials, tickets, proofs, or process tokens.
- Keep the relay byte-transparent after authentication: it is never a generic
  proxy for Agentflow `/api/*`.
- Write stable machine-readable error codes; diagnostic messages are not API
  contracts.
- Use Markdown for decisions and JSON for fixtures. Keep fixtures deterministic
  and validate them with the supplied scripts.
