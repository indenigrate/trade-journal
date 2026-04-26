package pipeline

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/onesine/nevup-backend/internal/cache"
	"github.com/onesine/nevup-backend/internal/domain"
	"github.com/onesine/nevup-backend/internal/store"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

const (
	StreamName   = "trades:events"
	GroupName    = "nevup-analytics"
	ConsumerName = "worker-1"
)

// Worker processes trade events from Redis Streams.
type Worker struct {
	streamClient *cache.StreamClient
	zsetClient   *cache.ZSetClient
	metricsStore *store.MetricsStore
	tradeStore   *store.TradeStore
	metricsCache *cache.MetricsCache
}

func NewWorker(sc *cache.StreamClient, zc *cache.ZSetClient, ms *store.MetricsStore, ts *store.TradeStore, mc *cache.MetricsCache) *Worker {
	return &Worker{
		streamClient: sc,
		zsetClient:   zc,
		metricsStore: ms,
		tradeStore:   ts,
		metricsCache: mc,
	}
}

// Run starts the consumer group loop.
func (w *Worker) Run(ctx context.Context) error {
	// Create consumer group (idempotent)
	if err := w.streamClient.CreateGroup(ctx, StreamName, GroupName, "0"); err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	log.Info().Msg("pipeline worker started, listening on " + StreamName)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("pipeline worker shutting down")
			return nil
		default:
		}

		streams, err := w.streamClient.ReadGroup(ctx, GroupName, ConsumerName, StreamName, 10, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Error().Err(err).Msg("XREAD failed")
			time.Sleep(1 * time.Second)
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := w.processMessage(ctx, msg.Values); err != nil {
					log.Error().Err(err).Str("msgId", msg.ID).Msg("failed to process message")
				}

				// Acknowledge
				if err := w.streamClient.Ack(ctx, StreamName, GroupName, msg.ID); err != nil {
					log.Error().Err(err).Str("msgId", msg.ID).Msg("failed to ACK message")
				}
			}
		}
	}
}

func (w *Worker) processMessage(ctx context.Context, values map[string]interface{}) error {
	tradeID := str(values, "tradeId")
	userID := str(values, "userId")
	sessionID := str(values, "sessionId")
	status := str(values, "status")
	outcome := str(values, "outcome")
	pnlStr := str(values, "pnl")
	emotionalState := str(values, "emotionalState")
	planAdherenceStr := str(values, "planAdherence")
	entryAtStr := str(values, "entryAt")
	revengeFlagStr := str(values, "revengeFlag")

	if status != "closed" {
		return nil // Only process closed trades
	}

	entryAt, _ := time.Parse(time.RFC3339, entryAtStr)
	bucket := entryAt.Truncate(time.Hour)

	// 1. Update behavioral_metrics bucket
	tradeCount := 1
	winCount := 0
	lossCount := 0
	if outcome == domain.OutcomeWin {
		winCount = 1
	} else {
		lossCount = 1
	}

	pnl := decimal.Zero
	if pnlStr != "" {
		pnl, _ = decimal.NewFromString(pnlStr)
	}

	revengeCount := 0
	if revengeFlagStr == "true" || revengeFlagStr == "1" {
		revengeCount = 1
	}

	if err := w.metricsStore.UpsertBucket(ctx, userID, bucket,
		tradeCount, winCount, lossCount, pnl,
		nil, nil, revengeCount, 0,
	); err != nil {
		log.Error().Err(err).Str("tradeId", tradeID).Msg("failed to upsert metrics bucket")
	}

	// 2. Rolling plan adherence
	if planAdherenceStr != "" {
		ratings, err := w.tradeStore.GetLastPlanAdherenceRatings(ctx, userID, 10)
		if err == nil && len(ratings) > 0 {
			avg := domain.RollingAvg(ratings)
			if err := w.metricsStore.UpsertBucket(ctx, userID, bucket,
				0, 0, 0, decimal.Zero, &avg, nil, 0, 0,
			); err != nil {
				log.Error().Err(err).Msg("failed to upsert plan adherence")
			}
		}
	}

	// 3. Session tilt index
	if sessionID != "" {
		lossFollow, total, err := w.tradeStore.GetSessionTiltData(ctx, sessionID)
		if err == nil {
			tilt := domain.SessionTiltIndex(lossFollow, total)
			if err := w.metricsStore.UpsertBucket(ctx, userID, bucket,
				0, 0, 0, decimal.Zero, nil, &tilt, 0, 0,
			); err != nil {
				log.Error().Err(err).Msg("failed to upsert session tilt")
			}
		}
	}

	// 4. Emotion win rates
	if emotionalState != "" {
		wins, losses := 0, 0
		if outcome == domain.OutcomeWin {
			wins = 1
		} else {
			losses = 1
		}
		dateBucket := entryAt.Truncate(24 * time.Hour)
		if err := w.metricsStore.UpsertEmotionWinRate(ctx, userID, emotionalState, dateBucket, wins, losses); err != nil {
			log.Error().Err(err).Msg("failed to upsert emotion win rate")
		}
	}

	// 5. Overtrading detection
	nowMs := entryAt.UnixMilli()
	key := "user:" + userID + ":trades-ts"
	count, err := w.zsetClient.OvertradingCheck(ctx, key, nowMs, tradeID, domain.WindowDurationMs)
	if err != nil {
		log.Error().Err(err).Msg("overtrading check failed")
	} else if count > int64(domain.MaxTradesPerWindow) {
		windowStart := time.UnixMilli(nowMs - domain.WindowDurationMs)
		if err := w.metricsStore.InsertOvertradingEvent(ctx, userID, windowStart, int(count)); err != nil {
			log.Error().Err(err).Msg("failed to insert overtrading event")
		}
	}

	// Invalidate metrics cache for user
	_ = w.metricsCache.InvalidateUser(ctx, userID)

	log.Debug().
		Str("tradeId", tradeID).
		Str("userId", userID).
		Str("outcome", outcome).
		Int("planAdherence", parseInt(planAdherenceStr)).
		Msg("processed trade event")

	return nil
}

func str(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
