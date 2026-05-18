# Alert definitions in a single table with JSONB params

Each of the six alert types (threshold_breach, short_cycle, scheduled_check, differential, count_per_window, cycle_complete) has a distinct parameter shape. Rather than one table per type or an EAV params table, all alert definitions live in a single `alerts` table with a `params JSONB` column for type-specific fields and top-level columns for shared attributes (`type`, `data_source`, `cooldown`, `enabled`).

This keeps all alert definitions in one place, avoids schema migrations when adding new alert types, and makes the alert engine's startup validation straightforward — each type validates its own expected keys in `params` at load time. The trade-off is that `params` structure is not enforced by the DB; correctness is the engine's responsibility.
