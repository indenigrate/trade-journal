-- +goose Up

CREATE TABLE emotion_win_rates (
    user_id         UUID        NOT NULL REFERENCES users(user_id),
    emotional_state emotion_enum NOT NULL,
    date_bucket     DATE        NOT NULL,
    wins            INT         NOT NULL DEFAULT 0,
    losses          INT         NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, emotional_state, date_bucket)
);

CREATE TABLE overtrading_events (
    event_id      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(user_id),
    window_start  TIMESTAMPTZ NOT NULL,
    trade_count   INT         NOT NULL,
    emitted_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS overtrading_events;
DROP TABLE IF EXISTS emotion_win_rates;
