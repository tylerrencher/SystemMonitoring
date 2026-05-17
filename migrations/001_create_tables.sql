CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ingestion watermarks for gap-fill tracking
CREATE TABLE IF NOT EXISTS ingestion_watermarks (
    source           TEXT PRIMARY KEY,
    last_ingested_at TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- IoTawatt readings (all devices, all series)
CREATE TABLE IF NOT EXISTS iotawatt_readings (
    time   TIMESTAMPTZ NOT NULL,
    device TEXT        NOT NULL,
    series TEXT        NOT NULL,
    watts  REAL,
    volts  REAL
);
SELECT create_hypertable('iotawatt_readings', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS iotawatt_readings_device_series_time
    ON iotawatt_readings (device, series, time DESC);

-- SolarAssistant per-inverter live metrics
CREATE TABLE IF NOT EXISTS solar_readings (
    time          TIMESTAMPTZ NOT NULL,
    inverter      TEXT        NOT NULL,
    metric        TEXT        NOT NULL,
    value_numeric REAL,
    value_text    TEXT
);
SELECT create_hypertable('solar_readings', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS solar_readings_inverter_metric_time
    ON solar_readings (inverter, metric, time DESC);

-- SolarAssistant totals (battery_state_of_charge, battery_power, battery_temperature)
CREATE TABLE IF NOT EXISTS solar_totals (
    time          TIMESTAMPTZ NOT NULL,
    metric        TEXT        NOT NULL,
    value_numeric REAL,
    value_text    TEXT
);
SELECT create_hypertable('solar_totals', 'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS solar_totals_metric_time
    ON solar_totals (metric, time DESC);

-- SolarAssistant config change log
CREATE TABLE IF NOT EXISTS solar_config_changes (
    time      TIMESTAMPTZ NOT NULL,
    inverter  TEXT        NOT NULL,
    key       TEXT        NOT NULL,
    old_value TEXT,
    new_value TEXT        NOT NULL
);

-- Weather readings (Ambient Weather API)
CREATE TABLE IF NOT EXISTS weather_readings (
    time          TIMESTAMPTZ NOT NULL,
    metric        TEXT        NOT NULL,
    value_numeric REAL,
    value_text    TEXT
);
SELECT create_hypertable('weather_readings', 'time',
    chunk_time_interval => INTERVAL '7 days',
    if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS weather_readings_metric_time
    ON weather_readings (metric, time DESC);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id            SERIAL PRIMARY KEY,
    name          TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    role          TEXT        NOT NULL CHECK (role IN ('admin', 'viewer')),
    phone         TEXT,
    email         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-user alert subscriptions
CREATE TABLE IF NOT EXISTS alert_preferences (
    user_id   INT  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_key TEXT NOT NULL,
    PRIMARY KEY (user_id, alert_key)
);

-- Per-user alert mutes
CREATE TABLE IF NOT EXISTS alert_mutes (
    user_id     INT         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    alert_key   TEXT        NOT NULL,
    muted_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, alert_key)
);

-- Sessions
CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT        PRIMARY KEY,
    user_id    INT         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
