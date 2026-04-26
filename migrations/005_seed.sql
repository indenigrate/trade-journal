-- +goose Up
-- +goose NO TRANSACTION

-- Seed users (extracted from nevup_seed_dataset.csv — 10 unique traders)
INSERT INTO users (user_id, name, role, pathology) VALUES
    ('f412f236-4edc-47a2-8f54-8763a6ed2ce8', 'Alex Mercer', 'trader', 'revenge_trading'),
    ('fcd434aa-2201-4060-aeb2-f44c77aa0683', 'Jordan Lee', 'trader', 'overtrading'),
    ('84a6a3dd-f2d0-4167-960b-7319a6033d49', 'Sam Rivera', 'trader', 'fomo_entries'),
    ('4f2f0816-f350-4684-b6c3-29bbddbb1869', 'Casey Kim', 'trader', 'plan_non_adherence'),
    ('75076413-e8e8-44ac-861f-c7acb3902d6d', 'Morgan Bell', 'trader', 'premature_exit'),
    ('8effb0f2-f16b-4b5f-87ab-7ffca376f309', 'Taylor Grant', 'trader', 'loss_running'),
    ('50dd1053-73b0-43c5-8d0f-d2af88c01451', 'Riley Stone', 'trader', 'session_tilt'),
    ('af2cfc5e-c132-4989-9c12-2913f89271fb', 'Drew Patel', 'trader', 'time_of_day_bias'),
    ('9419073a-3d58-4ee6-a917-be2d40aecef2', 'Quinn Torres', 'trader', 'position_sizing_inconsistency'),
    ('e84ea28c-e5a7-49ef-ac26-a873e32667bd', 'Avery Chen', 'trader', NULL)
ON CONFLICT (user_id) DO NOTHING;

