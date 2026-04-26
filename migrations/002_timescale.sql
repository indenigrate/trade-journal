-- +goose Up

CREATE EXTENSION IF NOT EXISTS timescaledb;

CREATE TABLE behavioral_metrics (
    bucket              TIMESTAMPTZ NOT NULL,
    user_id             UUID        NOT NULL,
    trade_count         INT         NOT NULL DEFAULT 0,
    win_count           INT         NOT NULL DEFAULT 0,
    loss_count          INT         NOT NULL DEFAULT 0,
    total_pnl           NUMERIC(18,8) NOT NULL DEFAULT 0,
    avg_plan_adherence  NUMERIC(5,4),
    session_tilt_index  NUMERIC(5,4),
    revenge_count       INT         NOT NULL DEFAULT 0,
    overtrading_events  INT         NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket, user_id)
);

SELECT create_hypertable('behavioral_metrics', 'bucket', chunk_time_interval => INTERVAL '1 hour');

-- +goose Down
DROP TABLE IF EXISTS behavioral_metrics;
-- Note: Cannot easily drop timescaledb extension if other hypertables exist
