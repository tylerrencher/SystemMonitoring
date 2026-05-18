# Home Energy Monitor

A home energy monitoring system for an off-grid property with solar panels, battery storage, a generator, and IoTawatt power monitors.

## Language

### Alert Domain

**Alert Definition**:
A named, typed rule stored in the `alerts` table that describes a condition to monitor, the data source to query, detection parameters, and a cooldown period.
_Avoid_: alert config, alert rule, monitor

**Alert Type**:
The detection algorithm an Alert Definition uses. One of: `threshold_breach`, `short_cycle`, `scheduled_check`, `differential`, `count_per_window`, `cycle_complete`.
_Avoid_: alert kind, alert category

**Alert Key**:
The unique text identifier for an Alert Definition (e.g., `short_cycle:UpstrHVAC`). Used as the primary reference in `alert_preferences` and `alert_mutes`.
_Avoid_: alert ID (numeric ID exists but is not the canonical reference)

**Alert Preference**:
A per-user opt-in to an Alert Definition, stored in `alert_preferences`. A user only receives notifications for alerts they have subscribed to.
_Avoid_: alert subscription, alert setting

**Alert Mute**:
A per-user suppression record in `alert_mutes` with a `muted_until` timestamp. Used for both system-set cooldown (set by the alert engine after firing) and user-set snooze (set by the user to suppress longer). The longer of the two always wins.
_Avoid_: cooldown, snooze (use "mute" as the unified term)

**Cooldown**:
The minimum interval between two firings of the same Alert Definition for the same user. Stored as an `INTERVAL` column on the `alerts` table. Implemented by setting an Alert Mute after each firing.
_Avoid_: suppression window, debounce

**Active Alert**:
An Alert Definition whose condition is currently true. Distinct from a notification — an Active Alert drives the UI banner. Clears automatically when the condition resolves.
_Avoid_: triggered alert, open alert

**Firing**:
The act of sending a notification and setting an Alert Mute. An Alert Definition fires when its condition becomes true and no Alert Mute is in effect for that user.
_Avoid_: triggering, sending

### Alert Types

**threshold_breach**:
Fires when a series value is above, below, or between thresholds. Supports an optional `sustained_min` duration the condition must hold before firing.

**short_cycle**:
Fires when a series cycles on/off (relative to `on_threshold_w`) N or more times, each run lasting less than `max_cycle_min` minutes, within a rolling `window_min` window.

**scheduled_check**:
Fires once per day at a fixed `check_time` (HH:MM) if a condition is true at that moment. Cooldown is typically 24 hours.

**differential**:
Fires when the difference between two series (`series_a` minus `series_b`, absolute value) exceeds `max_diff_w` for at least `sustained_min` minutes.

**count_per_window**:
Fires when a series starts a new session (crosses above `on_threshold_w` after being below for at least `min_gap_min` minutes) more than `max_count` times within a rolling `window_hours` window.

**cycle_complete**:
Fires when a series that was running (above `on_threshold_w`) for at least `min_run_min` minutes drops below `on_threshold_w` and stays there for at least `off_confirm_min` minutes. `off_confirm_min` also defines the minimum gap used to distinguish within-cycle noise from a true session end.

### Alert Engine

**Alert Engine**:
The background goroutine in `internal/alerts/` that evaluates all enabled Alert Definitions every 60 seconds, updates Active Alert State, and fires email notifications.
_Avoid_: alert poller, alert worker, alert service

**Active Alert State**:
An in-memory, mutex-protected structure shared between the Alert Engine and the API server. Holds the current set of active alert keys and names. Updated by the engine each tick; read by the dashboard handler to populate `active_alerts` in `DashboardResponse`.
_Avoid_: alert cache, alert store

**Rollup**:
A single email sent to a user at the end of an evaluation tick containing all alerts that fired for that user during that tick. One rollup per user per tick — never one email per alert.
_Avoid_: alert digest, batch notification

**XOAUTH2**:
The SASL mechanism used to authenticate SMTP sessions with an OAuth2 access token. The engine exchanges a stored refresh token for a short-lived access token via `golang.org/x/oauth2` before each send, then presents it as `AUTH XOAUTH2`.
_Avoid_: OAuth SMTP, token auth

### Data & Devices

**Series**:
An IoTawatt circuit label (e.g., `UpstrHVAC`, `WellPump`, `House`). Maps to the `series` column in `iotawatt_readings` and `iotawatt_1min`.

**Metric**:
A named measurement from the solar system (e.g., `battery_state_of_charge`, `pv_power`). Maps to the `metric` column in `solar_totals` or `solar_readings`.

**Data Source**:
Which table an Alert Definition queries. One of: `iotawatt`, `solar_totals`, `solar_readings`.

**Session** (in the context of `count_per_window` and `cycle_complete`):
A contiguous period where a series is above its `on_threshold_w`, with within-session gaps shorter than `min_gap_min` (or `off_confirm_min`) treated as noise and ignored.

## Relationships

- An **Alert Definition** has exactly one **Alert Type** and one **Data Source**
- A **User** opts in to zero or more **Alert Definitions** via **Alert Preferences**
- When an **Alert Definition** **fires** for a user, an **Alert Mute** is set for that user for the **Cooldown** duration
- An **Active Alert** is derived from evaluating **Alert Definitions** — it is not stored, it is computed
- An **Alert Mute** suppresses **Firing** but does not suppress the **Active Alert** banner

## Example dialogue

> **Dev:** "The battery evening alert fired but the banner is still showing — is that a bug?"
> **Domain expert:** "No — the banner reflects the Active Alert (condition still true), not the Firing. The Alert Mute suppressed the next email, but the SoC is still below 33%, so the banner stays until it recovers."

> **Dev:** "The dryer short-cycled twice and fired. Should we reset the Mute?"
> **Domain expert:** "That's not a short cycle — the dryer uses `cycle_complete`. Short cycle is for the heat pumps and the well pump dry detection."

## Flagged ambiguities

- "cooldown" was used to mean both the system-enforced quiet period and a user snooze — resolved: both are implemented as an **Alert Mute**; the **Cooldown** column defines the minimum system-set duration.
- "alert key" vs "alert ID" — resolved: **Alert Key** (text) is the canonical reference; numeric `id` is internal only.
