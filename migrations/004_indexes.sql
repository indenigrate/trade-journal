-- +goose Up

-- Primary query patterns for trades
CREATE INDEX idx_trades_user_entry    ON trades(user_id, entry_at DESC);
CREATE INDEX idx_trades_session       ON trades(session_id);
CREATE INDEX idx_trades_user_status   ON trades(user_id, status) WHERE status = 'closed';
CREATE INDEX idx_trades_user_exit     ON trades(user_id, exit_at DESC) WHERE exit_at IS NOT NULL;

-- Composite index for behavioral_metrics range queries
CREATE INDEX idx_metrics_user_bucket  ON behavioral_metrics(user_id, bucket DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_metrics_user_bucket;
DROP INDEX IF EXISTS idx_trades_user_exit;
DROP INDEX IF EXISTS idx_trades_user_status;
DROP INDEX IF EXISTS idx_trades_session;
DROP INDEX IF EXISTS idx_trades_user_entry;
