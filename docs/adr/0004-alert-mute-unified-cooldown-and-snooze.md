# Alert Mute serves as both system cooldown and user snooze

After an Alert Definition fires, two suppression needs arise simultaneously: the engine must not re-fire that alert for a cooldown period (e.g., 5 minutes for HVAC short-cycle, 24 hours for battery_evening), and the user may want to suppress it for longer (e.g., until tomorrow morning). Both are implemented by writing to the same `alert_mutes (user_id, alert_key, muted_until)` row. On conflict, the engine uses `GREATEST(EXCLUDED.muted_until, alert_mutes.muted_until)` so the longer suppression always wins.

This means a user-set snooze can never be silently shortened by a system cooldown write — the engine can fire, set its 5-minute cooldown, and if the user had already snoozed until 8am the next day, nothing changes. It also means the suppression check in the engine and the API is a single query (`muted_until > NOW()`), with no precedence rule to reason about.

## Considered options

- **Separate `alert_cooldowns` and `alert_snoozes` tables** — rejected: the engine would need to check both tables on every evaluation, the API would need to manage two records, and a precedence rule ("which one wins?") would need to be documented and enforced everywhere.
- **Cooldown as an in-memory timer** — rejected: a service restart would lose all cooldown state, potentially re-firing high-frequency alerts (e.g., battery_critical at 5-minute cooldown) immediately after restart.
