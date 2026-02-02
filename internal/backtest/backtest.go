package backtest

import (
	"Polymarket15MinsCryptoArbitrageTool/api"
	"Polymarket15MinsCryptoArbitrageTool/internal/datarecorder"
	"fmt"
	"math"
	"sort"
)

// BacktestConfig Configuration for backtesting
type BacktestConfig struct {
	StrategyConfig api.StrategyConfig
	Klines         []datarecorder.KlineRecord
	OrderBooks     []datarecorder.OrderBookRecord
	TokenIDMap     map[string]string // "UP" -> TokenID, "DOWN" -> TokenID
	InitialCapital float64
	MakerFeeRate   float64
	TakerFeeRate   float64
}

// BacktestResult Result of backtesting
type BacktestResult struct {
	TotalPnL       float64
	TradeCount     int
	WinCount       int
	LossCount      int
	Volume         float64
	MaxDrawdown    float64
	SharpeRatio    float64
	Trades         []TradeRecord
	SkipReasons    map[string]int // Count of skip reasons for debugging
	TotalDecisions int            // Total times EvaluateStrategy was called
}

type TradeRecord struct {
	Timestamp int64
	Symbol    string
	Side      string // UP/DOWN
	Action    string // BUY/SELL
	Price     float64
	Amount    float64
	PnL       float64
	ExitPrice float64
	FairPrice float64
	ZScore    float64
	Reason    string
}

// RunBacktest runs the strategy against historical data
// Uses MarketPrices dual-price mode for consistency with live trading
func RunBacktest(config BacktestConfig) (*BacktestResult, error) {
	if len(config.Klines) == 0 {
		return nil, fmt.Errorf("no kline data")
	}

	result := &BacktestResult{
		TotalPnL:    0,
		Trades:      make([]TradeRecord, 0),
		SkipReasons: make(map[string]int),
	}

	// 1. Sort data by timestamp
	sort.Slice(config.Klines, func(i, j int) bool {
		return config.Klines[i].Timestamp < config.Klines[j].Timestamp
	})
	sort.Slice(config.OrderBooks, func(i, j int) bool {
		return config.OrderBooks[i].Timestamp < config.OrderBooks[j].Timestamp
	})

	// 2. Build side to TokenID mapping
	sideToTokenID := config.TokenIDMap

	// 3. Group Klines by 15-minute market windows
	// Each market window starts at timestamp aligned to 15 minutes
	const marketDuration = int64(900 * 1000) // 900 seconds in milliseconds

	if len(config.Klines) < 2 {
		return result, nil
	}

	// Process each kline
	var currentMarketStart int64 = -1
	var hasPosition bool = false

	for _, kline := range config.Klines {
		// Detect market window boundaries
		// Assuming kline.Timestamp is the K-line start time in milliseconds
		windowStart := (kline.Timestamp / marketDuration) * marketDuration

		if windowStart != currentMarketStart {
			// New market window - reset position
			currentMarketStart = windowStart
			hasPosition = false
		}

		// Skip if already have a position in this window
		if hasPosition {
			continue
		}

		elapsedMs := kline.Timestamp - currentMarketStart
		elapsedSec := float64(elapsedMs) / 1000.0
		timeLeftSec := 900.0 - elapsedSec
		if timeLeftSec < 0 {
			timeLeftSec = 0
		}

		// Build MarketPrices map from OrderBooks
		// Find OrderBooks near this timestamp for both UP and DOWN tokens
		marketPrices := make(map[string]float64)

		for side, tokenID := range sideToTokenID {
			// Find the closest OrderBook for this tokenID
			price := findBestPrice(config.OrderBooks, tokenID, kline.Timestamp, 10000) // 10 second window
			if price > 0 {
				marketPrices[side] = price
			}
		}

		// Skip if we don't have prices for both sides (need both UP and DOWN for strategy)
		if len(marketPrices) < 2 {
			result.SkipReasons["no_orderbook_data"]++
			continue
		}

		// Prepare input using new MarketPrices mode
		input := api.StrategyInput{
			KlineOpen:       kline.Open,
			KlineClose:      kline.Close,
			MarketPrices:    marketPrices, // Use dual-price mode
			ElapsedSeconds:  elapsedSec,
			TimeLeftSeconds: timeLeftSec,
			Symbol:          extractSymbolFromKline(kline.Symbol),
		}

		// Evaluate strategy
		decision := api.EvaluateStrategy(input, config.StrategyConfig)
		result.TotalDecisions++

		if decision.Action == "SKIP" {
			result.SkipReasons[decision.Reason]++
			continue
		}

		if decision.Action == "BUY" {
			// Record Trade
			trade := TradeRecord{
				Timestamp: kline.Timestamp,
				Symbol:    kline.Symbol,
				Side:      decision.Side,
				Action:    "BUY",
				Price:     decision.Price,
				Amount:    decision.Amount,
				FairPrice: decision.FairPrice,
				ZScore:    decision.ZScore,
				Reason:    decision.Reason,
			}

			// Calculate theoretical PnL based on edge
			// In reality, PnL depends on settlement outcome
			// For backtesting, we use the price edge as theoretical profit
			pnl := decision.PriceEdge * decision.Amount
			trade.PnL = pnl
			trade.ExitPrice = decision.FairPrice

			result.Trades = append(result.Trades, trade)
			result.TradeCount++
			result.Volume += trade.Price * trade.Amount
			result.TotalPnL += pnl

			if pnl > 0 {
				result.WinCount++
			} else {
				result.LossCount++
			}

			hasPosition = true
		}
	}

	// Calculate additional metrics
	if result.TradeCount > 0 {
		result.MaxDrawdown = calculateMaxDrawdown(result.Trades)
		result.SharpeRatio = calculateSharpeRatio(result.Trades)
	}

	return result, nil
}

