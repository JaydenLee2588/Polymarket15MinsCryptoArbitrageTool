// Package api provides the strategy engine for the Polymarket trading system.
// This file contains pure functions for strategy evaluation, decoupled from
// network I/O, enabling both live trading and backtesting.
package api

import (
	"math"
)

// StrategyInput contains all data needed to evaluate a trading decision.
// This struct is designed to be populated by either live market data or historical backtest data.
type StrategyInput struct {
	// Binance K-line data
	KlineOpen  float64 // K-line open price
	KlineClose float64 // K-line close price (current)

	// Polymarket data - supports two modes:
	// 1. Single price mode (for backtesting): Set MarketPrice directly
	// 2. Dual price mode (for live trading): Set MarketPrices map with "UP" and "DOWN" keys
	MarketPrice  float64            // Current market price for the target side (backtest mode)
	MarketPrices map[string]float64 // Prices for both sides: {"UP": x, "DOWN": y} (live mode)

	// Time information
	ElapsedSeconds  float64 // Seconds since market start
	TimeLeftSeconds float64 // Seconds until market end

	// Market metadata
	Symbol string // "BTC", "ETH", "SOL", "XRP"
}

// StrategyDecision represents the output of strategy evaluation.
type StrategyDecision struct {
	Action    string  // "BUY" or "SKIP"
	Side      string  // "UP" or "DOWN"
	Price     float64 // Execution price
	Amount    float64 // Share amount to buy
	ZScore    float64 // Calculated Z-Score
	FairPrice float64 // Calculated fair price from Sigmoid model
	PriceEdge float64 // FairPrice - MarketPrice
	Reason    string  // Explanation for the decision (for debugging/logging)
}

// EvaluateStrategy is a pure function that evaluates whether to trade based on inputs and config.
// It contains no side effects, no network calls, making it suitable for both live trading and backtesting.
func EvaluateStrategy(input StrategyInput, config StrategyConfig) StrategyDecision {
	result := StrategyDecision{
		Action: "SKIP",
		Reason: "",
	}

	// 1. Check time window
	if input.ElapsedSeconds < float64(config.TrendWindowSecStart) {
		result.Reason = "too_early"
		return result
	}
	if input.ElapsedSeconds > float64(config.TrendWindowSecEnd) {
		result.Reason = "too_late"
		return result
	}

	// 2. Calculate trend
	if input.KlineOpen == 0 {
		result.Reason = "invalid_kline"
		return result
	}
	trendPercent := (input.KlineClose - input.KlineOpen) / input.KlineOpen

	// 3. Determine direction
	var targetSide string
	var signedTrend float64

	if trendPercent > config.MinTrendPercentUp {
		targetSide = "UP"
		signedTrend = trendPercent
	} else if trendPercent < -config.MinTrendPercentDown {
		targetSide = "DOWN"
		signedTrend = -trendPercent // DOWN token needs reversed trend
	} else {
		result.Reason = "trend_too_weak"
		return result
	}

	result.Side = targetSide

	// 4. Get market price for the target side
	// Support both single price mode (backtesting) and dual price mode (live trading)
	var price float64
	if input.MarketPrices != nil {
		// Live trading mode: get price for specific side
		var ok bool
		price, ok = input.MarketPrices[targetSide]
		if !ok || price <= 0 {
			result.Reason = "no_price_for_side"
			return result
		}
	} else {
		// Backtest mode: use single MarketPrice
		price = input.MarketPrice
		if price <= 0 {
			result.Reason = "invalid_market_price"
			return result
		}
	}

	// [FIX] 无论后续是否 SKIP，都记录获取到的价格，避免日志显示 0.000
	result.Price = price

	// 5. Calculate Z-Score and Fair Price using Sigmoid model
	volatility := GetVolatilityFactor(input.Symbol, config)
	sensitivity := config.SigmoidSensitivity
	if sensitivity <= 0 {
		sensitivity = 1.5 // Default
	}

	timeLeftSec := input.TimeLeftSeconds
	if timeLeftSec < 0 {
		timeLeftSec = 0
	}

	fairPrice := CalculateSigmoidFairPrice(signedTrend, timeLeftSec, volatility, sensitivity)

	// Calculate Z-Score for tier determination
	sqrtTime := math.Sqrt(math.Max(timeLeftSec, 10.0) / 900.0)
	zScore := signedTrend / (volatility * sqrtTime)

	// DEBUG LOG
	// fmt.Printf("DEBUG: Z=%.4f (Trend=%.4f, Vol=%.4f, Time=%.0f), Tier0=%.4f\n", zScore, signedTrend, volatility, timeLeftSec, config.Tier0ZScore)

	result.ZScore = zScore
	result.FairPrice = fairPrice
	result.PriceEdge = fairPrice - price

	// 6. Check Z-Score tier threshold
	if zScore < config.Tier0ZScore {
		result.Reason = "zscore_below_tier0"
		return result
	}

	// 7. Determine required edge based on tier
	requiredEdge := config.MinEdgeTier0
	if zScore >= config.Tier4ZScore {
		requiredEdge = config.MinEdgeTier4
	} else if zScore >= config.Tier3ZScore {
		requiredEdge = config.MinEdgeTier3
	} else if zScore >= config.Tier2ZScore {
		requiredEdge = config.MinEdgeTier2
	} else if zScore >= config.Tier1ZScore {
		requiredEdge = config.MinEdgeTier1
	}

	// 8. Check if edge is sufficient
	priceEdge := fairPrice - price
	if priceEdge < requiredEdge {
		result.Reason = "edge_insufficient"
		return result
	}

	// 9. Check price range
	if price < config.Leg0SharePrice0 {
		result.Reason = "price_too_low"
		return result
	}
	if price > config.SharePriceLevel3 {
		result.Reason = "price_too_high"
		return result
	}

	// 10. Determine amount based on price tier
	var amount float64
	if price <= config.SharePriceLevel1 {
		amount = config.ShareAmount1
	} else if price <= config.SharePriceLevel2 {
		amount = config.ShareAmount2
	} else {
		amount = config.ShareAmount3
	}

	// All checks passed - return BUY decision
	result.Action = "BUY"
	result.Price = price
	result.Amount = amount
	result.Reason = "signal_valid"

	return result
}

