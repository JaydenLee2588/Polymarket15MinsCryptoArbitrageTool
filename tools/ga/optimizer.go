package ga

import (
	"Polymarket15MinsCryptoArbitrageTool/api"
	"Polymarket15MinsCryptoArbitrageTool/tools/backtest"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"time"
)

// OptimizerConfig Configuration for GA
type OptimizerConfig struct {
	PopulationSize int
	MutationRate   float64
	Generations    int
	BacktestData   backtest.BacktestConfig
}

// Gene represents a tunable parameter
type Gene struct {
	Name  string
	Min   float64
	Max   float64
	Value float64 // Current value (Discrete or Continuous)
	IsInt bool    // If true, Value is rounded to integer
}

// Genome represents a single strategy configuration
type Genome struct {
	Genes   []Gene
	Fitness float64
}

// Optimizer manages the GA process
type Optimizer struct {
	Config     OptimizerConfig
	Population []Genome
	Generation int
	BestGenome Genome
	rng        *rand.Rand
}

// NewOptimizer creates a new optimizer
func NewOptimizer(config OptimizerConfig) *Optimizer {
	return &Optimizer{
		Config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// InitPopulation initializes the population with random genes
// paramSpecs defines the search space
func (o *Optimizer) InitPopulation(paramSpecs []Gene) {
	o.Population = make([]Genome, o.Config.PopulationSize)
	for i := 0; i < o.Config.PopulationSize; i++ {
		genes := make([]Gene, len(paramSpecs))
		copy(genes, paramSpecs)
		for j := range genes {
			// Randomize value
			delta := genes[j].Max - genes[j].Min
			val := genes[j].Min + o.rng.Float64()*delta
			if genes[j].IsInt {
				val = float64(int(val))
			}
			genes[j].Value = val
		}
		o.Population[i] = Genome{Genes: genes}
	}
}

// ApplyGenomeToConfig maps genome values to StrategyConfig
// Supports comprehensive parameter tuning for GA optimization
func ApplyGenomeToConfig(baseConfig api.StrategyConfig, genome Genome) api.StrategyConfig {
	newConfig := baseConfig
	for _, gene := range genome.Genes {
		val := gene.Value
		switch gene.Name {
		// Time Window Parameters
		case "TrendWindowSecStart":
			newConfig.TrendWindowSecStart = int(val)
		case "TrendWindowSecEnd":
			newConfig.TrendWindowSecEnd = int(val)

		// Trend Thresholds
		case "MinTrendPercentUp":
			newConfig.MinTrendPercentUp = val
		case "MinTrendPercentDown":
			newConfig.MinTrendPercentDown = val

		// Z-Score Tiers
		case "Tier0ZScore":
			newConfig.Tier0ZScore = val
		case "Tier1ZScore":
			newConfig.Tier1ZScore = val
		case "Tier2ZScore":
			newConfig.Tier2ZScore = val
		case "Tier3ZScore":
			newConfig.Tier3ZScore = val
		case "Tier4ZScore":
			newConfig.Tier4ZScore = val

		// Min Edge by Tier
		case "MinEdgeTier0":
			newConfig.MinEdgeTier0 = val
		case "MinEdgeTier1":
			newConfig.MinEdgeTier1 = val
		case "MinEdgeTier2":
			newConfig.MinEdgeTier2 = val
		case "MinEdgeTier3":
			newConfig.MinEdgeTier3 = val
		case "MinEdgeTier4":
			newConfig.MinEdgeTier4 = val

		// Sigmoid Sensitivity
		case "SigmoidSensitivity":
			newConfig.SigmoidSensitivity = val

		// Volatility Factors
		case "VolatilityBTC":
			newConfig.VolatilityBTC = val
		case "VolatilityETH":
			newConfig.VolatilityETH = val
		case "VolatilitySOL":
			newConfig.VolatilitySOL = val
		case "VolatilityXRP":
			newConfig.VolatilityXRP = val

		// Share Amount Tiers
		case "ShareAmount1":
			newConfig.ShareAmount1 = val
		case "ShareAmount2":
			newConfig.ShareAmount2 = val
		case "ShareAmount3":
			newConfig.ShareAmount3 = val

		// Price Level Thresholds
		case "Leg0SharePrice0":
			newConfig.Leg0SharePrice0 = val
		case "SharePriceLevel1":
			newConfig.SharePriceLevel1 = val
		case "SharePriceLevel2":
			newConfig.SharePriceLevel2 = val
		case "SharePriceLevel3":
			newConfig.SharePriceLevel3 = val
		}
	}
	return newConfig
}

// EvaluatePopulation runs backtests for all genomes in parallel
func (o *Optimizer) EvaluatePopulation() {
	var wg sync.WaitGroup
	configChan := make(chan int, o.Config.PopulationSize)

	// Create workers
	// Number of workers = number of CPUs for optimal parallelism
	numWorkers := runtime.NumCPU()
	if numWorkers > 16 {
		numWorkers = 16 // Cap at 16 to avoid excessive goroutines
	}
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range configChan {
				genome := &o.Population[idx]

				// Apply genes to strategy config
				runConfig := o.Config.BacktestData
				runConfig.StrategyConfig = ApplyGenomeToConfig(runConfig.StrategyConfig, *genome)

				// Run Backtest
				result, err := backtest.RunBacktest(runConfig)
				fitness := 0.0
				if err == nil {
					// Fitness Function: Combination of PnL, Sharpe, and trade activity
					// Primary: Total PnL
					fitness = result.TotalPnL

					// Bonus for higher Sharpe Ratio (risk-adjusted returns)
					if result.SharpeRatio > 0 {
						fitness += result.SharpeRatio * 10
					}

					// Proportional penalty for low trade count (encourage active strategies)
					if result.TradeCount < 10 {
						fitness -= float64(10-result.TradeCount) * 50 // Max penalty: 500
					}

					// Penalty for high drawdown
					if result.MaxDrawdown > 0 {
						fitness -= result.MaxDrawdown * 0.5
					}
				} else {
					fitness = -9999
				}

				genome.Fitness = fitness
			}
		}()
	}

	for i := 0; i < len(o.Population); i++ {
		configChan <- i
	}
	close(configChan)
	wg.Wait()
}

