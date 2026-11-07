# API contracts

- [Relay tunnel v1](../docs/contracts/relay-tunnel-v1.md) defines the desktop
  `desktop_hello` registration handshake and mobile `mobile_open` rendezvous.
- The outer relay transport uses JSON control frames and versioned binary frames
  (`DATA`, `FIN`, `RST`, `PING`, `PONG`); `DATA` is opaque TLS bytes.
- Locators route only to a live registered desktop channel. Duplicate channels,
  unknown locators, expiry, limits, and disconnects return stable transport
  errors or reset streams as specified by the contract.
- Agentflow owns the inner `/mobile/v1/*` REST and WebSocket API, TLS endpoint,
  DPoP/device authorization, and all payload semantics. The relay cannot route
  desktop `/api/*`, `/ws`, or `/api/session-token`.
- Pheron App pairing material supplies the relay URL, locator, and pinned
  Agentflow gateway SPKI. See pheron-app#12 and agentflow#422.

See [architecture](architecture.md) for component ownership.
