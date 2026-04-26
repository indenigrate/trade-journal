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

// EvidenceResult holds trade IDs and session IDs for a pathology claim.
type EvidenceResult struct {
	TradeIDs   []string
	SessionIDs []string
}

// GetEvidenceForPathology queries for trades matching a pathology pattern.
func (s *TradeStore) GetEvidenceForPathology(ctx context.Context, userID, pathology string) (EvidenceResult, error) {
	var whereClause string
	switch pathology {
	case "revenge_trading":
		whereClause = "AND revenge_flag = true"
	case "overtrading":
		// Trades in sessions with > 8 trades
		whereClause = "AND session_id IN (SELECT session_id FROM trades WHERE user_id = $1 GROUP BY session_id HAVING COUNT(*) > 8)"
	case "fomo_entries":
		whereClause = "AND emotional_state = 'greedy'"
	case "plan_non_adherence":
		whereClause = "AND plan_adherence IS NOT NULL AND plan_adherence <= 2"
	case "premature_exit":
		whereClause = "AND status = 'closed' AND exit_at IS NOT NULL AND EXTRACT(EPOCH FROM (exit_at - entry_at)) < 1800"
	case "loss_running":
		whereClause = "AND outcome = 'loss' AND ABS(pnl) > 100"
	case "session_tilt":
		// Trades in sessions where losses follow losses
		whereClause = "AND session_id IN (SELECT DISTINCT session_id FROM trades WHERE user_id = $1 AND outcome = 'loss')"
	case "time_of_day_bias":
		whereClause = "AND (EXTRACT(HOUR FROM entry_at) < 9 OR EXTRACT(HOUR FROM entry_at) > 15)"
	case "position_sizing_inconsistency":
		whereClause = "AND quantity > (SELECT AVG(quantity) * 1.5 FROM trades WHERE user_id = $1)"
	default:
		whereClause = ""
	}

	sql := `SELECT trade_id, session_id FROM trades WHERE user_id = $1 ` + whereClause + ` ORDER BY entry_at DESC LIMIT 10`

	rows, err := s.pool.Query(ctx, sql, userID)
	if err != nil {
		return EvidenceResult{}, err
	}
	defer rows.Close()

	var result EvidenceResult
	sessionSet := make(map[string]bool)
	for rows.Next() {
		var tradeID, sessionID string
		if err := rows.Scan(&tradeID, &sessionID); err != nil {
			return EvidenceResult{}, err
		}
		result.TradeIDs = append(result.TradeIDs, tradeID)
		if !sessionSet[sessionID] {
			sessionSet[sessionID] = true
			result.SessionIDs = append(result.SessionIDs, sessionID)
		}
	}
	return result, rows.Err()
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