// Evolve advances the population to the next generation
func (o *Optimizer) Evolve() {
	// Sort by Fitness data descending
	sort.Slice(o.Population, func(i, j int) bool {
		return o.Population[i].Fitness > o.Population[j].Fitness
	})

	// Elitism: Keep top 10%
	eliteCount := o.Config.PopulationSize / 10
	if eliteCount < 1 {
		eliteCount = 1
	}

	o.BestGenome = o.Population[0]

	newPop := make([]Genome, 0, o.Config.PopulationSize)
	newPop = append(newPop, o.Population[:eliteCount]...)

	// Fill rest with crossover/mutation
	for len(newPop) < o.Config.PopulationSize {
		// Tournament Selection
		p1 := o.TournamentSelect()
		p2 := o.TournamentSelect()

		// Crossover
		child := o.Crossover(p1, p2)

		// Mutation
		o.Mutate(&child)

		newPop = append(newPop, child)
	}

	o.Population = newPop
	o.Generation++

	fmt.Printf("Generation %d Best Fitness: %.4f\n", o.Generation, o.BestGenome.Fitness)
}

func (o *Optimizer) TournamentSelect() Genome {
	limit := 3
	best := o.Population[o.rng.Intn(len(o.Population))]
	for i := 0; i < limit; i++ {
		cand := o.Population[o.rng.Intn(len(o.Population))]
		if cand.Fitness > best.Fitness {
			best = cand
		}
	}
	return best
}

func (o *Optimizer) Crossover(p1, p2 Genome) Genome {
	childGenes := make([]Gene, len(p1.Genes))
	// Uniform Crossover
	for i := range p1.Genes {
		if o.rng.Float64() < 0.5 {
			childGenes[i] = p1.Genes[i]
		} else {
			childGenes[i] = p2.Genes[i]
		}
	}
	return Genome{Genes: childGenes}
}

func (o *Optimizer) Mutate(g *Genome) {
	for i := range g.Genes {
		if o.rng.Float64() < o.Config.MutationRate {
			// Gaussian mutation (perturb value)
			// Amount: 10% of range
			delta := g.Genes[i].Max - g.Genes[i].Min
			change := (o.rng.Float64() - 0.5) * 0.2 * delta // +/- 10%

			newVal := g.Genes[i].Value + change

			// Clamp
			if newVal < g.Genes[i].Min {
				newVal = g.Genes[i].Min
			}
			if newVal > g.Genes[i].Max {
				newVal = g.Genes[i].Max
			}

			if g.Genes[i].IsInt {
				newVal = float64(int(newVal))
			}
			g.Genes[i].Value = newVal
		}
	}
}

// Run executes the optimization loop
func (o *Optimizer) Run() Genome {
	for i := 0; i < o.Config.Generations; i++ {
		o.EvaluatePopulation()
		o.Evolve()
	}
	return o.BestGenome
}
