-- ON CONFLICT DO NOTHING requires unique indexes on hypertables.
-- TimescaleDB requires the partitioning column (time) to be part of every unique index.

CREATE UNIQUE INDEX IF NOT EXISTS iotawatt_readings_unique
    ON iotawatt_readings (time, device, series);

CREATE UNIQUE INDEX IF NOT EXISTS solar_readings_unique
    ON solar_readings (time, inverter, metric);

CREATE UNIQUE INDEX IF NOT EXISTS solar_totals_unique
    ON solar_totals (time, metric);

CREATE UNIQUE INDEX IF NOT EXISTS weather_readings_unique
    ON weather_readings (time, metric);
