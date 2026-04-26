package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/onesine/nevup-backend/internal/domain"
	"github.com/shopspring/decimal"
)

type TradeStore struct {
	pool *pgxpool.Pool
}

func NewTradeStore(pool *pgxpool.Pool) *TradeStore {
	return &TradeStore{pool: pool}
}

// Upsert inserts a trade or returns the existing record on conflict (idempotent).
func (s *TradeStore) Upsert(ctx context.Context, t domain.Trade) (domain.Trade, error) {
	const insertSQL = `
		INSERT INTO trades (
			trade_id, user_id, session_id, asset, asset_class, direction,
			entry_price, exit_price, quantity, entry_at, exit_at, status,
			plan_adherence, emotional_state, entry_rationale,
			outcome, pnl, revenge_flag, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,now(),now()
		)
		ON CONFLICT (trade_id) DO NOTHING
		RETURNING trade_id, user_id, session_id, asset, asset_class, direction,
			entry_price, exit_price, quantity, entry_at, exit_at, status,
			plan_adherence, emotional_state, entry_rationale,
			outcome, pnl, revenge_flag, created_at, updated_at`

	row := s.pool.QueryRow(ctx, insertSQL,
		t.TradeID, t.UserID, t.SessionID, t.Asset, t.AssetClass, t.Direction,
		t.EntryPrice, t.ExitPrice, t.Quantity, t.EntryAt, t.ExitAt, t.Status,
		t.PlanAdherence, t.EmotionalState, t.EntryRationale,
		t.Outcome, t.PnL, t.RevengeFlag,
	)

	trade, err := scanTrade(row)
	if errors.Is(err, pgx.ErrNoRows) {
		// Conflict: trade_id already exists — fetch the canonical record
		const selectSQL = `SELECT trade_id, user_id, session_id, asset, asset_class, direction,
			entry_price, exit_price, quantity, entry_at, exit_at, status,
			plan_adherence, emotional_state, entry_rationale,
			outcome, pnl, revenge_flag, created_at, updated_at
		FROM trades WHERE trade_id = $1`
		row = s.pool.QueryRow(ctx, selectSQL, t.TradeID)
		trade, err = scanTrade(row)
	}

	return trade, err
}

// GetByID fetches a single trade by ID.
func (s *TradeStore) GetByID(ctx context.Context, tradeID string) (domain.Trade, error) {
	const sql = `SELECT trade_id, user_id, session_id, asset, asset_class, direction,
		entry_price, exit_price, quantity, entry_at, exit_at, status,
		plan_adherence, emotional_state, entry_rationale,
		outcome, pnl, revenge_flag, created_at, updated_at
	FROM trades WHERE trade_id = $1`

	row := s.pool.QueryRow(ctx, sql, tradeID)
	trade, err := scanTrade(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Trade{}, domain.ErrNotFound
	}
	return trade, err
}

// ListBySession fetches all trades for a session, ordered by entry_at.
func (s *TradeStore) ListBySession(ctx context.Context, sessionID string) ([]domain.Trade, error) {
	const sql = `SELECT trade_id, user_id, session_id, asset, asset_class, direction,
		entry_price, exit_price, quantity, entry_at, exit_at, status,
		plan_adherence, emotional_state, entry_rationale,
		outcome, pnl, revenge_flag, created_at, updated_at
	FROM trades WHERE session_id = $1 ORDER BY entry_at`

	rows, err := s.pool.Query(ctx, sql, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []domain.Trade
	for rows.Next() {
		t, err := scanTradeRows(rows)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, rows.Err()
}

// GetLastPlanAdherenceRatings returns the last N plan adherence ratings for a user.
func (s *TradeStore) GetLastPlanAdherenceRatings(ctx context.Context, userID string, limit int) ([]int, error) {
	const sql = `SELECT plan_adherence
		FROM trades
		WHERE user_id = $1 AND status = 'closed' AND plan_adherence IS NOT NULL
		ORDER BY exit_at DESC
		LIMIT $2`

	rows, err := s.pool.Query(ctx, sql, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ratings []int
	for rows.Next() {
		var r int
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		ratings = append(ratings, r)
	}
	return ratings, rows.Err()
}

// GetSessionTiltData returns loss-follow count and total count for session tilt calculation.
func (s *TradeStore) GetSessionTiltData(ctx context.Context, sessionID string) (lossFollowCount, totalCount int, err error) {
	const sql = `SELECT
		COUNT(*) FILTER (WHERE prev_outcome = 'loss') AS loss_follow_count,
		COUNT(*) AS total_count
	FROM (
		SELECT
			outcome,
			LAG(outcome) OVER (PARTITION BY session_id ORDER BY entry_at) AS prev_outcome
		FROM trades
		WHERE session_id = $1
	) sub
	WHERE prev_outcome IS NOT NULL`

	err = s.pool.QueryRow(ctx, sql, sessionID).Scan(&lossFollowCount, &totalCount)
	return
}

func scanTrade(row pgx.Row) (domain.Trade, error) {
	var t domain.Trade
	var exitPrice *decimal.Decimal
	var exitAt *time.Time
	var planAdherence *int
	var emotionalState *string
	var entryRationale *string
	var outcome *string
	var pnl *decimal.Decimal

	err := row.Scan(
		&t.TradeID, &t.UserID, &t.SessionID, &t.Asset, &t.AssetClass, &t.Direction,
		&t.EntryPrice, &exitPrice, &t.Quantity, &t.EntryAt, &exitAt, &t.Status,
		&planAdherence, &emotionalState, &entryRationale,
		&outcome, &pnl, &t.RevengeFlag, &t.CreatedAt, &t.UpdatedAt,
	)
	t.ExitPrice = exitPrice
	t.ExitAt = exitAt
	t.PlanAdherence = planAdherence
	t.EmotionalState = emotionalState
	t.EntryRationale = entryRationale
	t.Outcome = outcome
	t.PnL = pnl
	return t, err
}

func scanTradeRows(rows pgx.Rows) (domain.Trade, error) {
	var t domain.Trade
	var exitPrice *decimal.Decimal
	var exitAt *time.Time
	var planAdherence *int
	var emotionalState *string
	var entryRationale *string
	var outcome *string
	var pnl *decimal.Decimal

	err := rows.Scan(
		&t.TradeID, &t.UserID, &t.SessionID, &t.Asset, &t.AssetClass, &t.Direction,
		&t.EntryPrice, &exitPrice, &t.Quantity, &t.EntryAt, &exitAt, &t.Status,
		&planAdherence, &emotionalState, &entryRationale,
		&outcome, &pnl, &t.RevengeFlag, &t.CreatedAt, &t.UpdatedAt,
	)
	t.ExitPrice = exitPrice
	t.ExitAt = exitAt
	t.PlanAdherence = planAdherence
	t.EmotionalState = emotionalState
	t.EntryRationale = entryRationale
	t.Outcome = outcome
	t.PnL = pnl
	return t, err
}