-- Seed sessions (extracted from nevup_seed_dataset.csv — 52 unique sessions)
-- Alex Mercer sessions
INSERT INTO sessions (session_id, user_id, started_at) VALUES
    ('4f39c2ea-8687-41f7-85a0-1fafd3e976df', 'f412f236-4edc-47a2-8f54-8763a6ed2ce8', '2025-01-06T09:30:00Z'),
    ('882aefb1-0306-46ce-b2fc-af5392fd5ede', 'f412f236-4edc-47a2-8f54-8763a6ed2ce8', '2025-01-13T09:30:00Z'),
    ('12b6ce81-67e7-45fc-a642-587f40142f4e', 'f412f236-4edc-47a2-8f54-8763a6ed2ce8', '2025-01-20T09:30:00Z'),
    ('ea912362-d5a7-4dfd-b415-ead75da78176', 'f412f236-4edc-47a2-8f54-8763a6ed2ce8', '2025-01-27T09:30:00Z'),
    ('57651b39-b4f9-496c-9afb-36535f841fb4', 'f412f236-4edc-47a2-8f54-8763a6ed2ce8', '2025-02-03T09:30:00Z'),
    -- Jordan Lee sessions
    ('29557b38-1332-4a4d-a688-f1cac77416c8', 'fcd434aa-2201-4060-aeb2-f44c77aa0683', '2025-01-07T09:00:00Z'),
    ('ae82ad74-8559-46f5-a382-64017ac6599b', 'fcd434aa-2201-4060-aeb2-f44c77aa0683', '2025-01-14T09:00:00Z'),
    ('19ae4f1d-a483-4f6e-997b-428d046b2a9e', 'fcd434aa-2201-4060-aeb2-f44c77aa0683', '2025-01-21T09:00:00Z'),
    ('272eff27-4b78-4e3a-8bbf-4fff93399e84', 'fcd434aa-2201-4060-aeb2-f44c77aa0683', '2025-01-28T09:00:00Z'),
    ('1b39f703-fd06-461b-8653-f6243adae70e', 'fcd434aa-2201-4060-aeb2-f44c77aa0683', '2025-02-04T09:00:00Z'),
    -- Sam Rivera sessions
    ('0f414e15-8904-4c86-a076-d7bcb90decc3', '84a6a3dd-f2d0-4167-960b-7319a6033d49', '2025-01-08T09:37:00Z'),
    ('0152de5e-1dbf-469d-8b93-2043fa7561a8', '84a6a3dd-f2d0-4167-960b-7319a6033d49', '2025-01-15T09:29:00Z'),
    ('96136dc0-6877-40ef-9c16-d215ece0d8bd', '84a6a3dd-f2d0-4167-960b-7319a6033d49', '2025-01-22T09:23:00Z'),
    ('aad293e2-1d80-4244-9f9e-90cd13c71f9d', '84a6a3dd-f2d0-4167-960b-7319a6033d49', '2025-01-29T09:10:00Z'),
    ('4d04641e-2c68-4083-b182-f7d2a998c28e', '84a6a3dd-f2d0-4167-960b-7319a6033d49', '2025-02-05T09:18:00Z'),
    -- Casey Kim sessions
    ('d0e24e7b-14e8-4de5-bb00-8dd60f980f11', '4f2f0816-f350-4684-b6c3-29bbddbb1869', '2025-01-09T09:30:00Z'),
    ('c002e183-f562-4e86-ac99-93cf98f1f84e', '4f2f0816-f350-4684-b6c3-29bbddbb1869', '2025-01-16T09:30:00Z'),
    ('88609253-0c98-4918-9fb9-4ceed399d320', '4f2f0816-f350-4684-b6c3-29bbddbb1869', '2025-01-23T09:30:00Z'),
    ('ef3ff180-8ad4-4e05-861d-c0e2cec878e3', '4f2f0816-f350-4684-b6c3-29bbddbb1869', '2025-01-30T09:30:00Z'),
    ('d64b42ed-ef85-4ed7-87d2-67c538c0adb7', '4f2f0816-f350-4684-b6c3-29bbddbb1869', '2025-02-06T09:30:00Z'),
    -- Morgan Bell sessions
    ('12865ff1-720a-41b6-a2b4-7728ccaca660', '75076413-e8e8-44ac-861f-c7acb3902d6d', '2025-01-10T09:00:00Z'),
    ('5073c733-2b53-48f7-9a43-234a5fa7751c', '75076413-e8e8-44ac-861f-c7acb3902d6d', '2025-01-17T09:00:00Z'),
    ('021498af-96ae-4f2b-89ef-a01859a7c547', '75076413-e8e8-44ac-861f-c7acb3902d6d', '2025-01-24T09:00:00Z'),
    ('b08709ce-3354-43c2-a2c8-80f861be185c', '75076413-e8e8-44ac-861f-c7acb3902d6d', '2025-01-31T09:00:00Z'),
    ('77bee375-cb75-4e94-a2ff-1681da84d912', '75076413-e8e8-44ac-861f-c7acb3902d6d', '2025-02-07T09:00:00Z'),
    -- Taylor Grant sessions
    ('722d0010-d93d-4c9c-97d7-5189a875edc9', '8effb0f2-f16b-4b5f-87ab-7ffca376f309', '2025-01-13T09:30:00Z'),
    ('e24b1de1-0bb7-42df-b928-70057dee7b99', '8effb0f2-f16b-4b5f-87ab-7ffca376f309', '2025-01-20T09:30:00Z'),
    ('a0e1f294-c4df-4ca9-94e5-da10052668c7', '8effb0f2-f16b-4b5f-87ab-7ffca376f309', '2025-01-27T09:30:00Z'),
    ('64e07407-c163-459b-8870-ca88e64f3214', '8effb0f2-f16b-4b5f-87ab-7ffca376f309', '2025-02-03T09:30:00Z'),
    ('483d8cdc-9845-4a2b-900d-fa71959a78e2', '8effb0f2-f16b-4b5f-87ab-7ffca376f309', '2025-02-10T09:30:00Z'),
    -- Riley Stone sessions
    ('dec67127-f4c1-4f6f-9fc2-dbe046718f58', '50dd1053-73b0-43c5-8d0f-d2af88c01451', '2025-01-14T09:00:00Z'),
    ('cf9e770c-913f-4c1d-8bed-d6b95c7ba50a', '50dd1053-73b0-43c5-8d0f-d2af88c01451', '2025-01-21T09:00:00Z'),
    ('49c2a73f-38c3-4852-b614-eaa2c6ef518c', '50dd1053-73b0-43c5-8d0f-d2af88c01451', '2025-01-28T09:00:00Z'),
    ('42cf9c77-bcfb-4306-899f-d2b5fb5f763a', '50dd1053-73b0-43c5-8d0f-d2af88c01451', '2025-02-04T09:00:00Z'),
    ('f86490ed-8b70-4090-a595-a09ef25691fe', '50dd1053-73b0-43c5-8d0f-d2af88c01451', '2025-02-11T09:00:00Z'),
    -- Drew Patel sessions
    ('29322429-a5b4-4e7c-8d8d-c78f1bbbe460', 'af2cfc5e-c132-4989-9c12-2913f89271fb', '2025-01-15T09:30:00Z'),
    ('cbd7f851-85f5-41e5-9de6-599a1d0c6054', 'af2cfc5e-c132-4989-9c12-2913f89271fb', '2025-01-20T09:30:00Z'),
    ('62b3c2d4-e31f-4bf1-bbb3-498d99c30b94', 'af2cfc5e-c132-4989-9c12-2913f89271fb', '2025-01-25T09:30:00Z'),
    ('1546371f-8b18-4033-8968-228630891e66', 'af2cfc5e-c132-4989-9c12-2913f89271fb', '2025-01-30T09:30:00Z'),
    ('2d3c10dc-798c-48d4-8cb9-3585020bcdc4', 'af2cfc5e-c132-4989-9c12-2913f89271fb', '2025-02-04T09:30:00Z'),
    ('22f2b21f-d838-4e9e-93c5-c560c4f46fbe', 'af2cfc5e-c132-4989-9c12-2913f89271fb', '2025-02-09T09:30:00Z'),
    -- Quinn Torres sessions
    ('2eee3ecd-1c43-41c0-8ded-96d6ba475b39', '9419073a-3d58-4ee6-a917-be2d40aecef2', '2025-01-16T09:30:00Z'),
    ('f97a2050-b038-4864-87d4-3c3d019827d3', '9419073a-3d58-4ee6-a917-be2d40aecef2', '2025-01-23T09:30:00Z'),
    ('6cd4b274-5212-4f2e-ab81-7d332437087a', '9419073a-3d58-4ee6-a917-be2d40aecef2', '2025-01-30T09:30:00Z'),
    ('b907e0bc-1213-4ffd-9195-9bdb7d4d2310', '9419073a-3d58-4ee6-a917-be2d40aecef2', '2025-02-06T09:30:00Z'),
    ('92cfb202-0675-4883-9156-ba41949abde1', '9419073a-3d58-4ee6-a917-be2d40aecef2', '2025-02-13T09:30:00Z'),
    -- Avery Chen sessions
    ('1aeec0aa-c818-4150-9b00-74eedce478f7', 'e84ea28c-e5a7-49ef-ac26-a873e32667bd', '2025-01-17T09:30:00Z'),
    ('906efb8a-f7de-4e99-ab85-5e01d253a3ae', 'e84ea28c-e5a7-49ef-ac26-a873e32667bd', '2025-01-22T09:30:00Z'),
    ('68c2df05-bedf-42ed-a341-908a3f92f589', 'e84ea28c-e5a7-49ef-ac26-a873e32667bd', '2025-01-27T09:30:00Z'),
    ('5889716a-8d1e-40ff-9e99-9c0243ae2888', 'e84ea28c-e5a7-49ef-ac26-a873e32667bd', '2025-02-01T09:30:00Z'),
    ('3d647291-1b5e-4ba9-9a6d-e56383cf30d6', 'e84ea28c-e5a7-49ef-ac26-a873e32667bd', '2025-02-06T09:30:00Z'),
    ('3385003e-9b9a-4eb9-9a8a-7e5d5f83f051', 'e84ea28c-e5a7-49ef-ac26-a873e32667bd', '2025-02-11T09:30:00Z')
