# Testing

## Unit Tests

No dependencies required.

```sh
go test ./...
```

## Integration Tests

Requires Docker. Each package spins up a `timescale/timescaledb:latest-pg16` container, runs migrations, and tears it down on completion. Allow ~45 seconds for container startup across all packages.

```sh
go test -tags integration -timeout 300s ./...
```

Run a single package:

```sh
go test -tags integration -timeout 120s ./internal/db/...
go test -tags integration -timeout 120s ./internal/api/...
go test -tags integration -timeout 120s ./internal/ingestion/iotawatt/...
```

Run a specific test:

```sh
go test -tags integration -timeout 120s -run TestLoginSuccess ./internal/api/...
```

## Coverage

| Package | Unit | Integration |
|---|---|---|
| `internal/config` | `parseDevices`, `parseFloat`, `parseDuration` | — |
| `internal/ingestion/solar` | `Classify` — all 3 classes, 25 cases | — |
| `internal/ingestion/iotawatt` | `GetSeries`, `Query` parsing, empty + malformed rows | `Backfill` inserts, idempotency, watts/volts routing |
| `internal/ingestion/weather` | `camelToSnake`, `extractTime` | — |
| `internal/api` | `classifyPowerSource`, `autoResolution`, table selection, time parsing, all HTTP validation (400/401) paths, CORS middleware | Login/logout flow, session cookie attributes, dashboard auth, chart query on empty DB |
| `internal/db` | — | `GetUserByName`, `CreateSession`, `GetSessionUser`, `DeleteSession`, `GetWatermark`, `SetWatermark` |

## Notes

- Integration tests use `//go:build integration` — they are excluded from `go test ./...` without the tag.
- Tests that require a live DB but aren't integration tests are skipped with `t.Skip`.
- Each integration test truncates only the tables it needs before running; tables are not reset between packages.
- The `ON CONFLICT DO NOTHING` clauses in ingestion code rely on unique indexes added in `migrations/003_unique_indexes.sql`. The idempotency test catches regressions if those indexes are dropped.
