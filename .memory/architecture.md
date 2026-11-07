# Architecture

- `pheron-relay` contains a provider-neutral Go relay data-plane in `relay/`.
  `Server` exposes only `/tunnel/v1/desktop` and `/tunnel/v1/mobile` WebSocket
  endpoints; there is deliberately no desktop `/api/*` proxy.
- `TicketAuthenticator` is the control-plane boundary for one-time ticket and
  key-proof verification. `Store` is the atomic shared routing/lease boundary;
  `MemoryStore` is for local/test use and must be replaced across replicas.
- The server maps an authenticated desktop route to a mobile stream, applies
  configurable handshake, lease, stream, frame, buffer and write-time limits,
  and copies valid binary frames unchanged. TLS and application bytes are not
  parsed or logged.
- The data plane is an outbound desktop WebSocket channel plus a mobile relay
  transport. It forwards bounded opaque TLS bytes to Agentflow's dedicated
  `/mobile/v1` gateway, where TLS and device authorization terminate.
- The relay owns registration-ticket validation, locator rendezvous, routing
  leases, quotas, and byte forwarding. It never exposes Agentflow desktop
  `/api/*`, `/ws`, or `/api/session-token` surfaces.
- `docs/contracts/relay-tunnel-v1.md` is normative; `docs/adr/` records runtime
  decisions. `fixtures/relay-tunnel-v1/` is validated by standard-library
  Python scripts in `scripts/`; `relay/server_test.go` covers socket behavior.
- Agentflow#422 implements the gateway/tunnel integration; pheron-app#12 owns
  pairing material and the mobile client contract.

See [API contracts](api_contracts.md) and [coding standards](coding_standards.md).
