# Relay tunnel protocol v1

Status: normative design contract. This contract is implemented by the relay
and the Agentflow integration tracked in [agentflow#422]. It carries the mobile
gateway described by [pheron-app#12] without changing that API.

## Scope and trust boundary

The relay supplies rendezvous and bounded byte forwarding. A desktop Agentflow
instance makes an outbound channel to the relay; a native Pheron App opens a
separate relay transport and the relay joins it to that channel. The byte stream
inside the transport is TLS between the mobile client and Agentflow's dedicated
`/mobile/v1` gateway. The relay does not terminate, inspect, transform, retry,
or log that TLS stream.

The relay is **not** a proxy to desktop `/api/*`, `/ws`, or
`/api/session-token`. Agentflow accepts the stream only at its mobile gateway,
which applies device credentials, DPoP, scopes, tickets, and revocation from
the mobile contract. The privileged per-process Agentflow token is neither a
relay credential nor an inner-stream value.

Pairing material from pheron-app#12 pins the Agentflow gateway SPKI and names
the relay URL plus opaque locator. The outer relay connection uses normal Web
PKI. The mobile client validates the *inner* Agentflow certificate/SPKI before
sending any mobile credentials. A relay compromise can observe locator, timing,
and byte counts or deny service, but cannot read or alter authenticated mobile
traffic.

## Names and registration

`instance_id` is a stable UUID for an Agentflow installation. `locator` is an
opaque, high-entropy, case-insensitive rendezvous handle advertised only in
pairing material; it has no authorization meaning. `channel_id` is a fresh UUID
for each desktop connection. A locator is never reused for another instance.

Before opening a channel, the desktop obtains a one-time opaque
`registration_ticket` from the relay control plane using its Pheron-account
provisioning flow. The ticket is bound to `instance_id`, `locator`, the desktop
Ed25519 public-key thumbprint, and an expiry no longer than 10 minutes. It is
not an Agentflow process token, mobile credential, or pairing secret.

The desktop opens `wss://{relay-host}/tunnel/v1/desktop` and sends, within five
seconds, the UTF-8 control message below (maximum 4096 bytes):

```json
{
  "type": "desktop_hello",
  "version": 1,
  "instance_id": "<uuid>",
  "locator": "<opaque locator>",
  "channel_id": "<uuid>",
  "registration_ticket": "<opaque>",
  "key_proof": "<base64url Ed25519 signature over canonical hello without key_proof>"
}
```

The relay atomically consumes the ticket, verifies the key proof and binding,
then returns `desktop_ready` with `lease_seconds: 45` and a server-generated
`connection_id`. It records only a routing entry keyed by a keyed hash of the
locator. A second live channel for the same instance or locator is rejected
with `duplicate_channel`; it never silently replaces the working channel.
Renewal needs a newly issued ticket and key proof after the channel ends.

## Mobile rendezvous and tunnel

The native client opens `wss://{relay-host}/tunnel/v1/mobile`, sends
`mobile_open` within five seconds (maximum 1024 bytes), and receives only
transport status:

```json
{"type":"mobile_open","version":1,"locator":"<opaque locator>","stream_id":17}
```

The relay checks the active routing entry, reserves `stream_id` on that desktop
channel, and sends `stream_open` to the desktop. The desktop may accept only
after it has connected that stream to Agentflow's explicit local mobile-gateway
listener; it sends `stream_accept` or `stream_reject`. The relay answers
`stream_ready` only after acceptance. The client then writes an ordinary TLS
ClientHello as opaque `DATA` bytes. Its SNI/certificate validation use the
pinned gateway endpoint supplied while pairing; the desktop accepts no other
local destination. Opening, acceptance, or transport failure has a 10-second
deadline.

The relay does not authenticate the mobile at this outer layer. Its only
authorization decision is whether the locator has a live desktop channel.
Agentflow performs device authentication inside the pinned TLS stream. This
keeps DPoP proofs, enrollment material, mobile WebSocket tickets, and request
payloads opaque to the relay.

## Frames and state machine

Control messages are JSON text frames, maximum 4096 bytes. After
`stream_ready`, application bytes use binary frames with this fixed header:

| Bytes | Field | Rule |
|---:|---|---|
| 1 | version | must be `1` |
| 1 | type | `DATA=1`, `FIN=2`, `RST=3`, `PING=4`, `PONG=5` |
| 2 | reserved | zero; otherwise reject |
| 4 | stream_id | non-zero unsigned big-endian |
| 4 | payload_length | unsigned big-endian, at most 65,536 |

`DATA` contains exactly `payload_length` opaque bytes. `FIN`, `RST`, `PING`,
and `PONG` have an empty payload. Frames may not arrive before `stream_ready`,
after `FIN`/`RST`, or for another party's stream. The relay copies valid
`DATA` bytes unchanged and never coalesces data across stream IDs. It forwards
`FIN`; `RST` closes the matching stream immediately. Either outer WebSocket
closing ends all of its streams with a synthetic `RST`.

State is `desktop_connecting → ready → draining → closed` and, per stream,
`opening → open → half_closed|reset → closed`. There is no resumption of an
inner TLS stream: after any disconnect the mobile client follows the mobile
contract, creates a fresh transport and reconciles state after reconnect.

## Liveness, expiry, and recovery

- Each ready desktop sends `PING` every 15 seconds and expects `PONG` within 45
  seconds. Any valid traffic refreshes the same lease.
- Missing lease, invalid frame, key/ticket expiry, or outer close removes the
  routing entry immediately, resets active streams, and emits no payload logs.
- The desktop reconnects with a new channel ID and registration ticket using
  exponential full jitter (base 1 second, cap 30 seconds). It must not create a
  parallel replacement while a channel remains ready.
- A mobile `mobile_open` for an existing but offline locator gets
  `instance_offline`; an unknown, expired, or retired locator gets
  `unknown_locator`. Both are externally generic and rate-limited.
- The relay returns `relay_unavailable` only for its own admission/outage
  condition. It does not turn a failed inner TLS, DPoP, or mobile request into a
  relay error.

## Isolation, limits, and abuse controls

Routing state and quotas are partitioned by `instance_id`, locator hash, and
authenticated desktop account. A stream can join only the exact active channel
for that locator; data has no cross-instance broadcast path.

| Limit | v1 value | Enforcement |
|---|---:|---|
| desktop channels | 1 per instance | atomic routing claim |
| concurrent mobile streams | 8 per instance | reject new with `stream_limit` |
| buffered bytes | 1 MiB per stream; 8 MiB per instance | pause reads, then reset after 15 s |
| frame payload | 64 KiB | close malformed peer |
| desktop/mobile hello | 4 KiB / 1 KiB | close malformed peer |
| opens | 30/minute per locator and 120/minute per source IP | `rate_limited` |
| new desktop attempts | 10/minute per instance and 60/minute per source IP | `rate_limited` |
| bandwidth | 10 MiB/s per instance, token bucket | backpressure then `rate_limited` |

Admission limits apply before allocating stream buffers. Backpressure is
bidirectional: when an outbound queue reaches its cap the relay stops reading
the opposite peer; if it cannot drain within 15 seconds it sends `RST` and
counts the event. Enforce connection, frame, handshake, CPU, and egress quotas
at the edge as well as in the relay. Rate-limit keys use privacy-preserving
hashed identifiers; operators may retain aggregate counters, connection IDs,
error codes, sizes, and timings for seven days, but never payload bytes,
locators, tickets, proofs, tokens, query strings, or secrets.

## Stable transport errors

Errors are text control messages, never inner TLS responses:

```json
{"type":"error","version":1,"code":"unknown_locator","retryable":true,"retry_after_seconds":2}
```

`message` is intentionally omitted and not contractual. Codes are
`invalid_handshake`, `unsupported_version`, `registration_invalid`,
`duplicate_channel`, `unknown_locator`, `instance_offline`,
`stream_rejected`, `stream_limit`, `rate_limited`,
`protocol_violation`, and `relay_unavailable`. `retry_after_seconds` is
required only for `rate_limited`. The relay closes with WebSocket
policy/protocol codes after an error; no code is used to reveal mobile identity
or Agentflow authorization state.

## Availability and operations

Run at least two relay replicas across failure domains. Replicas require a
strongly consistent or atomic shared routing/lease store and distributed rate
limits; a node-local map alone is insufficient. Drain deployments by refusing
new opens, giving ready desktops a reconnect signal, and allowing existing
streams up to 30 seconds before reset. Health checks distinguish control-plane
availability, routing-store availability, and data-plane admission. Alert on
lease loss, reconnect storms, quota resets, and routing-store errors without
capturing sensitive content.

The integration dependency is [agentflow#422]; the client pairing and mobile
API dependency is [pheron-app#12].
