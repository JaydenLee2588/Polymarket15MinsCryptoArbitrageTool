package main

import (
	"Polymarket15MinsCryptoArbitrageTool/api"
	"Polymarket15MinsCryptoArbitrageTool/internal/datarecorder"
	"Polymarket15MinsCryptoArbitrageTool/tools/backtest"
	"Polymarket15MinsCryptoArbitrageTool/tools/ga"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var (
		dataDir     = flag.String("data", "data/history", "Directory containing historical CSV data")
		popSize     = flag.Int("pop", 50, "Population size")
		generations = flag.Int("gen", 20, "Number of generations")
	)
	flag.Parse()

	// 1. Load Data
	// Find latest kline and orderbook files
	klineFile, obFile, err := findLatestDataFiles(*dataDir)
	if err != nil {
		log.Fatalf("Failed to find data files: %v", err)
	}

	fmt.Printf("Loading data from:\n  Klines: %s\n  OrderBooks: %s\n", klineFile, obFile)

	klines, err := datarecorder.LoadKlines(klineFile)
	if err != nil {
		log.Fatalf("Failed to load klines: %v", err)
	}
	orderBooks, err := datarecorder.LoadOrderBooks(obFile)
	if err != nil {
		log.Fatalf("Failed to load orderbooks: %v", err)
	}

	fmt.Printf("Loaded %d klines and %d orderbooks.\n", len(klines), len(orderBooks))

	// 2. Setup Base Config (complete with all required fields)
	baseConfig := api.StrategyConfig{
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

	// 3. Define Gene Search Space (expanded for comprehensive optimization)
	genes := []ga.Gene{
		// Time Window
		{Name: "TrendWindowSecStart", Min: 60, Max: 600, IsInt: true}, // 1~10 mins
		{Name: "TrendWindowSecEnd", Min: 400, Max: 850, IsInt: true},  // 6.6~14 mins

		// Trend Thresholds
		{Name: "MinTrendPercentUp", Min: 0.0005, Max: 0.005, IsInt: false},   // 0.05% ~ 0.5%
		{Name: "MinTrendPercentDown", Min: 0.0005, Max: 0.005, IsInt: false}, // 0.05% ~ 0.5%

		// Z-Score Tiers
		{Name: "Tier0ZScore", Min: 0.5, Max: 2.0, IsInt: false}, // Baseline
		{Name: "Tier1ZScore", Min: 1.0, Max: 2.5, IsInt: false}, // Tier 1
		{Name: "Tier2ZScore", Min: 1.5, Max: 3.0, IsInt: false}, // Tier 2

		// Min Edge by Tier
		{Name: "MinEdgeTier0", Min: 0.01, Max: 0.10, IsInt: false},  // 1% ~ 10%
		{Name: "MinEdgeTier1", Min: 0.005, Max: 0.05, IsInt: false}, // 0.5% ~ 5%
		{Name: "MinEdgeTier2", Min: 0.001, Max: 0.03, IsInt: false}, // 0.1% ~ 3%

		// Sigmoid Sensitivity
		{Name: "SigmoidSensitivity", Min: 0.5, Max: 5.0, IsInt: false}, // 0.5 ~ 5.0

		// Volatility Factors
		{Name: "VolatilityBTC", Min: 0.0005, Max: 0.005, IsInt: false}, // 0.05% ~ 0.5%
		{Name: "VolatilityETH", Min: 0.001, Max: 0.006, IsInt: false},  // 0.1% ~ 0.6%

		// Share Amounts
		{Name: "ShareAmount1", Min: 20, Max: 100, IsInt: true}, // 20 ~ 100
		{Name: "ShareAmount2", Min: 10, Max: 80, IsInt: true},  // 10 ~ 80
		{Name: "ShareAmount3", Min: 5, Max: 50, IsInt: true},   // 5 ~ 50
	}

	// Token ID Map - Build from orderbook data
	// Find distinct TokenIDs and assign to UP/DOWN deterministically
	tokenMap := buildTokenIDMap(orderBooks)

	// 4. Initialize Optimizer
	optConfig := ga.OptimizerConfig{
		PopulationSize: *popSize,
		MutationRate:   0.2, // 20%
		Generations:    *generations,
		BacktestData: backtest.BacktestConfig{
			StrategyConfig: baseConfig,
			Klines:         klines,
			OrderBooks:     orderBooks,
			TokenIDMap:     tokenMap,
			InitialCapital: 1000,
		},
	}

	optimizer := ga.NewOptimizer(optConfig)
	optimizer.InitPopulation(genes)

	fmt.Println("Starting optimization...")
	bestGenome := optimizer.Run()

	fmt.Println("\nOptimization Complete!")
	fmt.Printf("Best Fitness: %.4f\n", bestGenome.Fitness)
	fmt.Println("Best Parameters:")
	for _, gene := range bestGenome.Genes {
		fmt.Printf("  %s: %.4f\n", gene.Name, gene.Value)
	}
}

func findLatestDataFiles(dir string) (string, string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", "", err
	}

	var latestK, latestOB string
	var latestTsK, latestTsOB string // Using string comparison for YYYYMMDD... works

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
		return "", "", fmt.Errorf("could not find matching pair of klines/orderbooks in %s", dir)
	}
	return latestK, latestOB, nil
}

// buildTokenIDMap builds UP/DOWN mapping from distinct TokenIDs in orderbooks
// Uses sorted order for deterministic assignment
func buildTokenIDMap(orderBooks []datarecorder.OrderBookRecord) map[string]string {
	// Collect distinct TokenIDs
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

	// Assign to UP/DOWN
	result := make(map[string]string)
	if len(tokenList) >= 2 {
		result["UP"] = tokenList[0]
		result["DOWN"] = tokenList[1]
	} else if len(tokenList) == 1 {
		// Fallback: use same token for both
		result["UP"] = tokenList[0]
		result["DOWN"] = tokenList[0]
	} else {
		result["UP"] = "UNKNOWN"
		result["DOWN"] = "UNKNOWN"
	}

	return result
}
