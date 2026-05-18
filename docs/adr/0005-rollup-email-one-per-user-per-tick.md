# Notifications are sent as a rollup email, one per user per tick

When multiple Alert Definitions fire for the same user during a single 60-second evaluation tick, the engine collects them and sends one email listing all alerts rather than one email per alert.

All alert types in this system are sustained-condition or windowed (threshold held for N minutes, N short cycles in M minutes, etc.) — none are point-in-time events where sub-minute latency matters. Conditions that fire together are often correlated: a sunset storm may trigger battery_evening and battery_critical simultaneously. Separate emails for each alert on the same tick would create noise without adding information. The 60-second polling interval already means alerts are not delivered in real time, so batching within a tick costs nothing in practice.

## Considered options

- **One email per alert, sent immediately when the condition is detected** — rejected: correlated alerts fire together and would generate a burst of emails, and "immediately" is already bounded by the 60-second tick interval.
- **Push notifications or WebSocket delivery** — not ruled out for the future, but the rollup email design does not preclude adding a push path later; the engine's fired-alert list is available before the email is sent.
