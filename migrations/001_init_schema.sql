-- +goose Up

-- Enumerated Types
CREATE TYPE asset_class_enum   AS ENUM ('equity', 'crypto', 'forex');
CREATE TYPE direction_enum     AS ENUM ('long', 'short');
CREATE TYPE status_enum        AS ENUM ('open', 'closed', 'cancelled');
CREATE TYPE emotion_enum       AS ENUM ('calm', 'anxious', 'greedy', 'fearful', 'neutral');
CREATE TYPE outcome_enum       AS ENUM ('win', 'loss');

-- Users Table
CREATE TABLE users (
    user_id    UUID        PRIMARY KEY,
    name       TEXT        NOT NULL,
    role       TEXT        NOT NULL DEFAULT 'trader',
    pathology  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sessions Table
CREATE TABLE sessions (
    session_id UUID        PRIMARY KEY,
    user_id    UUID        NOT NULL REFERENCES users(user_id),
    started_at TIMESTAMPTZ NOT NULL,
    notes      TEXT
);

CREATE INDEX idx_sessions_user ON sessions(user_id);

-- Trades Table (Central Fact Table)
CREATE TABLE trades (
    trade_id        UUID            PRIMARY KEY,
    user_id         UUID            NOT NULL REFERENCES users(user_id),
    session_id      UUID            NOT NULL REFERENCES sessions(session_id),
    asset           TEXT            NOT NULL,
    asset_class     asset_class_enum NOT NULL,
    direction       direction_enum  NOT NULL,
    entry_price     NUMERIC(18,8)   NOT NULL,
    exit_price      NUMERIC(18,8),
    quantity        NUMERIC(18,8)   NOT NULL,
    entry_at        TIMESTAMPTZ     NOT NULL,
    exit_at         TIMESTAMPTZ,
    status          status_enum     NOT NULL DEFAULT 'open',
    plan_adherence  SMALLINT        CHECK (plan_adherence BETWEEN 1 AND 5),
    emotional_state emotion_enum,
    entry_rationale VARCHAR(500),
    outcome         outcome_enum,
    pnl             NUMERIC(18,8),
    revenge_flag    BOOLEAN         NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now()
);

-- Debriefs Table
CREATE TABLE debriefs (
    debrief_id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id            UUID        NOT NULL REFERENCES sessions(session_id),
    overall_mood          emotion_enum NOT NULL,
    key_mistake           TEXT,
    key_lesson            TEXT,
    plan_adherence_rating SMALLINT    CHECK (plan_adherence_rating BETWEEN 1 AND 5),
    will_review_tomorrow  BOOLEAN     NOT NULL DEFAULT false,
    saved_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS debriefs;
DROP TABLE IF EXISTS trades;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS outcome_enum;
DROP TYPE IF EXISTS emotion_enum;
DROP TYPE IF EXISTS status_enum;
DROP TYPE IF EXISTS direction_enum;
DROP TYPE IF EXISTS asset_class_enum;
