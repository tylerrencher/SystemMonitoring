-- 1-minute continuous aggregate for IoTawatt
CREATE MATERIALIZED VIEW IF NOT EXISTS iotawatt_1min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', time) AS bucket,
    device,
    series,
    AVG(watts) AS avg_watts,
    MAX(watts) AS max_watts,
    MIN(watts) AS min_watts,
    AVG(volts) AS avg_volts
FROM iotawatt_readings
GROUP BY bucket, device, series
WITH NO DATA;

SELECT add_continuous_aggregate_policy('iotawatt_1min',
    start_offset => INTERVAL '3 minutes',
    end_offset   => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute',
    if_not_exists => TRUE);

-- 1-hour continuous aggregate for IoTawatt
CREATE MATERIALIZED VIEW IF NOT EXISTS iotawatt_1hour
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', bucket) AS bucket,
    device,
    series,
    AVG(avg_watts) AS avg_watts,
    MAX(max_watts) AS max_watts,
    MIN(min_watts) AS min_watts,
    AVG(avg_volts) AS avg_volts
FROM iotawatt_1min
GROUP BY time_bucket('1 hour', bucket), device, series
WITH NO DATA;

SELECT add_continuous_aggregate_policy('iotawatt_1hour',
    start_offset => INTERVAL '3 hours',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

-- 1-day continuous aggregate for IoTawatt
CREATE MATERIALIZED VIEW IF NOT EXISTS iotawatt_1day
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', bucket) AS bucket,
    device,
    series,
    AVG(avg_watts) AS avg_watts,
    MAX(max_watts) AS max_watts,
    MIN(min_watts) AS min_watts,
    AVG(avg_volts) AS avg_volts
FROM iotawatt_1hour
GROUP BY time_bucket('1 day', bucket), device, series
WITH NO DATA;

SELECT add_continuous_aggregate_policy('iotawatt_1day',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE);

-- 1-minute continuous aggregate for solar readings (numeric only)
CREATE MATERIALIZED VIEW IF NOT EXISTS solar_readings_1min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', time) AS bucket,
    inverter,
    metric,
    AVG(value_numeric) AS avg_value,
    MAX(value_numeric) AS max_value,
    MIN(value_numeric) AS min_value
FROM solar_readings
WHERE value_numeric IS NOT NULL
GROUP BY bucket, inverter, metric
WITH NO DATA;

SELECT add_continuous_aggregate_policy('solar_readings_1min',
    start_offset => INTERVAL '3 minutes',
    end_offset   => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute',
    if_not_exists => TRUE);

-- 1-hour continuous aggregate for solar readings
CREATE MATERIALIZED VIEW IF NOT EXISTS solar_readings_1hour
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', bucket) AS bucket,
    inverter,
    metric,
    AVG(avg_value) AS avg_value,
    MAX(max_value) AS max_value,
    MIN(min_value) AS min_value
FROM solar_readings_1min
GROUP BY time_bucket('1 hour', bucket), inverter, metric
WITH NO DATA;

SELECT add_continuous_aggregate_policy('solar_readings_1hour',
    start_offset => INTERVAL '3 hours',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

-- 1-day continuous aggregate for solar readings
CREATE MATERIALIZED VIEW IF NOT EXISTS solar_readings_1day
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', bucket) AS bucket,
    inverter,
    metric,
    AVG(avg_value) AS avg_value,
    MAX(max_value) AS max_value,
    MIN(min_value) AS min_value
FROM solar_readings_1hour
GROUP BY time_bucket('1 day', bucket), inverter, metric
WITH NO DATA;

SELECT add_continuous_aggregate_policy('solar_readings_1day',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE);

-- 1-minute continuous aggregate for solar totals
CREATE MATERIALIZED VIEW IF NOT EXISTS solar_totals_1min
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', time) AS bucket,
    metric,
    AVG(value_numeric) AS avg_value,
    MAX(value_numeric) AS max_value,
    MIN(value_numeric) AS min_value
FROM solar_totals
WHERE value_numeric IS NOT NULL
GROUP BY bucket, metric
WITH NO DATA;

SELECT add_continuous_aggregate_policy('solar_totals_1min',
    start_offset => INTERVAL '3 minutes',
    end_offset   => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute',
    if_not_exists => TRUE);
