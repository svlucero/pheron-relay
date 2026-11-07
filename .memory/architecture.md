# Architecture

- `pheron-relay` is a documentation-first repository for Pheron's outbound relay
  transport. It does not yet contain a relay runtime.
- The eventual relay is a rendezvous and byte-forwarding service between a
  desktop-owned outbound channel and a mobile client connection.
- Agentflow owns the authenticated `/mobile/v1` gateway and TLS endpoint;
  Pheron App owns the mobile client. The relay must not expose Agentflow's
  loopback desktop API.
- `docs/` holds normative contracts and ADRs, `fixtures/` holds deterministic
  protocol scenarios, and `scripts/` holds their validators.

See [API contracts](api_contracts.md) for the documented boundary.
