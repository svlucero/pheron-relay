# Architecture

- `pheron-relay` is a documentation-first contract repository; no relay runtime
  is implemented here yet.
- The data plane is an outbound desktop WebSocket channel plus a mobile relay
  transport. It forwards bounded opaque TLS bytes to Agentflow's dedicated
  `/mobile/v1` gateway, where TLS and device authorization terminate.
- The relay owns registration-ticket validation, locator rendezvous, routing
  leases, quotas, and byte forwarding. It never exposes Agentflow desktop
  `/api/*`, `/ws`, or `/api/session-token` surfaces.
- `docs/contracts/relay-tunnel-v1.md` is normative; `docs/adr/` records runtime
  decisions. `fixtures/relay-tunnel-v1/` is validated by standard-library
  Python scripts in `scripts/`.
- Agentflow#422 implements the gateway/tunnel integration; pheron-app#12 owns
  pairing material and the mobile client contract.

See [API contracts](api_contracts.md) and [coding standards](coding_standards.md).
