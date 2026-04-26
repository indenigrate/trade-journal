package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/onesine/nevup-backend/internal/api/middleware"
	"github.com/onesine/nevup-backend/internal/cache"
	"github.com/onesine/nevup-backend/internal/domain"
	"github.com/onesine/nevup-backend/internal/store"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

type TradeHandler struct {
	tradeStore   *store.TradeStore
	streamClient *cache.StreamClient
	zsetClient   *cache.ZSetClient
	metricsCache *cache.MetricsCache
}

func NewTradeHandler(ts *store.TradeStore, sc *cache.StreamClient, zc *cache.ZSetClient, mc *cache.MetricsCache) *TradeHandler {
	return &TradeHandler{
		tradeStore:   ts,
		streamClient: sc,
		zsetClient:   zc,
		metricsCache: mc,
	}
}

// Create handles POST /trades.
func (h *TradeHandler) Create(w http.ResponseWriter, r *http.Request) {
	jwtSub := middleware.UserIDFromContext(r.Context())

	var input domain.TradeInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondBadRequest(w, r, "Invalid request body: "+err.Error())
		return
	}

	// Validate input
	if err := input.Validate(); err != nil {
		respondBadRequest(w, r, err.Error())
		return
	}

	// Tenancy check: JWT sub must match trade userId
	if input.UserID != jwtSub {
		respondForbidden(w, r)
		return
	}

	// Build trade
	trade := domain.Trade{
		TradeID:        input.TradeID,
		UserID:         input.UserID,
		SessionID:      input.SessionID,
		Asset:          input.Asset,
		AssetClass:     input.AssetClass,
		Direction:      input.Direction,
		EntryPrice:     input.EntryPrice,
		ExitPrice:      input.ExitPrice,
		Quantity:       input.Quantity,
		EntryAt:        input.EntryAt,
		ExitAt:         input.ExitAt,
		Status:         input.Status,
		PlanAdherence:  input.PlanAdherence,
		EmotionalState: input.EmotionalState,
		EntryRationale: input.EntryRationale,
	}

	// Compute P&L if closed
	if trade.Status == domain.StatusClosed && trade.ExitPrice != nil {
		pnl, outcome := domain.ComputePnL(trade.Direction, trade.EntryPrice, *trade.ExitPrice, trade.Quantity)
		trade.PnL = &pnl
		trade.Outcome = &outcome
	}

	// Revenge flag check (synchronous, before DB write)
	if trade.EmotionalState != nil {
		lastLossUnix, err := h.metricsCache.GetLastLoss(r.Context(), trade.UserID)
		if err == nil && lastLossUnix > 0 {
			prevExitAt := time.Unix(lastLossUnix, 0)
			if domain.IsRevenge(prevExitAt, trade.EntryAt, *trade.EmotionalState) {
				trade.RevengeFlag = true
			}
		}
	}

	// Upsert trade (idempotent)
	result, err := h.tradeStore.Upsert(r.Context(), trade)
	if err != nil {
		log.Error().Err(err).Str("tradeId", trade.TradeID).Msg("trade upsert failed")
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save trade: "+err.Error())
		return
	}

	// Async side effects (non-blocking for the response)
	// Set last_loss TTL if this is a closed losing trade
	if result.Status == domain.StatusClosed && result.Outcome != nil && *result.Outcome == domain.OutcomeLoss && result.ExitAt != nil {
		go h.metricsCache.SetLastLoss(r.Context(), result.UserID, result.ExitAt.Unix())
	}

	// Publish to Redis Stream for async pipeline
	go func() {
		eventData := map[string]interface{}{
			"tradeId":   result.TradeID,
			"userId":    result.UserID,
			"sessionId": result.SessionID,
			"status":    result.Status,
			"direction": result.Direction,
			"entryAt":   result.EntryAt.Format(time.RFC3339),
		}
		if result.Outcome != nil {
			eventData["outcome"] = *result.Outcome
		}
		if result.PnL != nil {
			eventData["pnl"] = result.PnL.String()
		}
		if result.EmotionalState != nil {
			eventData["emotionalState"] = *result.EmotionalState
		}
		if result.PlanAdherence != nil {
			eventData["planAdherence"] = *result.PlanAdherence
		}
		if result.ExitAt != nil {
			eventData["exitAt"] = result.ExitAt.Format(time.RFC3339)
		}
		eventData["revengeFlag"] = result.RevengeFlag
		h.streamClient.Add(r.Context(), "trades:events", eventData)
	}()

	// ZADD for overtrading window (fire-and-forget)
	go func() {
		nowMs := result.EntryAt.UnixMilli()
		key := "user:" + result.UserID + ":trades-ts"
		h.zsetClient.AddNX(r.Context(), key, float64(nowMs), result.TradeID)
	}()

	respondJSON(w, http.StatusOK, result)
}

// Get handles GET /trades/{tradeId}.
func (h *TradeHandler) Get(w http.ResponseWriter, r *http.Request) {
	tradeID := chi.URLParam(r, "tradeId")
	jwtSub := middleware.UserIDFromContext(r.Context())

	trade, err := h.tradeStore.GetByID(r.Context(), tradeID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respondNotFound(w, r, "Trade with the given tradeId does not exist.")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch trade")
		return
	}

	// Row-level tenancy: never 404 when found but unauthorized
	if trade.UserID != jwtSub {
		respondForbidden(w, r)
		return
	}

	respondJSON(w, http.StatusOK, trade)
}

// init registers decimal as JSON number
func init() {
	decimal.MarshalJSONWithoutQuotes = true
}
