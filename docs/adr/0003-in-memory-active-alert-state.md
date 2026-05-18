# Active Alert State is in-memory, not persisted

The alert engine evaluates all enabled Alert Definitions every 60 seconds and needs to expose the current set of active alerts to the dashboard API handler. Rather than writing results to a DB table on each tick, the engine maintains an in-memory, mutex-protected `*alerts.State` struct that the API handler reads directly.

Active alert state is fully derived from the underlying time-series data, so it is reconstructed within one evaluation tick (≤ 60 seconds) after a service restart — there is nothing to lose from a crash. Dashboard reads are O(1) pointer dereferences with no DB round-trip. A persisted table would require the engine to write on every tick (adding a failure mode: what happens if the write fails mid-evaluation?), and would give the dashboard only a snapshot that could be stale by the time the handler reads it.

## Considered options

- **Dedicated `active_alerts` table written by the engine** — rejected: write-on-every-tick semantics complicate engine error handling, and the table would be stale for up to 60 seconds anyway. The dashboard already aggregates several DB queries; adding another does not materially help.
- **Query the condition directly in the dashboard handler** — rejected: re-evaluating all conditions on every dashboard request is expensive and duplicates the engine logic.