ON CONFLICT (session_id) DO NOTHING;

-- Create a temporary staging table for CSV import
CREATE TEMP TABLE trades_staging (
    trade_id TEXT,
    user_id TEXT,
    trader_name TEXT,
    session_id TEXT,
    asset TEXT,
    asset_class TEXT,
    direction TEXT,
    entry_price TEXT,
    exit_price TEXT,
    quantity TEXT,
    entry_at TEXT,
    exit_at TEXT,
    status TEXT,
    outcome TEXT,
    pnl TEXT,
    plan_adherence TEXT,
    emotional_state TEXT,
    entry_rationale TEXT,
    revenge_flag TEXT,
    ground_truth_pathologies TEXT
);

-- Load CSV into staging table
COPY trades_staging FROM '/seeds/nevup_seed_dataset.csv' WITH (FORMAT csv, HEADER true);

-- Insert into trades from staging, mapping columns and handling types
INSERT INTO trades (
    trade_id, user_id, session_id, asset, asset_class, direction,
    entry_price, exit_price, quantity, entry_at, exit_at, status,
    plan_adherence, emotional_state, entry_rationale,
    outcome, pnl, revenge_flag, created_at, updated_at
)
SELECT
    trade_id::uuid,
    user_id::uuid,
    session_id::uuid,
    asset,
    asset_class::asset_class_enum,
    direction::direction_enum,
    entry_price::numeric(18,8),
    CASE WHEN exit_price = '' OR exit_price IS NULL THEN NULL ELSE exit_price::numeric(18,8) END,
    quantity::numeric(18,8),
    entry_at::timestamptz,
    CASE WHEN exit_at = '' OR exit_at IS NULL THEN NULL ELSE exit_at::timestamptz END,
    status::status_enum,
    CASE WHEN plan_adherence = '' OR plan_adherence IS NULL THEN NULL ELSE plan_adherence::smallint END,
    CASE WHEN emotional_state = '' OR emotional_state IS NULL THEN NULL ELSE emotional_state::emotion_enum END,
    CASE WHEN entry_rationale = '' THEN NULL ELSE entry_rationale END,
    CASE WHEN outcome = '' OR outcome IS NULL THEN NULL ELSE outcome::outcome_enum END,
    CASE WHEN pnl = '' OR pnl IS NULL THEN NULL ELSE pnl::numeric(18,8) END,
    CASE WHEN revenge_flag = 'true' THEN true ELSE false END,
    entry_at::timestamptz,  -- use trade entry time as created_at for seeded data
    entry_at::timestamptz   -- use trade entry time as updated_at for seeded data