// CalculateSigmoidFairPrice calculates the fair price using the Z-Score Sigmoid model.
//
// Formula:
//
//	Z = trend / (σ * sqrt(timeRatio))
//	FairPrice = 1 / (1 + e^(-K * Z))
//
// Parameters:
//   - trend: Trend percentage (currentClose - klineOpen) / klineOpen, signed
//   - timeLeftSec: Seconds until settlement (0 ~ 900)
//   - volatility: Volatility factor (BTC=0.0015, ETH=0.002, SOL=0.003, XRP=0.004)
//   - sensitivity: Sensitivity coefficient K (recommended 1.0 ~ 2.0)
//
// Returns:
//   - fairPrice: Fair price between 0 and 1
func CalculateSigmoidFairPrice(trend float64, timeLeftSec float64, volatility float64, sensitivity float64) float64 {
	// 1. Time normalization (minimum 10 seconds to avoid division by zero)
	timeRatio := math.Max(timeLeftSec, 10.0) / 900.0

	// 2. Construct Z-Score
	// Z = trend / (σ * sqrt(timeRatio))
	// Larger trend -> larger Z
	// Less time -> smaller sqrt(timeRatio) -> larger Z (Gamma effect)
	sqrtTime := math.Sqrt(timeRatio)
	if sqrtTime < 0.001 {
		sqrtTime = 0.001 // Prevent division by zero
	}
	zScore := trend / (volatility * sqrtTime)

	// 3. Sigmoid activation function
	// FairPrice = 1 / (1 + e^(-K * Z))
	fairPrice := 1.0 / (1.0 + math.Exp(-sensitivity*zScore))

	return fairPrice
}

// GetVolatilityFactor returns the volatility factor for a given symbol.
func GetVolatilityFactor(symbol string, config StrategyConfig) float64 {
	switch symbol {
	case "BTC":
		if config.VolatilityBTC > 0 {
			return config.VolatilityBTC
		}
		return 0.0015 // Default
	case "ETH":
		if config.VolatilityETH > 0 {
			return config.VolatilityETH
		}
		return 0.002
	case "SOL":
		if config.VolatilitySOL > 0 {
			return config.VolatilitySOL
		}
		return 0.003
	case "XRP":
		if config.VolatilityXRP > 0 {
			return config.VolatilityXRP
		}
		return 0.004
	default:
		return 0.003 // Unknown symbol, use medium volatility
	}
}
