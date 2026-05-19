# System Monitoring — Build Plan

## Architecture

**Backend (Go):** `github.com/tylerrencher/systemmonitoring`
- TimescaleDB (hypertables) via pgx/v5
- Ingestion: IoTawatt (HTTP poll), Solar Assistant (MQTT), Ambient Weather (HTTP poll)
- REST + WebSocket API on `:8080`
- Session cookie auth (HttpOnly, SameSite=Strict)
- Binary embeds frontend via `//go:embed dist` in `ui/ui.go`

**Frontend (React):** `ui/` directory
- Vite + React + TypeScript
- shadcn/ui + Tailwind v3
- TanStack Router (code-based routing)
- TanStack Query for REST, custom `useDashboard` hook for WebSocket
- Built to `ui/dist/`, embedded in Go binary for production

---

## Phases

### Phase 1 — Data ingestion ✅
- IoTawatt HTTP poller with exponential backoff retry on startup
- Solar Assistant MQTT subscriber (per-inverter readings + system totals)
- Ambient Weather HTTP poller with metric normalization (`tempf` → `temp_f`)
- TimescaleDB hypertables: `iotawatt_readings`, `solar_readings`, `solar_totals`, `weather_readings`
- Continuous aggregates: 1min / 1hour / 1day for IoTawatt, solar readings, and solar totals
- Ingestion watermarks for gap-fill tracking
- Backfill subcommand for IoTawatt historical data

### Phase 2 — REST API + WebSocket ✅
- `GET /api/v1/dashboard` — live snapshot (solar, battery, generator, consumption, weather, top consumers)
- `GET /api/v1/ws` — WebSocket stream pushing dashboard updates on a ticker
- `GET /api/v1/series` — distinct IoTawatt series (power chart dropdown)
- `GET /api/v1/solar/inverters` — distinct inverter names
- `GET /api/v1/charts/power` — IoTawatt time-series chart, auto-resolution, device optional
- `GET /api/v1/charts/solar` — Solar time-series chart; no inverter → totals, inverter → per-inverter
- `GET /api/v1/charts/weather` — Weather metric time-series
- `POST /api/v1/auth/login` / `POST /api/v1/auth/logout` — session cookie auth
- `useradd` CLI subcommand to create users (admin or viewer role)
- Dashboard consumption: House series kWh accumulated since local midnight

### Phase 3 — React frontend ✅
- Sidebar layout: Dashboard, Power, Solar, Weather; mobile sheet drawer
- Dashboard: power source badge (green/yellow/red/orange), stat cards, top consumers bar chart, weather snapshot, WebSocket live updates
- Power page: series dropdown, time range picker, Recharts line chart
- Solar page: metric + inverter dropdowns, time range picker; All → totals, specific inverter → per-inverter
- Weather page: temperature + humidity charts stacked
- ChartTimeRange: presets (1h / 6h / 24h / 7d / 30d / 90d) + custom calendar date range picker
- Theme: system / light / dark cycle, persisted in localStorage
- Auth: optional login (app is open by default), user menu shows name + role + logout

### Phase 4 — Alert engine ✅
- Alert definitions in DB (`alerts` table, 15 seeded rules via migration 005)
- `internal/alerts/` package: `Engine` (60s tick), `State` (in-memory RWMutex), `evaluate.go`, `email.go`
- SMTP delivery via Gmail AUTH LOGIN (Outlook blocked basic auth — see ADR 0002)
- Per-user subscriptions (`alert_preferences`) and mutes (`alert_mutes`) with snooze durations
- Active alerts surfaced in WebSocket dashboard payload; banner filtered to user's subscribed alerts
- `/alerts` management page: subscribe toggles + inline mute controls
- Alert nav item shown when logged in; `Cache-Control: no-store` on `index.html` fixes mobile caching

### Phase 5 — Production deployment ⬜
- Systemd service unit for the Go binary
- Reverse proxy (nginx or Caddy) for TLS termination
- Automated certificate renewal
- Docker Compose or bare-metal deployment notes
- Log rotation and monitoring

---

## Key constraints

- Tailwind must stay at v3 — v4 moved PostCSS plugin, breaks shadcn
- react-day-picker must stay at v9 — v10 changed ClassNames API, breaks shadcn calendar
- TimescaleDB unique indexes must include `time` (partitioning column)
- `ON CONFLICT DO NOTHING` is a no-op without those unique indexes
- Solar battery metrics (`battery_power`, `battery_state_of_charge`) only exist in `solar_totals`, not `solar_readings`
