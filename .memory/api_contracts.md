# API contracts

- No runtime API is implemented in this repository yet.
- The relay contract will define a versioned control handshake for desktop
  channels and opaque data-channel forwarding to Agentflow's mobile gateway.
- Mobile API semantics remain owned by Agentflow's `/mobile/v1/*` contract;
  the relay must never route the privileged desktop `/api/*`, `/ws`, or
  `/api/session-token` surfaces.
- Pheron App's pairing material supplies the opaque instance locator, relay URL,
  and Agentflow TLS SPKI pin needed by the mobile client.

See [architecture](architecture.md) for ownership boundaries.
