package main

import (
	"Polymarket15MinsCryptoArbitrageTool/api"
	"Polymarket15MinsCryptoArbitrageTool/internal/datarecorder"
	"Polymarket15MinsCryptoArbitrageTool/tools/backtest"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	var (
		dataDir    = flag.String("data", "data/history", "Directory containing historical CSV data")
		configFile = flag.String("config", "", "Path to strategy config YAML file (optional)")
		symbol     = flag.String("symbol", "", "Filter by symbol (e.g., BTC, ETH)")
		verbose    = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	fmt.Println("===============================================")
	fmt.Println("  Polymarket Strategy Backtest Engine")
	fmt.Println("===============================================")

	// 1. Load Data
	klineFile, obFile, err := findLatestDataFiles(*dataDir)
	if err != nil {
		log.Fatalf("❌ Failed to find data files in %s: %v", *dataDir, err)
	}

	fmt.Printf("📂 Loading data:\n")
	fmt.Printf("   Klines:     %s\n", filepath.Base(klineFile))
	fmt.Printf("   OrderBooks: %s\n", filepath.Base(obFile))

	klines, err := datarecorder.LoadKlines(klineFile)
	if err != nil {
		log.Fatalf("❌ Failed to load klines: %v", err)
	}

	orderBooks, err := datarecorder.LoadOrderBooks(obFile)
	if err != nil {
		log.Fatalf("❌ Failed to load orderbooks: %v", err)
	}

	// Filter by symbol if specified
	if *symbol != "" {
		filteredKlines := make([]datarecorder.KlineRecord, 0)
		for _, k := range klines {
			if strings.Contains(k.Symbol, strings.ToUpper(*symbol)) {
				filteredKlines = append(filteredKlines, k)
			}
		}
		klines = filteredKlines
		fmt.Printf("   Filtered to symbol: %s\n", *symbol)
	}

	fmt.Printf("📊 Loaded: %d klines, %d orderbooks\n\n", len(klines), len(orderBooks))

	if len(klines) == 0 {
		log.Fatal("❌ No kline data available for backtesting")
	}

	// 2. Load or create strategy config
	var strategyConfig api.StrategyConfig

	if *configFile != "" {
		// Load from YAML file
		configData, err := os.ReadFile(*configFile)
		if err != nil {
			log.Fatalf("❌ Failed to read config file: %v", err)
		}

		// Parse YAML - it might be a list of configs
		var configs []api.StrategyConfig
		if err := yaml.Unmarshal(configData, &configs); err != nil {
			// Try single config
			if err := yaml.Unmarshal(configData, &strategyConfig); err != nil {
				log.Fatalf("❌ Failed to parse config YAML: %v", err)
			}
		} else if len(configs) > 0 {
			strategyConfig = configs[0]
			fmt.Printf("📋 Using first config from file: %s\n", strategyConfig.ID)
		}
	} else {
		// Use default config
		strategyConfig = getDefaultConfig()
		fmt.Println("📋 Using default strategy config")
	}

	// 3. Build TokenID map (mock - in real scenario, derive from data)
	tokenMap := buildTokenIDMap(orderBooks)
	fmt.Printf("🔗 Token mappings: UP=%s, DOWN=%s\n\n",
		truncateID(tokenMap["UP"]), truncateID(tokenMap["DOWN"]))

	// 4. Run Backtest
	fmt.Println("🚀 Running backtest...")

	btConfig := backtest.BacktestConfig{
		StrategyConfig: strategyConfig,
		Klines:         klines,
		OrderBooks:     orderBooks,
		TokenIDMap:     tokenMap,
		InitialCapital: 1000,
	}

	result, err := backtest.RunBacktest(btConfig)
	if err != nil {
		log.Fatalf("❌ Backtest failed: %v", err)
	}

	// 5. Print Results
	fmt.Println("\n===============================================")
	fmt.Println("  BACKTEST RESULTS")
	fmt.Println("===============================================")
	fmt.Printf("  Total Decisions:  %d\n", result.TotalDecisions)
	fmt.Printf("  Trade Count:      %d\n", result.TradeCount)
	fmt.Printf("  Win / Loss:       %d / %d\n", result.WinCount, result.LossCount)
	fmt.Printf("  Total Volume:     $%.2f\n", result.Volume)
	fmt.Printf("  Total PnL:        $%.4f\n", result.TotalPnL)
	fmt.Printf("  Max Drawdown:     $%.4f\n", result.MaxDrawdown)
	fmt.Printf("  Sharpe Ratio:     %.4f\n", result.SharpeRatio)

	if result.TradeCount > 0 {
		winRate := float64(result.WinCount) / float64(result.TradeCount) * 100
		avgPnL := result.TotalPnL / float64(result.TradeCount)
		fmt.Printf("  Win Rate:         %.1f%%\n", winRate)
		fmt.Printf("  Avg PnL/Trade:    $%.4f\n", avgPnL)
	}

	// Skip Reasons Summary
	if len(result.SkipReasons) > 0 {
		fmt.Println("\n📊 Skip Reasons:")
		for reason, count := range result.SkipReasons {
			fmt.Printf("   %-25s %d\n", reason, count)
		}
	}

	// Verbose: Print sample trades
	if *verbose && len(result.Trades) > 0 {
		fmt.Println("\n📝 Sample Trades (first 10):")
		limit := 10
		if len(result.Trades) < limit {
			limit = len(result.Trades)
		}
		for i := 0; i < limit; i++ {
			t := result.Trades[i]
			fmt.Printf("   [%d] %s %s @ %.4f x%.0f | Z=%.2f | PnL=$%.4f\n",
				t.Timestamp, t.Side, t.Action, t.Price, t.Amount, t.ZScore, t.PnL)
		}
	}

	fmt.Println("\n✅ Backtest complete!")
}

func getDefaultConfig() api.StrategyConfig {
	return api.StrategyConfig{
		ID:                  "backtest_default",
		TrendWindowSecStart: 300,
		TrendWindowSecEnd:   600,
		MinTrendPercentUp:   0.001,
		MinTrendPercentDown: 0.001,
		Tier0ZScore:         1.0,
		Tier1ZScore:         1.5,
		Tier2ZScore:         2.0,
		Tier3ZScore:         2.5,
		Tier4ZScore:         3.0,
		MinEdgeTier0:        0.05,
		MinEdgeTier1:        0.03,
		MinEdgeTier2:        0.02,
		MinEdgeTier3:        0.01,
		MinEdgeTier4:        0.00,
		SigmoidSensitivity:  1.5,
		VolatilityBTC:       0.0015,
		VolatilityETH:       0.002,
		VolatilitySOL:       0.003,
		VolatilityXRP:       0.004,
		Leg0SharePrice0:     0.50,
		SharePriceLevel1:    0.70,
		SharePriceLevel2:    0.85,
		SharePriceLevel3:    0.97,
		ShareAmount1:        50,
		ShareAmount2:        40,
		ShareAmount3:        30,
	}
}

func buildTokenIDMap(orderBooks []datarecorder.OrderBookRecord) map[string]string {
	// In a real scenario, we need to know which TokenID is UP and which is DOWN
	// For now, use the first two distinct TokenIDs found, sorted for deterministic assignment
	tokens := make(map[string]bool)
	for _, ob := range orderBooks {
		tokens[ob.TokenID] = true
	}

	// Sort for deterministic assignment
	tokenList := make([]string, 0, len(tokens))
	for tokenID := range tokens {
		tokenList = append(tokenList, tokenID)
	}
	sort.Strings(tokenList)

	result := make(map[string]string)
	if len(tokenList) >= 2 {
		result["UP"] = tokenList[0]
		result["DOWN"] = tokenList[1]
	} else if len(tokenList) == 1 {
		result["UP"] = tokenList[0]
		result["DOWN"] = tokenList[0]
	}

	return result
}

func findLatestDataFiles(dir string) (string, string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", "", err
	}

	var latestK, latestOB string
	var latestTsK, latestTsOB string

	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, "_klines.csv") {
			ts := strings.TrimSuffix(name, "_klines.csv")
			if ts > latestTsK {
				latestTsK = ts
				latestK = filepath.Join(dir, name)
			}
		}
		if strings.HasSuffix(name, "_orderbooks.csv") {
			ts := strings.TrimSuffix(name, "_orderbooks.csv")
			if ts > latestTsOB {
				latestTsOB = ts
				latestOB = filepath.Join(dir, name)
			}
		}
	}

	if latestK == "" || latestOB == "" {
		return "", "", fmt.Errorf("could not find klines/orderbooks CSV files in %s", dir)
	}
	return latestK, latestOB, nil
}

func truncateID(id string) string {
	if len(id) > 12 {
		return id[:6] + "..." + id[len(id)-4:]
	}
	return id
}
