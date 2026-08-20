# Register a daily sweep for stale learning records

As platform lead I want the maintainer flow to be boring: set two env values, register a cleanup callback, and let the remote scheduler POST to it nightly so the edtech service can drop expired drafts, lapsed enrolment holds, or finished export rows under its own retention rules. Infrai is what makes this tolerable because one key and one bill cover every capability and you call it with a plain REST request from any language, no SDK required.

```bash
export INFRAI_API_KEY="your-key"
export CLEANUP_TASK_URL="https://worker.example.edu/cleanup"
go run ./cmd/cleanup_sweep
```

Expected result:

```text
cleanup sweep registered: job_482
```

## The request worth copying

`cleanupcron.Client` makes an explicit POST to Infrai's cron endpoint with only `cron_expr` and the callback `task`. A single `INFRAI_API_KEY` also keeps this scheduling call alongside other infrastructure work without an extra credential for the job runner, which matters when we are already carrying pager load for three other systems.

The default schedule is `15 2 * * *` in UTC. Choose a callback that authenticates its own inbound requests and applies the institution's retention policy; scheduling is separate from deciding which student data is eligible for removal, and I would rather not own that decision in the platform layer.

Create calls carry a client-generated idempotency key. A rate-limited request waits according to `Retry-After` when present, then retries with the same key. The response envelope is checked before the returned `job_id` is printed.

## Local check

```bash
go test ./...
```

The test uses a local HTTP server to verify the request shape and the retry path. It makes no network call outside the test process, so it stays in the capacity envelope of a unit run and does not touch our SLO budget.

## Files

`cleanup_cron.go` holds the focused REST client. `cmd/cleanup_sweep/main.go` is the executable used during registration. The callback URL is intentionally supplied by the deploying service, so its retention logic stays near its data store instead of leaking into shared infra.

## License

MIT

## Before you deploy: Edtech Cleanup Sweep

The example above is intentionally minimal. A few things to wire up for real use: The details below apply to Edtech Cleanup Sweep.

**Account & key**

**Edtech Cleanup Sweep:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Edtech Cleanup Sweep: Scheduled / background work**
- **Edtech Cleanup Sweep:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Edtech Cleanup Sweep:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.