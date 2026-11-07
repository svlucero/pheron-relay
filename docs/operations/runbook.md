# Relay operations runbook

## Deploy and TLS

Expose only `wss://<relay-host>/tunnel/v1/{desktop,mobile}` and protect health
and metrics endpoints at the operator boundary. Terminate outer TLS with a
certificate valid for the relay hostname (the process supports TLS 1.3); inner
TLS remains pinned end-to-end between mobile and Agentflow. Never terminate or
log inner TLS at the relay. Pass certificate paths and control-plane credentials
at runtime through the platform secret mechanism, never an image, compose file,
fixture, or environment template.

The bundled binary is development-only and intentionally fails outside
`RELAY_ENVIRONMENT=development`: production requires a real ticket verifier and
an atomic shared store selected by the operator. Do not replace that check with
the memory store or accept-all authenticator.

## Scaling and recovery

Use at least two replicas across failure domains. The routing store must claim,
touch, resolve, and remove routes atomically. It cannot move a live desktop
socket; configure owner-aware routing for the locator or implement tested
inter-replica forwarding before using more than one replica. A lost owner
causes synthetic stream resets and desktop exponential-full-jitter reconnect
(1-second base, 30-second cap) with a new ticket; mobile opens a new stream and
does not pair again. Validate this behavior at every provider or load-balancer
change.

For rollout, remove the replica from new admission, call drain, wait up to 30
seconds for active streams, then terminate. `/healthz` reports process liveness;
`/readyz` becomes 503 while draining. Never use either endpoint as a data
diagnostic.

## Safeguards and observability

Defaults: 8 streams/instance, 1 MiB/frame-path bound, 64 KiB frame payload,
10 desktop attempts/minute/instance, 30 mobile opens/minute/locator, 120 opens
per minute/source, 45-second leases, and a 15-second write timeout. Apply edge
connection, CPU, egress, and DDoS protections as well. Metrics are aggregate
connection/stream/event counts. Logs may record event class and process state,
never payload bytes, locators, addresses, tickets, proofs, tokens, query
strings, or secrets. Alert on routing-store errors, lease loss, reconnect
storms, rate-limit spikes, reset spikes, readiness failure, and drain timeout.

## Capacity validation and rollback

Run `go test -race ./...` for the local concurrent socket/disconnect/isolation
coverage. On 2026-11-07, the race-enabled 16-concurrent-desktop admission test
passed three times in 1.47 seconds total on a 10-CPU, 16-GiB macOS development
host. This verifies only the checked-in admission cap and local isolation; it
is not provider capacity. Record CPU, memory, region, connection duration,
bandwidth, error rate, and p95 handshake/open latency in the operator's load-test
report before raising limits. There is no hosted measurement in this repository.

To roll back: stop new admission, drain the new replicas, restore the previous
immutable image and its compatible routing-store schema, confirm `/readyz`, and
watch reconnect/reset/error aggregates. Do not delete routes manually or reuse
registration tickets. If rollback does not recover readiness, keep the relay
unavailable and escalate rather than routing a stream to a different instance.
