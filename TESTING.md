# Testing

## Unit Tests

No dependencies required.

```sh
go test ./...
```

## Integration Tests

Requires Docker. Each package spins up a `timescale/timescaledb:latest-pg16` container, runs migrations, and tears it down on completion. Allow ~45 seconds for container startup across all packages. Run with `-p 1` to prevent packages from racing on Docker port allocation.

```sh
go test -tags integration -timeout 10m -p 1 ./...
```

Run a single package:

```sh
go test -tags integration -timeout 120s ./internal/db/...
go test -tags integration -timeout 120s ./internal/api/...
go test -tags integration -timeout 120s ./internal/alerts/...
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
| `internal/alerts` | `applyOp` (GT/LT/between/unknown), `State` CRUD + concurrent access, `buildMessage` subject/body/headers | All 6 evaluator types (threshold_breach, short_cycle, scheduled_check, differential, count_per_window, cycle_complete) fire/no-fire/no-data cases; `loadAlertDefs` enabled filter; `loadRecipients` mute logic (active/expired); `setMute` GREATEST semantics |
| `internal/api` | `classifyPowerSource`, `autoResolution`, table selection, time parsing, all HTTP validation (400/401) paths, CORS middleware, alert bad JSON + invalid duration + auth guard | Login/logout flow, session cookie attributes, dashboard auth, chart query on empty DB, alert list, preferences CRUD, mute/unmute flow, mute durations, dashboard `active_alerts` field |
| `internal/db` | — | `GetUserByName`, `CreateSession`, `GetSessionUser`, `DeleteSession`, `GetWatermark`, `SetWatermark` |

## Notes

- Integration tests use `//go:build integration` — excluded from `go test ./...` without the tag.
- Tests that require a live DB but aren't integration tests are skipped with `t.Skip`.
- Each integration test truncates only the tables it needs before running; tables are not reset between packages.
- The `ON CONFLICT DO NOTHING` clauses in ingestion code rely on unique indexes added in `migrations/003_unique_indexes.sql`. The idempotency test catches regressions if those indexes are dropped.
- `-p 1` is required for integration tests — parallel package execution races on Docker port binding.
