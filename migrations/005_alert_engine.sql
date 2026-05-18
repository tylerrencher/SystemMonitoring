CREATE TABLE alerts (
    id          SERIAL      PRIMARY KEY,
    key         TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    type        TEXT        NOT NULL CHECK (type IN (
                    'threshold_breach',
                    'short_cycle',
                    'scheduled_check',
                    'differential',
                    'count_per_window',
                    'cycle_complete'
                )),
    data_source TEXT        NOT NULL CHECK (data_source IN (
                    'iotawatt', 'solar_totals', 'solar_readings'
                )),
    params      JSONB       NOT NULL,
    cooldown    INTERVAL    NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE alert_preferences
    ADD CONSTRAINT fk_alert_preferences_alert
    FOREIGN KEY (alert_key) REFERENCES alerts(key) ON DELETE CASCADE;

ALTER TABLE alert_mutes
    ADD CONSTRAINT fk_alert_mutes_alert
    FOREIGN KEY (alert_key) REFERENCES alerts(key) ON DELETE CASCADE;

-- Seed alert definitions
INSERT INTO alerts (key, name, type, data_source, params, cooldown) VALUES

('short_cycle:UpstrHVAC',
 'Upstairs HVAC Short Cycling',
 'short_cycle', 'iotawatt',
 '{"series":"UpstrHVAC","on_threshold_w":50,"max_cycle_min":10,"cycle_count":3,"window_min":60}',
 INTERVAL '5 minutes'),

('short_cycle:BsmtHVAC',
 'Basement HVAC Short Cycling',
 'short_cycle', 'iotawatt',
 '{"series":"BsmtHVAC","on_threshold_w":50,"max_cycle_min":10,"cycle_count":3,"window_min":60}',
 INTERVAL '5 minutes'),

('well_pump_low_power',
 'Well Pump Low Power',
 'threshold_breach', 'iotawatt',
 '{"series":"WellPump","operator":"between","low":100,"high":2700,"sustained_min":60}',
 INTERVAL '5 minutes'),

('well_pump_pre_dry',
 'Well Pump About to Run Dry',
 'threshold_breach', 'iotawatt',
 '{"series":"WellPump","operator":"gt","threshold":3200,"sustained_min":0}',
 INTERVAL '5 minutes'),

('well_pump_dry',
 'Well Pump Has Run Dry',
 'short_cycle', 'iotawatt',
 '{"series":"WellPump","on_threshold_w":3200,"max_cycle_min":1,"cycle_count":3,"window_min":30}',
 INTERVAL '5 minutes'),

('battery_evening',
 'Low Battery Entering Evening',
 'scheduled_check', 'solar_totals',
 '{"check_time":"16:00","series":"battery_state_of_charge","operator":"lt","threshold":33}',
 INTERVAL '24 hours'),

('battery_critical',
 'Battery Critically Low',
 'threshold_breach', 'solar_totals',
 '{"series":"battery_state_of_charge","operator":"lt","threshold":10,"sustained_min":0}',
 INTERVAL '5 minutes'),

('generator_late',
 'Generator Running Past 1am',
 'scheduled_check', 'iotawatt',
 '{"check_time":"01:00","series":"Generator","operator":"gt","threshold":100}',
 INTERVAL '24 hours'),

('generator_low_output',
 'Generator Low Output',
 'threshold_breach', 'iotawatt',
 '{"series":"Generator","operator":"between","low":100,"high":20000,"sustained_min":60}',
 INTERVAL '5 minutes'),

('gen_heater_on',
 'Generator Block Heater Left On',
 'threshold_breach', 'iotawatt',
 '{"series":"GenHtr","operator":"gt","threshold":100,"sustained_min":60}',
 INTERVAL '5 minutes'),

('house_overload',
 'House Power Overload',
 'threshold_breach', 'iotawatt',
 '{"series":"House","operator":"gt","threshold":18000,"sustained_min":0}',
 INTERVAL '5 minutes'),

('leg_imbalance',
 'House Leg Imbalance',
 'differential', 'iotawatt',
 '{"series_a":"HouseL1","series_b":"HouseL2","max_diff_w":2000,"sustained_min":10}',
 INTERVAL '30 minutes'),

('dryer_done',
 'Dryer Cycle Complete',
 'cycle_complete', 'iotawatt',
 '{"series":"Dryer","on_threshold_w":50,"min_run_min":60,"off_confirm_min":10}',
 INTERVAL '60 minutes'),

('hybdwh_element_frequent',
 'Hybrid Water Heater Element Running Frequently',
 'count_per_window', 'iotawatt',
 '{"series":"HybdWH","on_threshold_w":4000,"min_gap_min":5,"max_count":1,"window_hours":24}',
 INTERVAL '24 hours'),

('resistwh_on',
 'Resistance Water Heater Left On',
 'threshold_breach', 'iotawatt',
 '{"series":"ResistWH","operator":"gt","threshold":100,"sustained_min":0}',
 INTERVAL '12 hours')

ON CONFLICT (key) DO NOTHING;
