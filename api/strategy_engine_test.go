package api

import (
	"math"
	"testing"
)

func TestCalculateSigmoidFairPrice(t *testing.T) {
	tests := []struct {
		name        string
		trend       float64
		timeLeftSec float64
		volatility  float64
		sensitivity float64
		wantMin     float64
		wantMax     float64
	}{
		{
			name:        "Zero trend returns ~0.5",
			trend:       0,
			timeLeftSec: 450, // 7.5 minutes left
			volatility:  0.0015,
			sensitivity: 1.5,
			wantMin:     0.45,
			wantMax:     0.55,
		},
		{
			name:        "Strong positive trend returns high value",
			trend:       0.005, // 0.5% up
			timeLeftSec: 300,   // 5 minutes left
			volatility:  0.0015,
			sensitivity: 1.5,
			wantMin:     0.7,
			wantMax:     1.0,
		},
		{
			name:        "Strong negative trend returns low value",
			trend:       -0.005, // 0.5% down
			timeLeftSec: 300,
			volatility:  0.0015,
			sensitivity: 1.5,
			wantMin:     0.0,
			wantMax:     0.3,
		},
		{
			name:        "Very little time left amplifies signal (Gamma effect)",
			trend:       0.002, // moderate trend
			timeLeftSec: 30,    // 30 seconds left
			volatility:  0.0015,
			sensitivity: 1.5,
			wantMin:     0.8, // Should be high due to Gamma
			wantMax:     1.0,
		},
		{
			name:        "Minimum time boundary (10 seconds)",
			trend:       0.001,
			timeLeftSec: 0, // Should be clamped to 10
			volatility:  0.0012,
			sensitivity: 2.0,
			wantMin:     0.5,
			wantMax:     1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateSigmoidFairPrice(tt.trend, tt.timeLeftSec, tt.volatility, tt.sensitivity)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculateSigmoidFairPrice() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestGetVolatilityFactor(t *testing.T) {
	config := StrategyConfig{
		VolatilityBTC: 0.002,
		VolatilityETH: 0.003,
		VolatilitySOL: 0.004,
		VolatilityXRP: 0.005,
	}

	tests := []struct {
		symbol string
		want   float64
	}{
		{"BTC", 0.002},
		{"ETH", 0.003},
		{"SOL", 0.004},
		{"XRP", 0.005},
		{"UNKNOWN", 0.003}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			got := GetVolatilityFactor(tt.symbol, config)
			if got != tt.want {
				t.Errorf("GetVolatilityFactor(%s) = %v, want %v", tt.symbol, got, tt.want)
			}
		})
	}
}

func TestGetVolatilityFactor_Defaults(t *testing.T) {
	// Empty config should return defaults
	config := StrategyConfig{}

	tests := []struct {
		symbol string
		want   float64
	}{
		{"BTC", 0.0015},
		{"ETH", 0.002},
		{"SOL", 0.003},
		{"XRP", 0.004},
	}

	for _, tt := range tests {
		t.Run(tt.symbol+"_default", func(t *testing.T) {
			got := GetVolatilityFactor(tt.symbol, config)
			if got != tt.want {
				t.Errorf("GetVolatilityFactor(%s) default = %v, want %v", tt.symbol, got, tt.want)
			}
		})
	}
}

func TestEvaluateStrategy_TimeWindow(t *testing.T) {
	config := StrategyConfig{
		TrendWindowSecStart: 300,
		TrendWindowSecEnd:   600,
	}

	tests := []struct {
		name           string
		elapsedSeconds float64
		wantReason     string
	}{
		{"Too early", 100, "too_early"},
		{"Too late", 700, "too_late"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := StrategyInput{
				ElapsedSeconds: tt.elapsedSeconds,
				KlineOpen:      100,
				KlineClose:     101,
			}
			result := EvaluateStrategy(input, config)
			if result.Action != "SKIP" {
				t.Errorf("Expected SKIP, got %s", result.Action)
			}
			if result.Reason != tt.wantReason {
				t.Errorf("Expected reason %s, got %s", tt.wantReason, result.Reason)
			}
		})
	}
}

