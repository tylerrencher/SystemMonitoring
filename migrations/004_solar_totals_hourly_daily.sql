-- 1-hour continuous aggregate for solar totals
CREATE MATERIALIZED VIEW IF NOT EXISTS solar_totals_1hour
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', bucket) AS bucket,
    metric,
    AVG(avg_value) AS avg_value,
    MAX(max_value) AS max_value,
    MIN(min_value) AS min_value
FROM solar_totals_1min
GROUP BY time_bucket('1 hour', bucket), metric
WITH NO DATA;

SELECT add_continuous_aggregate_policy('solar_totals_1hour',
    start_offset => INTERVAL '3 hours',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);

-- 1-day continuous aggregate for solar totals
CREATE MATERIALIZED VIEW IF NOT EXISTS solar_totals_1day
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', bucket) AS bucket,
    metric,
    AVG(avg_value) AS avg_value,
    MAX(max_value) AS max_value,
    MIN(min_value) AS min_value
FROM solar_totals_1hour
GROUP BY time_bucket('1 day', bucket), metric
WITH NO DATA;

SELECT add_continuous_aggregate_policy('solar_totals_1day',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE);
