# System Monitoring

Home energy monitoring system. Ingests data from IoTawatt power meters, a Solar Assistant MQTT broker, and an Ambient Weather station. Exposes a REST/WebSocket API and serves a React dashboard from a single Go binary.

## Prerequisites

- Go 1.25+
- Node.js 20+ (UI builds only)
- Docker (database)

## Quick start

### 1. Start the database

```sh
docker compose up -d
```

TimescaleDB runs on `localhost:5432`. Credentials default to `monitor/monitor`.

### 2. Configure

Copy `.env.example` to `.env` (or export variables directly). The binary reads `.env` automatically at startup.

```sh
# .env
DB_URL=postgres://monitor:monitor@localhost:5432/systemmonitoring

# IoTawatt devices — name=host pairs, comma-separated
IOTAWATT_DEVICES=pwrmone1=pwrmone1.local,pwrmona1=pwrmona1.local,pwrmona2=pwrmona2.local
IOTAWATT_POLL_INTERVAL=10s

# Solar Assistant MQTT broker
SOLAR_MQTT_HOST=192.168.1.56
SOLAR_MQTT_PORT=1883

# Ambient Weather (optional — service starts without these)
AMBIENT_API_KEY=
AMBIENT_APP_KEY=
AMBIENT_MAC_ADDRESS=
AMBIENT_POLL_INTERVAL=5m

# API
API_LISTEN=:8080
DEV_CORS_ORIGIN=           # set to http://localhost:5173 during UI dev

# Dashboard thresholds
BATTERY_CAPACITY_KWH=135
SOLAR_ACTIVE_THRESHOLD_W=50
GENERATOR_ACTIVE_THRESHOLD_W=100
```

### 3. Run migrations

```sh
go run ./cmd/monitor migrate
```

### 4. Build and embed the UI

```sh
cd ui
npm install
npm run build
cd ..
```

The build output (`ui/dist`) is embedded into the Go binary at compile time.

### 5. Start the service

```sh
go run ./cmd/monitor serve
```

The dashboard is available at `http://localhost:8080`.

---

## CLI reference

```
monitor serve                        Start ingestion + API server
monitor migrate                      Run SQL migrations
monitor backfill --source all        Backfill historical data
monitor useradd --name <n> --password <p> --role viewer|admin
```

### Backfill

```sh
# All IoTawatt devices + weather, last 30 days
go run ./cmd/monitor backfill --source all

# Single device, specific date range
go run ./cmd/monitor backfill --source pwrmone1 --from 2026-01-01 --to 2026-05-01

# Sources: pwrmone1 | pwrmona1 | pwrmona2 | ambient_weather | all
```

### Create a user account

```sh
go run ./cmd/monitor useradd --name tyler --password secret --role admin --email tyler@example.com
```

| Flag | Required | Default | Description |
|---|---|---|---|
| `--name` | yes | — | Username |
| `--password` | yes | — | Password (bcrypt-hashed before storage) |
| `--role` | no | `viewer` | `admin` or `viewer` |
| `--email` | no | — | Email address for alert notifications |
| `--phone` | no | — | Phone number for SMS alerts |

---

## UI development

Run the Vite dev server alongside the Go backend for hot reload:

```sh
# Terminal 1 — backend
go run ./cmd/monitor serve

# Terminal 2 — frontend
cd ui
npm run dev
```

Set `DEV_CORS_ORIGIN=http://localhost:5173` in `.env`. The Vite dev server proxies `/api` and WebSocket requests to `localhost:8080`.

UI is at `http://localhost:5173` during development.

---

## Testing

See [TESTING.md](TESTING.md) for unit and integration test instructions.