func TestEvaluateStrategy_TrendDirection(t *testing.T) {
	config := StrategyConfig{
		TrendWindowSecStart: 300,
		TrendWindowSecEnd:   600,
		MinTrendPercentUp:   0.001, // 0.1%
		MinTrendPercentDown: 0.001,
		Tier0ZScore:         0.5,
		MinEdgeTier0:        0.01,
		Leg0SharePrice0:     0.5,
		SharePriceLevel1:    0.7,
		SharePriceLevel2:    0.85,
		SharePriceLevel3:    0.97,
		ShareAmount1:        50,
		ShareAmount2:        40,
		ShareAmount3:        30,
		SigmoidSensitivity:  1.8,
		VolatilityBTC:       0.0012,
	}

	tests := []struct {
		name       string
		klineOpen  float64
		klineClose float64
		wantSide   string
	}{
		{"UP trend", 100, 100.5, "UP"},    // +0.5%
		{"DOWN trend", 100, 99.5, "DOWN"}, // -0.5%
		{"Weak trend", 100, 100.05, ""},   // +0.05% (below threshold)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := StrategyInput{
				ElapsedSeconds:  400,
				TimeLeftSeconds: 500,
				KlineOpen:       tt.klineOpen,
				KlineClose:      tt.klineClose,
				MarketPrice:     0.6,
				Symbol:          "BTC",
			}
			result := EvaluateStrategy(input, config)
			if tt.wantSide == "" {
				// Expecting SKIP for weak trend
				if result.Action != "SKIP" || result.Reason != "trend_too_weak" {
					t.Errorf("Expected SKIP with trend_too_weak, got Action=%s Reason=%s", result.Action, result.Reason)
				}
			} else {
				if result.Side != tt.wantSide {
					t.Errorf("Expected side %s, got %s", tt.wantSide, result.Side)
				}
			}
		})
	}
}

func TestEvaluateStrategy_ZScoreTiers(t *testing.T) {
	config := StrategyConfig{
		TrendWindowSecStart: 300,
		TrendWindowSecEnd:   600,
		MinTrendPercentUp:   0.001,
		MinTrendPercentDown: 0.001,
		Tier0ZScore:         1.0,
		Tier1ZScore:         1.5,
		Tier2ZScore:         2.0,
		Tier3ZScore:         2.5,
		Tier4ZScore:         3.0,
		MinEdgeTier0:        0.10,
		MinEdgeTier1:        0.05,
		MinEdgeTier2:        0.03,
		MinEdgeTier3:        0.01,
		MinEdgeTier4:        0.00,
		Leg0SharePrice0:     0.5,
		SharePriceLevel1:    0.7,
		SharePriceLevel2:    0.85,
		SharePriceLevel3:    0.97,
		ShareAmount1:        50,
		ShareAmount2:        40,
		ShareAmount3:        30,
		SigmoidSensitivity:  1.8,
		VolatilityBTC:       0.0012,
	}

	// Test that Z-Score below Tier0 is rejected
	t.Run("ZScore below Tier0", func(t *testing.T) {
		input := StrategyInput{
			ElapsedSeconds:  400,
			TimeLeftSeconds: 900,
			KlineOpen:       100,
			KlineClose:      100.11, // Trend 0.11% (passes min trend > 0.1%) -> Z = 0.916 < 1.0
			MarketPrice:     0.6,
			Symbol:          "BTC",
		}
		result := EvaluateStrategy(input, config)
		if result.Action != "SKIP" || result.Reason != "zscore_below_tier0" {
			t.Errorf("Expected SKIP with zscore_below_tier0, got Action=%s Reason=%s ZScore=%.4f (Tier0=%.2f)",
				result.Action, result.Reason, result.ZScore, config.Tier0ZScore)
		}
	})
}

