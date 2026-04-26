package domain

// AssetClass enum values
const (
	AssetClassEquity = "equity"
	AssetClassCrypto = "crypto"
	AssetClassForex  = "forex"
)

// Direction enum values
const (
	DirectionLong  = "long"
	DirectionShort = "short"
)

// Status enum values
const (
	StatusOpen      = "open"
	StatusClosed    = "closed"
	StatusCancelled = "cancelled"
)

// EmotionalState enum values
const (
	EmotionCalm    = "calm"
	EmotionAnxious = "anxious"
	EmotionGreedy  = "greedy"
	EmotionFearful = "fearful"
	EmotionNeutral = "neutral"
)

// Outcome enum values
const (
	OutcomeWin  = "win"
	OutcomeLoss = "loss"
)

var validAssetClasses = map[string]bool{
	AssetClassEquity: true,
	AssetClassCrypto: true,
	AssetClassForex:  true,
}

var validDirections = map[string]bool{
	DirectionLong:  true,
	DirectionShort: true,
}

var validStatuses = map[string]bool{
	StatusOpen:      true,
	StatusClosed:    true,
	StatusCancelled: true,
}

var validEmotions = map[string]bool{
	EmotionCalm:    true,
	EmotionAnxious: true,
	EmotionGreedy:  true,
	EmotionFearful: true,
	EmotionNeutral: true,
}

func IsValidAssetClass(s string) bool   { return validAssetClasses[s] }
func IsValidDirection(s string) bool     { return validDirections[s] }
func IsValidStatus(s string) bool        { return validStatuses[s] }
func IsValidEmotionalState(s string) bool { return validEmotions[s] }
