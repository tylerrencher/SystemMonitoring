# TimescaleDB as the time-series storage engine

All sensor readings (IoTawatt, Solar Assistant, Ambient Weather) are stored in TimescaleDB, a PostgreSQL extension that adds hypertable partitioning, automatic chunk management, and continuous aggregates. The data model is a narrow time-value schema (`time, device/inverter/metric, value`) rather than a wide per-sensor column schema.

Migrating to TimescaleDB costs nothing — it is a PostgreSQL extension, so the existing `pgx` driver and plain SQL work unchanged. What it buys: automatic time-based chunk partitioning (queries only scan relevant chunks), `time_bucket()` for aggregate grouping, and continuous aggregates (automated materialized views that refresh incrementally). The alternative — plain PostgreSQL with `PARTITION BY RANGE (time)` — requires manual partition creation and provides no equivalent to continuous aggregates; maintaining rolling averages would move to the application layer. A purpose-built TSDB (InfluxDB, Prometheus, VictoriaMetrics) would have required a second persistence layer alongside PostgreSQL for users, sessions, and alerts, or a translation layer for relational joins (e.g., joining alert_mutes to sensor readings in one query).

## Considered options

- **Plain PostgreSQL with range partitioning** — rejected: no continuous aggregates, manual partition management, application-level rollup code required.
- **InfluxDB** — rejected: requires a separate service; no relational model, making auth/session/alert tables awkward; line protocol is a separate client.
- **Prometheus + long-term storage** — rejected: pull-only model does not match push ingestion from IoTawatt and Solar Assistant; no relational joins.