func TestEvaluateStrategy_PriceRange(t *testing.T) {
	config := StrategyConfig{
		TrendWindowSecStart: 300,
		TrendWindowSecEnd:   600,
		MinTrendPercentUp:   0.001,
		MinTrendPercentDown: 0.001,
		Tier0ZScore:         0.1, // Low threshold for testing
		MinEdgeTier0:        -1,  // Always pass edge check
		Leg0SharePrice0:     0.5,
		SharePriceLevel1:    0.7,
		SharePriceLevel2:    0.85,
		SharePriceLevel3:    0.97,
		ShareAmount1:        50,
		ShareAmount2:        40,
		ShareAmount3:        30,
		SigmoidSensitivity:  1.8,
		VolatilityBTC:       0.0012,
	}

	tests := []struct {
		name       string
		price      float64
		wantReason string
	}{
		{"Price too low", 0.4, "price_too_low"},
		{"Price too high", 0.99, "price_too_high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := StrategyInput{
				ElapsedSeconds:  400,
				TimeLeftSeconds: 500,
				KlineOpen:       100,
				KlineClose:      100.5,
				MarketPrice:     tt.price,
				Symbol:          "BTC",
			}
			result := EvaluateStrategy(input, config)
			if result.Action != "SKIP" || result.Reason != tt.wantReason {
				t.Errorf("Expected SKIP with %s, got Action=%s Reason=%s", tt.wantReason, result.Action, result.Reason)
			}
		})
	}
}

func TestEvaluateStrategy_AmountTiers(t *testing.T) {
	config := StrategyConfig{
		TrendWindowSecStart: 300,
		TrendWindowSecEnd:   600,
		MinTrendPercentUp:   0.001,
		MinTrendPercentDown: 0.001,
		Tier0ZScore:         0.1,
		MinEdgeTier0:        -1, // Always pass edge check
		Leg0SharePrice0:     0.5,
		SharePriceLevel1:    0.7,
		SharePriceLevel2:    0.85,
		SharePriceLevel3:    0.97,
		ShareAmount1:        50,
		ShareAmount2:        40,
		ShareAmount3:        30,
		SigmoidSensitivity:  1.8,
		VolatilityBTC:       0.0012,
	}

	tests := []struct {
		name       string
		price      float64
		wantAmount float64
	}{
		{"Tier 1 price", 0.65, 50},
		{"Tier 2 price", 0.75, 40},
		{"Tier 3 price", 0.90, 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := StrategyInput{
				ElapsedSeconds:  400,
				TimeLeftSeconds: 500,
				KlineOpen:       100,
				KlineClose:      100.5,
				MarketPrice:     tt.price,
				Symbol:          "BTC",
			}
			result := EvaluateStrategy(input, config)
			if result.Action != "BUY" {
				t.Errorf("Expected BUY, got %s (reason: %s)", result.Action, result.Reason)
				return
			}
			if math.Abs(result.Amount-tt.wantAmount) > 0.01 {
				t.Errorf("Expected amount %.0f, got %.0f", tt.wantAmount, result.Amount)
			}
		})
	}
}

func TestEvaluateStrategy_FullBuySignal(t *testing.T) {
	config := StrategyConfig{
		TrendWindowSecStart: 300,
		TrendWindowSecEnd:   600,
		MinTrendPercentUp:   0.002,
		MinTrendPercentDown: 0.002,
		Tier0ZScore:         1.0,
		Tier1ZScore:         1.5,
		MinEdgeTier0:        0.05,
		MinEdgeTier1:        0.02,
		Leg0SharePrice0:     0.5,
		SharePriceLevel1:    0.7,
		SharePriceLevel2:    0.85,
		SharePriceLevel3:    0.97,
		ShareAmount1:        50,
		ShareAmount2:        40,
		ShareAmount3:        30,
		SigmoidSensitivity:  2.0,
		VolatilityBTC:       0.0012,
	}

	// Strong UP trend with favorable market price
	input := StrategyInput{
		ElapsedSeconds:  400,
		TimeLeftSeconds: 400, // 400s left -> sqrt(400/900) ≈ 0.67
		KlineOpen:       100,
		KlineClose:      100.5, // +0.5% trend
		MarketPrice:     0.65,  // Low price
		Symbol:          "BTC",
	}

	result := EvaluateStrategy(input, config)

	t.Logf("Result: Action=%s Side=%s ZScore=%.2f FairPrice=%.3f Edge=%.3f Reason=%s",
		result.Action, result.Side, result.ZScore, result.FairPrice, result.PriceEdge, result.Reason)

	if result.Action != "BUY" {
		t.Errorf("Expected BUY signal, got %s (reason: %s)", result.Action, result.Reason)
	}
	if result.Side != "UP" {
		t.Errorf("Expected UP side, got %s", result.Side)
	}
	if result.Amount != 50 {
		t.Errorf("Expected amount 50, got %.0f", result.Amount)
	}
}
