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

// Session represents a trading session.
type Session struct {
	SessionID string    `json:"sessionId"`
	UserID    string    `json:"userId"`
	StartedAt time.Time `json:"date"`
	Notes     *string   `json:"notes"`
}

// SessionSummary is the response shape for GET /sessions/:id.
type SessionSummary struct {
	SessionID  string          `json:"sessionId"`
	UserID     string          `json:"userId"`
	Date       time.Time       `json:"date"`
	Notes      *string         `json:"notes"`
	TradeCount int             `json:"tradeCount"`
	WinRate    float64         `json:"winRate"`
	TotalPnl   decimal.Decimal `json:"totalPnl"`
	Trades     []domain.Trade  `json:"trades"`
}

// Debrief represents a session debrief.
type Debrief struct {
	DebriefID           string    `json:"debriefId"`
	SessionID           string    `json:"sessionId"`
	OverallMood         string    `json:"overallMood"`
	KeyMistake          *string   `json:"keyMistake"`
	KeyLesson           *string   `json:"keyLesson"`
	PlanAdherenceRating *int      `json:"planAdherenceRating"`
	WillReviewTomorrow  bool      `json:"willReviewTomorrow"`
	SavedAt             time.Time `json:"savedAt"`
}

type SessionStore struct {
	pool *pgxpool.Pool
}

func NewSessionStore(pool *pgxpool.Pool) *SessionStore {
	return &SessionStore{pool: pool}
}

// GetSession fetches a session by ID.
func (s *SessionStore) GetSession(ctx context.Context, sessionID string) (Session, error) {
	const sql = `SELECT session_id, user_id, started_at, notes FROM sessions WHERE session_id = $1`
	var sess Session
	err := s.pool.QueryRow(ctx, sql, sessionID).Scan(&sess.SessionID, &sess.UserID, &sess.StartedAt, &sess.Notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, domain.ErrNotFound
	}
	return sess, err
}

// GetWithTrades returns a session summary with all trades.
func (s *SessionStore) GetWithTrades(ctx context.Context, sessionID string) (SessionSummary, error) {
	sess, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}

	tradeStore := NewTradeStore(s.pool)
	trades, err := tradeStore.ListBySession(ctx, sessionID)
	if err != nil {
		return SessionSummary{}, err
	}

	summary := SessionSummary{
		SessionID:  sess.SessionID,
		UserID:     sess.UserID,
		Date:       sess.StartedAt,
		Notes:      sess.Notes,
		TradeCount: len(trades),
		TotalPnl:   decimal.Zero,
		Trades:     trades,
	}

	winCount := 0
	for _, t := range trades {
		if t.PnL != nil {
			summary.TotalPnl = summary.TotalPnl.Add(*t.PnL)
		}
		if t.Outcome != nil && *t.Outcome == domain.OutcomeWin {
			winCount++
		}
	}

	if len(trades) > 0 {
		summary.WinRate = float64(winCount) / float64(len(trades))
	}

	return summary, nil
}

// InsertDebrief persists a debrief and returns the result.
func (s *SessionStore) InsertDebrief(ctx context.Context, sessionID string, d Debrief) (Debrief, error) {
	const sql = `INSERT INTO debriefs (
		session_id, overall_mood, key_mistake, key_lesson,
		plan_adherence_rating, will_review_tomorrow
	) VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING debrief_id, session_id, saved_at`

	var result Debrief
	err := s.pool.QueryRow(ctx, sql,
		sessionID, d.OverallMood, d.KeyMistake, d.KeyLesson,
		d.PlanAdherenceRating, d.WillReviewTomorrow,
	).Scan(&result.DebriefID, &result.SessionID, &result.SavedAt)

	return result, err
}