FROM trades_staging
ON CONFLICT (trade_id) DO NOTHING;

DROP TABLE trades_staging;

-- Seed behavioral_metrics from trades data so metrics are queryable immediately
INSERT INTO behavioral_metrics (bucket, user_id, trade_count, win_count, loss_count, total_pnl, avg_plan_adherence, revenge_count)
SELECT
    date_trunc('hour', entry_at) AS bucket,
    user_id,
    COUNT(*) AS trade_count,
    COUNT(*) FILTER (WHERE outcome = 'win') AS win_count,
    COUNT(*) FILTER (WHERE outcome = 'loss') AS loss_count,
    COALESCE(SUM(pnl), 0) AS total_pnl,
    AVG(plan_adherence)::numeric(5,4) AS avg_plan_adherence,
    COUNT(*) FILTER (WHERE revenge_flag = true) AS revenge_count
FROM trades
WHERE status = 'closed'
GROUP BY date_trunc('hour', entry_at), user_id
ON CONFLICT (bucket, user_id) DO UPDATE SET
    trade_count = EXCLUDED.trade_count,
    win_count = EXCLUDED.win_count,
    loss_count = EXCLUDED.loss_count,
    total_pnl = EXCLUDED.total_pnl,
    avg_plan_adherence = EXCLUDED.avg_plan_adherence,
    revenge_count = EXCLUDED.revenge_count;

-- Seed emotion_win_rates
INSERT INTO emotion_win_rates (user_id, emotional_state, date_bucket, wins, losses)
SELECT
    user_id,
    emotional_state,
    entry_at::date AS date_bucket,
    COUNT(*) FILTER (WHERE outcome = 'win') AS wins,
    COUNT(*) FILTER (WHERE outcome = 'loss') AS losses
FROM trades
WHERE status = 'closed' AND emotional_state IS NOT NULL
GROUP BY user_id, emotional_state, entry_at::date
ON CONFLICT (user_id, emotional_state, date_bucket) DO UPDATE SET
    wins = EXCLUDED.wins,
    losses = EXCLUDED.losses;

-- Seed session tilt index into behavioral_metrics
-- For each session, calculate tilt and update the metrics for the session's hour bucket
WITH session_tilts AS (
    SELECT
        session_id,
        user_id,
        MIN(entry_at) AS session_start,
        COUNT(*) FILTER (WHERE prev_outcome = 'loss') AS loss_follow_count,
        COUNT(*) AS total_count
    FROM (
        SELECT
            session_id,
            user_id,
            entry_at,
            outcome,
            LAG(outcome) OVER (PARTITION BY session_id ORDER BY entry_at) AS prev_outcome
        FROM trades
        WHERE status = 'closed'
    ) sub
    WHERE prev_outcome IS NOT NULL
    GROUP BY session_id, user_id
)
UPDATE behavioral_metrics bm
SET session_tilt_index = CASE
    WHEN st.total_count > 0 THEN (st.loss_follow_count::numeric / st.total_count)::numeric(5,4)
    ELSE 0
END
FROM session_tilts st
WHERE bm.user_id = st.user_id
  AND bm.bucket = date_trunc('hour', st.session_start);

-- +goose Down
DELETE FROM emotion_win_rates;
DELETE FROM behavioral_metrics;
DELETE FROM trades;
DELETE FROM sessions;
DELETE FROM users;
