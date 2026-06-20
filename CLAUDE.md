# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Structure

This is a **multi-module Go monorepo** where each library under `libs/` has its own `go.mod` and is independently versioned. There is no shared build at the root — commands must be run from each library's directory.

```
libs/
  rate-limiter/      # Redis-backed rate limiter (module: github.com/Aibier/go-common-libs/libs/rate-limiter)
  idempotency/       # DB-backed idempotency keys (module: idempotency)
  metrics/           # Metrics abstraction layer (module: github.com/Aibier/go-common-libs/libs/metrics)
  middleware/        # Gin HTTP middleware (module: within libs/middleware/go.mod)
  httpRequester/     # HTTP client wrapper (no separate go.mod — part of root module)
  example/           # Usage examples
```

## Commands

All commands must be run from the specific library directory, not the repo root.

```bash
# Run all tests in a library
cd libs/rate-limiter && go test ./...

# Run tests with race detector and coverage (matches CI)
cd libs/rate-limiter && go test -v -race -cover './...'

# Run a single test
cd libs/idempotency && go test -run TestFunctionName ./...

# Run benchmarks
cd libs/idempotency && go test -bench=. ./...
```

**CI requires Redis at `localhost:6379`** for rate-limiter tests. Set `REDIS_ADDR` env var if non-default:
```bash
REDIS_ADDR=localhost:6379 go test ./...
```

Idempotency tests use MySQL and Postgres — see `postgres_store_test.go` and `mysql_store_test.go` for connection setup.

## Architecture

### Rate Limiter (`libs/rate-limiter`)
`Factory` (in `rate_limiter.go`) creates and caches `RateLimiter` instances keyed by string identifier. Three algorithm implementations live in sub-packages:
- `configurablewindow/` — sliding window using Redis sorted sets
- `tokenbucket_lua/` — token bucket via Lua script (atomic)
- `tokenbucket_multi_exec/` — token bucket via Redis multi-exec pipeline

Configuration is versioned: configs are stored in Redis under `config_<key>` as `VersionedConfig` JSON. `GetRateLimiter` re-creates the limiter when the config version changes.

### Idempotency (`libs/idempotency`)
`IdempotencyService` wraps a `Store` (MySQL or Postgres). `SetKey` returns a status string (`SUCCEEDED`, `DUPLICATE`, `FAILED`). The store uses DB unique constraints as the concurrency primitive — `UniqueConstraintError` signals a duplicate. Keys are namespaced (`namespace + key`) and optionally expire. ULIDs (`ulid.go`) are used for generating unique IDs.

### Metrics (`libs/metrics`)
`NewMetricManager(typ)` is a factory returning a `MetricsManager` interface (from `model/model.go`). Backends: `ddstatsd` (Datadog StatsD) and `dummy` (no-op for testing). Consumers only depend on the `model` package interfaces.

### Middleware (`libs/middleware`)
Single Gin middleware: `RequestLogger` attaches a UUID to each request via `c.Set("unique_request_uuid", ...)`, logs request/response bodies (configurable), and logs duration. Depends on `go.uber.org/zap`.

### HTTP Requester (`libs/httpRequester`)
Stateless `HttpRequest(ctx, RequestConfig)` function with built-in retry loop, backoff, JSON marshaling/unmarshaling, form-data support, and optional response decoding into a target struct via `DecodeTarget`.
