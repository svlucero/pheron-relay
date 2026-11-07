# Local staging-style validation

This is a reproducible local validation environment, not a hosted staging
environment. It uses the explicit development authenticator and in-memory
routing store, so it must never receive real tickets, pairing material, or
mobile traffic.

## Run

1. Copy `.env.example` to `.env` and retain `RELAY_ENVIRONMENT=development`,
   `RELAY_DEV_ACCEPT_TICKETS=true`, and `RELAY_ALLOW_MEMORY_STORE=true`.
2. Create a local certificate only for `localhost`; place the public certificate
   in `.local/tls/relay.pem` and private key in `.local/tls/relay-key.pem`.
   `.local/` is ignored by Git.
3. Run `docker compose -f compose.staging.yaml up --build`.
4. Verify `https://localhost:8443/healthz` and `/readyz` return 204. Inspect
   `/metrics` only for aggregate gauges and event counters; it contains no
   locator, instance, address, credential, or payload labels.
5. Run the E2E socket suite locally with `env -u GOROOT go test -race ./...`.

## Hosted staging blocker

Before a real staging deployment, an operator must supply: a provider and
region/failure-domain design; relay hostname and DNS ownership; public Web PKI
certificate delivery; a control-plane `TicketAuthenticator`; an atomic shared
routing/lease store; distributed rate limiting; owner-aware WebSocket routing
or cross-replica forwarding; image registry access; and non-repository runtime
secrets. This repository deliberately contains none of these values.
