package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// MetricsBucket represents one time bucket of behavioral metrics.
type MetricsBucket struct {
	Bucket            time.Time       `json:"bucket"`
	TradeCount        int             `json:"tradeCount"`
	WinCount          int             `json:"winCount"`
	LossCount         int             `json:"lossCount"`
	PnL               decimal.Decimal `json:"pnl"`
	AvgPlanAdherence  *float64        `json:"avgPlanAdherence"`
	SessionTiltIndex  *float64        `json:"sessionTiltIndex"`
	RevengeCount      int             `json:"revengeCount"`
	OvertradingEvents int             `json:"overtradingEvents"`
}

// EmotionWinRate represents win/loss count per emotional state.
type EmotionWinRate struct {
	EmotionalState string  `json:"emotionalState"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	WinRate        float64 `json:"winRate"`
}

type MetricsStore struct {
	pool *pgxpool.Pool
}

func NewMetricsStore(pool *pgxpool.Pool) *MetricsStore {
	return &MetricsStore{pool: pool}
}

// QueryRange returns bucketed behavioral metrics for a user in a date range.
func (s *MetricsStore) QueryRange(ctx context.Context, userID string, from, to time.Time, interval string) ([]MetricsBucket, error) {
	const sql = `SELECT
		time_bucket($1::interval, bucket) AS b,
		SUM(trade_count)               AS trade_count,
		SUM(win_count)                 AS win_count,
		SUM(loss_count)                AS loss_count,
		SUM(total_pnl)                 AS pnl,
		AVG(avg_plan_adherence)        AS avg_plan_adherence,
		AVG(session_tilt_index)        AS session_tilt_index,
		SUM(revenge_count)             AS revenge_count,
		SUM(overtrading_events)        AS overtrading_events
	FROM behavioral_metrics
	WHERE user_id = $2
	  AND bucket >= $3
	  AND bucket <= $4
	GROUP BY b
	ORDER BY b`

	rows, err := s.pool.Query(ctx, sql, interval, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []MetricsBucket
	for rows.Next() {
		var b MetricsBucket
		err := rows.Scan(
			&b.Bucket, &b.TradeCount, &b.WinCount, &b.LossCount,
			&b.PnL, &b.AvgPlanAdherence, &b.SessionTiltIndex,
			&b.RevengeCount, &b.OvertradingEvents,
		)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

// QueryEmotionWinRates returns per-emotional-state win/loss counts in a date range.
func (s *MetricsStore) QueryEmotionWinRates(ctx context.Context, userID string, from, to time.Time) ([]EmotionWinRate, error) {
	const sql = `SELECT emotional_state, SUM(wins) AS wins, SUM(losses) AS losses
	FROM emotion_win_rates
	WHERE user_id = $1
	  AND date_bucket >= $2::date
	  AND date_bucket <= $3::date
	GROUP BY emotional_state`

	rows, err := s.pool.Query(ctx, sql, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []EmotionWinRate
	for rows.Next() {
		var r EmotionWinRate
		err := rows.Scan(&r.EmotionalState, &r.Wins, &r.Losses)
		if err != nil {
			return nil, err
		}
		total := r.Wins + r.Losses
		if total > 0 {
			r.WinRate = float64(r.Wins) / float64(total)
		}
		rates = append(rates, r)
	}
	return rates, rows.Err()
}

// UpsertBucket upserts a single behavioral metrics bucket.
func (s *MetricsStore) UpsertBucket(ctx context.Context, userID string, bucket time.Time,
	tradeCount, winCount, lossCount int, totalPnl decimal.Decimal,
	avgPlanAdherence *float64, sessionTiltIndex *float64,
	revengeCount, overtradingEvents int) error {

	const sql = `INSERT INTO behavioral_metrics (
		bucket, user_id, trade_count, win_count, loss_count, total_pnl,
		avg_plan_adherence, session_tilt_index, revenge_count, overtrading_events
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (bucket, user_id) DO UPDATE SET
		trade_count = behavioral_metrics.trade_count + EXCLUDED.trade_count,
		win_count = behavioral_metrics.win_count + EXCLUDED.win_count,
		loss_count = behavioral_metrics.loss_count + EXCLUDED.loss_count,
		total_pnl = behavioral_metrics.total_pnl + EXCLUDED.total_pnl,
		avg_plan_adherence = COALESCE(EXCLUDED.avg_plan_adherence, behavioral_metrics.avg_plan_adherence),
		session_tilt_index = COALESCE(EXCLUDED.session_tilt_index, behavioral_metrics.session_tilt_index),
		revenge_count = behavioral_metrics.revenge_count + EXCLUDED.revenge_count,
		overtrading_events = behavioral_metrics.overtrading_events + EXCLUDED.overtrading_events`

	_, err := s.pool.Exec(ctx, sql,
		bucket, userID, tradeCount, winCount, lossCount, totalPnl,
		avgPlanAdherence, sessionTiltIndex, revengeCount, overtradingEvents,
	)
	return err
}

// UpsertEmotionWinRate upserts emotion win rate data.
func (s *MetricsStore) UpsertEmotionWinRate(ctx context.Context, userID, emotionalState string, dateBucket time.Time, wins, losses int) error {
	const sql = `INSERT INTO emotion_win_rates (user_id, emotional_state, date_bucket, wins, losses)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (user_id, emotional_state, date_bucket)
	DO UPDATE SET
		wins = emotion_win_rates.wins + EXCLUDED.wins,
		losses = emotion_win_rates.losses + EXCLUDED.losses`

	_, err := s.pool.Exec(ctx, sql, userID, emotionalState, dateBucket, wins, losses)
	return err
}

// InsertOvertradingEvent persists an overtrading event.
func (s *MetricsStore) InsertOvertradingEvent(ctx context.Context, userID string, windowStart time.Time, tradeCount int) error {
	const sql = `INSERT INTO overtrading_events (user_id, window_start, trade_count) VALUES ($1, $2, $3)`
	_, err := s.pool.Exec(ctx, sql, userID, windowStart, tradeCount)
	return err
}

// CountOvertradingEvents counts overtrading events for a user in a date range.
func (s *MetricsStore) CountOvertradingEvents(ctx context.Context, userID string, from, to time.Time) (int, error) {
	const sql = `SELECT COUNT(*) FROM overtrading_events WHERE user_id = $1 AND emitted_at >= $2 AND emitted_at <= $3`
	var count int
	err := s.pool.QueryRow(ctx, sql, userID, from, to).Scan(&count)
	return count, err
}

// CountRevengeTrades counts revenge-flagged trades for a user in a date range.
func (s *MetricsStore) CountRevengeTrades(ctx context.Context, userID string, from, to time.Time) (int, error) {
	const sql = `SELECT COUNT(*) FROM trades WHERE user_id = $1 AND revenge_flag = true AND entry_at >= $2 AND entry_at <= $3`
	var count int
	err := s.pool.QueryRow(ctx, sql, userID, from, to).Scan(&count)
	return count, err
}