// findBestPrice finds the BestAsk price for a token near the given timestamp
func findBestPrice(orderBooks []datarecorder.OrderBookRecord, tokenID string, timestamp int64, maxDiffMs int64) float64 {
	// Binary search for closest timestamp
	idx := sort.Search(len(orderBooks), func(i int) bool {
		return orderBooks[i].Timestamp >= timestamp
	})

	// Check candidates: idx and idx-1
	var bestPrice float64 = 0
	var minDiff int64 = math.MaxInt64

	candidates := []int{idx, idx - 1, idx + 1}
	for _, i := range candidates {
		if i >= 0 && i < len(orderBooks) {
			ob := orderBooks[i]
			if ob.TokenID != tokenID {
				continue
			}
			diff := timestamp - ob.Timestamp
			if diff < 0 {
				diff = -diff
			}
			if diff <= maxDiffMs && diff < minDiff {
				minDiff = diff
				bestPrice = ob.BestAsk // Use BestAsk for BUY orders
			}
		}
	}

	return bestPrice
}

// extractSymbolFromKline extracts base symbol (BTC, ETH, etc.) from kline symbol
func extractSymbolFromKline(symbol string) string {
	// Symbol might be "BTCUSDT", we need "BTC"
	suffixes := []string{"USDT", "USDC", "USD", "BUSD"}
	for _, suffix := range suffixes {
		if len(symbol) > len(suffix) && symbol[len(symbol)-len(suffix):] == suffix {
			return symbol[:len(symbol)-len(suffix)]
		}
	}
	return symbol
}

// calculateMaxDrawdown calculates the maximum drawdown from trade history
func calculateMaxDrawdown(trades []TradeRecord) float64 {
	if len(trades) == 0 {
		return 0
	}

	var cumPnL float64 = 0
	var peak float64 = 0
	var maxDD float64 = 0

	for _, t := range trades {
		cumPnL += t.PnL
		if cumPnL > peak {
			peak = cumPnL
		}
		dd := peak - cumPnL
		if dd > maxDD {
			maxDD = dd
		}
	}

	return maxDD
}

// calculateSharpeRatio calculates a simplified Sharpe ratio
func calculateSharpeRatio(trades []TradeRecord) float64 {
	if len(trades) < 2 {
		return 0
	}

	// Calculate mean and std of PnL
	var sum float64 = 0
	for _, t := range trades {
		sum += t.PnL
	}
	mean := sum / float64(len(trades))

	var variance float64 = 0
	for _, t := range trades {
		variance += (t.PnL - mean) * (t.PnL - mean)
	}
	variance /= float64(len(trades))
	std := math.Sqrt(variance)

	if std == 0 {
		return 0
	}

	// Simplified Sharpe (assuming risk-free rate = 0)
	return mean / std
}
