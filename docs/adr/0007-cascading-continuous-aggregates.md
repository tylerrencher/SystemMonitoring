# Cascading continuous aggregates for chart downsampling

Chart queries select from different pre-aggregated views depending on the requested time span: raw `iotawatt_readings` for spans under 2 hours, `iotawatt_1min` for under 7 days, `iotawatt_1hour` for under 90 days, `iotawatt_1day` beyond that. The 1-hour aggregate is built from the 1-minute aggregate (not from raw), and the 1-day aggregate is built from 1-hour. The same three-tier chain exists for solar data.

The cascading build (1min → 1hour → 1day rather than all three from raw) is deliberate: TimescaleDB refreshes each tier incrementally, and computing hourly averages over minute averages is cheaper than re-scanning raw 10-second readings. The client-side `autoResolution` function picks the right table automatically based on the from/to span, so the frontend never needs to specify a resolution. This degrades gracefully: if an aggregate has not yet materialized for recent data, the API returns what is available rather than an error, because the chart queries include a time range that the aggregate policy `end_offset` may not have covered yet.

The breakpoints (2h, 7d, 90d) were chosen so that the point density visible in the UI stays roughly constant: at most a few thousand data points per chart regardless of zoom level.

## Considered options

- **Always query raw data, downsample in the API** — rejected: scanning years of 10-second readings on every chart request is prohibitively slow and shifts aggregation compute to the application.
- **Single aggregate tier (1-minute only)** — rejected: 7-day chart at 1-minute resolution returns ~10,000 points per series, which is fine, but 90-day and year-long views would be impractical.
- **Explicit resolution parameter on chart API** — considered but left out: the caller rarely has reason to override the automatic selection, and exposing it would require documenting valid values per data source.
