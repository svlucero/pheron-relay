# ADR 0001: provider-neutral relay with durable routing requirements

- Status: accepted
- Date: 2026-11-07
- Relates to: [relay tunnel v1](../contracts/relay-tunnel-v1.md),
  [pheron-app#12](https://github.com/svlucero/pheron-app/pull/12), and
  [agentflow#422](https://github.com/svlucero/agentflow/issues/422)

## Context

The relay holds outbound desktop and mobile WebSocket connections, forwards
opaque TLS bytes, and needs a shared live locator-to-channel mapping across
replicas. It must drain safely and sustain long-lived channels with predictable
backpressure. A runtime/provider has not been selected.

## Decision

Keep the protocol and control-plane interfaces provider-neutral. Production
data-plane candidates must provide: WebSockets for the required connection
lifetime, graceful drain, bounded binary buffering, and atomically shared
routing/lease and rate-limit state. Select a long-running service or a managed
WebSocket platform meeting those requirements after a load test and cost model;
do not infer a provider decision from this ADR.

Vercel Functions are eligible only after an environment-specific proof against
those requirements. Vercel now supports WebSockets, but a connection remains
pinned only for the function's maximum duration and later connections are not
guaranteed to reach the same function; Vercel recommends external durable state
for cross-connection state. Function execution duration remains plan/runtime
bounded, including a documented 300-second Edge streaming limit and bounded
Node durations. This makes reconnect handling and shared state mandatory and
means it is not automatically equivalent to a durable relay service.

| Concern | Vercel Functions/WebSockets | Long-running service |
|---|---|---|
| Socket lifetime | bounded by configured/function plan duration; reconnect required | controlled by service policy and load balancer |
| Routing state | external atomic store required across invocations | still external for HA; local cache can be an optimization |
| Deploy/drain | invocation termination/lifetime handling must be proven | explicit connection drain and rollout control |
| Cost | provider execution/connection plan and duration dependent | compute, egress, and operations are directly provisioned |
| Operations | reduced host management, provider constraints | more operational responsibility, greater lifecycle control |

The evidence above is from Vercel's current WebSocket knowledge base and
[function limits documentation](https://vercel.com/docs/functions/limitations)
(checked 2026-11-07). Re-evaluate it before committing a provider.

## Consequences

- The relay implementation must use a pluggable routing/lease store and not
  depend on connection affinity.
- A routing store alone cannot move a live WebSocket between processes. Until a
  cross-replica stream-forwarding transport is implemented, the edge must send
  desktop and matching mobile connections to the active route owner (or return
  the documented retryable unavailable outcome). A deployment must prove this
  ownership routing during scale and failover tests; it must never fall back to
  another instance's channel.
- A chosen serverless deployment must force planned reconnect before its maximum
  lifetime and demonstrate no routing split-brain during scale/deploy.
- A long-running service must budget health checks, autoscaling, patching, and
  multi-zone recovery.
- No TLS termination decision changes: inner TLS remains end-to-end to
  Agentflow regardless of provider.

## Alternatives

- **Vercel-only now:** rejected because verified lifetime and cross-instance
  routing constraints need workload and plan-specific validation.
- **Single VM with in-memory routing:** rejected because a restart partitions
  routes and is not highly available.
- **Terminate gateway TLS at the relay:** rejected because it exposes mobile
  authentication and payloads to a service outside Agentflow's trust boundary.
