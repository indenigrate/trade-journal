package domain

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Trade represents the canonical trade schema.
// JSON tags use camelCase to match the OpenAPI contract exactly.
type Trade struct {
	TradeID        string           `json:"tradeId"`
	UserID         string           `json:"userId"`
	SessionID      string           `json:"sessionId"`
	Asset          string           `json:"asset"`
	AssetClass     string           `json:"assetClass"`
	Direction      string           `json:"direction"`
	EntryPrice     decimal.Decimal  `json:"entryPrice"`
	ExitPrice      *decimal.Decimal `json:"exitPrice"`
	Quantity       decimal.Decimal  `json:"quantity"`
	EntryAt        time.Time        `json:"entryAt"`
	ExitAt         *time.Time       `json:"exitAt"`
	Status         string           `json:"status"`
	PlanAdherence  *int             `json:"planAdherence"`
	EmotionalState *string          `json:"emotionalState"`
	EntryRationale *string          `json:"entryRationale"`
	Outcome        *string          `json:"outcome"`
	PnL            *decimal.Decimal `json:"pnl"`
	RevengeFlag    bool             `json:"revengeFlag"`
	CreatedAt      time.Time        `json:"createdAt"`
	UpdatedAt      time.Time        `json:"updatedAt"`
}

// TradeInput represents the client-submitted trade payload.
type TradeInput struct {
	TradeID        string           `json:"tradeId"`
	UserID         string           `json:"userId"`
	SessionID      string           `json:"sessionId"`
	Asset          string           `json:"asset"`
	AssetClass     string           `json:"assetClass"`
	Direction      string           `json:"direction"`
	EntryPrice     decimal.Decimal  `json:"entryPrice"`
	ExitPrice      *decimal.Decimal `json:"exitPrice"`
	Quantity       decimal.Decimal  `json:"quantity"`
	EntryAt        time.Time        `json:"entryAt"`
	ExitAt         *time.Time       `json:"exitAt"`
	Status         string           `json:"status"`
	PlanAdherence  *int             `json:"planAdherence"`
	EmotionalState *string          `json:"emotionalState"`
	EntryRationale *string          `json:"entryRationale"`
}

// Validate checks required fields and enum values.
func (t *TradeInput) Validate() error {
	if t.TradeID == "" {
		return fmt.Errorf("%w: tradeId is required", ErrBadRequest)
	}
	if t.UserID == "" {
		return fmt.Errorf("%w: userId is required", ErrBadRequest)
	}
	if t.SessionID == "" {
		return fmt.Errorf("%w: sessionId is required", ErrBadRequest)
	}
	if t.Asset == "" {
		return fmt.Errorf("%w: asset is required", ErrBadRequest)
	}
	if !IsValidAssetClass(t.AssetClass) {
		return fmt.Errorf("%w: invalid assetClass: %s", ErrBadRequest, t.AssetClass)
	}
	if !IsValidDirection(t.Direction) {
		return fmt.Errorf("%w: invalid direction: %s", ErrBadRequest, t.Direction)
	}
	if t.EntryPrice.IsZero() {
		return fmt.Errorf("%w: entryPrice is required", ErrBadRequest)
	}
	if t.Quantity.IsZero() {
		return fmt.Errorf("%w: quantity is required", ErrBadRequest)
	}
	if t.EntryAt.IsZero() {
		return fmt.Errorf("%w: entryAt is required", ErrBadRequest)
	}
	if !IsValidStatus(t.Status) {
		return fmt.Errorf("%w: invalid status: %s", ErrBadRequest, t.Status)
	}
	if t.PlanAdherence != nil && (*t.PlanAdherence < 1 || *t.PlanAdherence > 5) {
		return fmt.Errorf("%w: planAdherence must be between 1 and 5", ErrBadRequest)
	}
	if t.EmotionalState != nil && !IsValidEmotionalState(*t.EmotionalState) {
		return fmt.Errorf("%w: invalid emotionalState: %s", ErrBadRequest, *t.EmotionalState)
	}
	if t.EntryRationale != nil && len(*t.EntryRationale) > 500 {
		return fmt.Errorf("%w: entryRationale must be ≤ 500 characters", ErrBadRequest)
	}
	return nil
}

// ComputePnL computes P&L and outcome for a closed trade.
// Long trade:  pnl = (exitPrice - entryPrice) × quantity
// Short trade: pnl = (entryPrice - exitPrice) × quantity
func ComputePnL(direction string, entryPrice, exitPrice, quantity decimal.Decimal) (pnl decimal.Decimal, outcome string) {
	switch direction {
	case DirectionLong:
		pnl = exitPrice.Sub(entryPrice).Mul(quantity)
	case DirectionShort:
		pnl = entryPrice.Sub(exitPrice).Mul(quantity)
	}
	if pnl.IsPositive() {
		outcome = OutcomeWin
	} else {
		outcome = OutcomeLoss
	}
	return pnl, outcome
}
