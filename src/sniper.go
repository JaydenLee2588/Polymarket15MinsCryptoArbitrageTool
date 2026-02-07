package src

import (
	"math"
)

// ==================== Sniper 狙击策略 v4.4 ====================

// SniperInput 狙击策略输入
type SniperInput struct {
	CurrentPrice  float64 // Binance 当前价格
	StrikePrice   float64 // 行权价
	TimeLeftSec   float64 // 剩余秒数
	Sigma         float64 // 实时波动率
	PolyPriceUp   float64 // Polymarket UP 价格
	PolyPriceDown float64 // Polymarket DOWN 价格
	Last1MinHigh  float64 // 最近1分钟高点
	Last1MinLow   float64 // 最近1分钟低点
	PolySpread    float64 // Poly 盘口价差 (Ask - Bid)
}

// SniperDecision 决策结果
type SniperDecision struct {
	Action     string  // "BUY_UP", "BUY_DOWN", "SELL_UP", "SELL_DOWN", "HOLD", "EXIT"
	Reason     string  // 决策原因
	Phase      int     // 当前阶段 (1-4)
	ZScore     float64 // 当前 Z-Score
	TheoProb   float64 // 理论胜率 (UP 方向)
	Edge       float64 // Edge = TheoProb - PolyPrice
	TargetSide string  // "UP" or "DOWN"
}

// SniperConfig 狙击策略配置
type SniperConfig struct {
	ZScoreEntryThreshold float64 // Z-Score 入场阈值 (默认 3.0)
	ZScoreExitThreshold  float64 // Z-Score 退出阈值 (默认 1.0)
	ProfitExitPct        float64 // 盈利退出百分比 (默认 5.0)
	SpreadMax            float64 // 最大价差 (默认 0.05)
	DeadFishMinEdge      float64 // 死鱼最小 Edge (默认 0.02)
	EdgeThreshold        float64 // Edge 阈值 (默认 0.06)
}

// DefaultSniperConfig 默认配置
func DefaultSniperConfig() SniperConfig {
	return SniperConfig{
		ZScoreEntryThreshold: 3.0,
		ZScoreExitThreshold:  1.0,
		ProfitExitPct:        5.0,
		SpreadMax:            0.05,
		DeadFishMinEdge:      0.02,
		EdgeThreshold:        0.06,
	}
}

// ==================== 核心计算函数 ====================

// GetPhase 根据剩余时间判定阶段
// Phase 1&2 (>3min): 极值回归
// Phase 3 (30s-3min): 分歧狙击
// Phase 4 (<30s): 死鱼收割
func GetPhase(timeLeftSec float64) int {
	switch {
	case timeLeftSec > 180:
		return 1 // > 3min: 极值回归
	case timeLeftSec > 30:
		return 3 // 30s-3min: 分歧狙击
	default:
		return 4 // < 30s: 死鱼收割
	}
}

// CalculateZScore 计算 Z-Score
// Z = (CurrentPrice - StrikePrice) / Sigma
func CalculateZScore(currentPrice, strikePrice, sigma float64) float64 {
	if sigma < 0.001 {
		sigma = 0.001
	}
	return (currentPrice - strikePrice) / sigma
}

// CheckBreakout 检测1分钟突破 (带容差)
// 返回: 是否向上突破, 是否向下突破
// v4.5: 加入 0.1% 容差，价格接近高/低点即算突破
func CheckBreakout(currentPrice, high1m, low1m float64) (isLongBreakout, isShortBreakout bool) {
	tolerance := 0.001 // 0.1% 容差
	isLongBreakout = currentPrice >= high1m*(1-tolerance)
	isShortBreakout = currentPrice <= low1m*(1+tolerance)
	return
}

// ==================== 主决策函数 ====================

// EvaluateSniper 主决策入口
func EvaluateSniper(input SniperInput, cfg SniperConfig) SniperDecision {
	phase := GetPhase(input.TimeLeftSec)
	zScore := CalculateZScore(input.CurrentPrice, input.StrikePrice, input.Sigma)
	theoProb := NormalCDF(zScore) // 理论 UP 胜率

	decision := SniperDecision{
		Action:   "HOLD",
		Phase:    phase,
		ZScore:   zScore,
		TheoProb: theoProb,
	}

	// 滑点熔断
	if input.PolySpread > cfg.SpreadMax {
		decision.Reason = "spread_too_wide"
		return decision
	}

	// 根据阶段分发决策
	switch phase {
	case 1, 2:
		return evaluateReversal(input, cfg, zScore, theoProb)
	case 3:
		return evaluateArbitrage(input, cfg, zScore, theoProb)
	case 4:
		return evaluateDeadFish(input, cfg, zScore, theoProb)
	}

	return decision
}

// ==================== Phase 1 & 2: 极值反转 ====================

func evaluateReversal(input SniperInput, cfg SniperConfig, zScore, theoProb float64) SniperDecision {
	decision := SniperDecision{
		Action:   "HOLD",
		Phase:    GetPhase(input.TimeLeftSec),
		ZScore:   zScore,
		TheoProb: theoProb,
	}

	absZ := math.Abs(zScore)

	// 1. Z-Score 必须达到极值 (> 3.0)
	if absZ < cfg.ZScoreEntryThreshold {
		decision.Reason = "zscore_not_extreme"
		return decision
	}

	// 2. 检测突破确认
	isLongBreakout, isShortBreakout := CheckBreakout(input.CurrentPrice, input.Last1MinHigh, input.Last1MinLow)

	// 3. 极值回归逻辑
	// 3. 极值回归逻辑
	if zScore < -cfg.ZScoreEntryThreshold {
		// Z-Score 极度偏低 (价格远低于行权价)
		if !isLongBreakout {
			decision.Reason = "no_breakout_confirmation"
			return decision
		}

		// 向上突破 → 检查 Edge
		edge := theoProb - input.PolyPriceUp
		if edge > cfg.EdgeThreshold {
			decision.Action = "BUY_UP"
			decision.TargetSide = "UP"
			decision.Edge = edge
			decision.Reason = "reversal_long_breakout"
			return decision
		} else {
			decision.Reason = "edge_too_low"
			decision.Edge = edge
			decision.TargetSide = "UP"
			return decision
		}
	}

	if zScore > cfg.ZScoreEntryThreshold {
		// Z-Score 极度偏高 (价格远高于行权价)
		if !isShortBreakout {
			decision.Reason = "no_breakout_confirmation"
			return decision
		}

		// 向下突破 → 检查 Edge
		theoDown := 1 - theoProb
		edge := theoDown - input.PolyPriceDown
		if edge > cfg.EdgeThreshold {
			decision.Action = "BUY_DOWN"
			decision.TargetSide = "DOWN"
			decision.Edge = edge
			decision.Reason = "reversal_short_breakout"
			return decision
		} else {
			decision.Reason = "edge_too_low"
			decision.Edge = edge
			decision.TargetSide = "DOWN"
			return decision
		}
	}

	decision.Reason = "zscore_not_extreme_enough_for_direction" // 理论上前面的 absZ check 已经拦截，但这作为最后兜底
	return decision
}

// ==================== Phase 3: 分歧套利 ====================

func evaluateArbitrage(input SniperInput, cfg SniperConfig, zScore, theoProb float64) SniperDecision {
	decision := SniperDecision{
		Action:   "HOLD",
		Phase:    3,
		ZScore:   zScore,
		TheoProb: theoProb,
	}

	// 计算 Edge
	edgeUp := theoProb - input.PolyPriceUp
	edgeDown := (1 - theoProb) - input.PolyPriceDown

	// 1. 做多套利: 理论胜率高，但 Poly 价格低
	if edgeUp > cfg.EdgeThreshold {
		decision.Action = "BUY_UP"
		decision.TargetSide = "UP"
		decision.Edge = edgeUp
		decision.Reason = "arbitrage_edge_up"
		return decision
	}

	// 2. 做空套利: 理论胜率低，但 Poly UP 价格高
	// 支持做空: Poly_Price > 0.95 且 Theoretical_Prob < 50%
	if input.PolyPriceUp > 0.95 && theoProb < 0.50 {
		edge := input.PolyPriceUp - theoProb
		if edge > cfg.EdgeThreshold {
			decision.Action = "SELL_UP"
			decision.TargetSide = "UP"
			decision.Edge = edge
			decision.Reason = "arbitrage_short_overpriced"
			return decision
		}
	}

	// 3. 做多 DOWN 套利
	if edgeDown > cfg.EdgeThreshold {
		decision.Action = "BUY_DOWN"
		decision.TargetSide = "DOWN"
		decision.Edge = edgeDown
		decision.Reason = "arbitrage_edge_down"
		return decision
	}

	decision.Reason = "no_arbitrage_opportunity"
	return decision
}

// ==================== Phase 4: 死鱼收割 ====================

func evaluateDeadFish(input SniperInput, cfg SniperConfig, zScore, theoProb float64) SniperDecision {
	decision := SniperDecision{
		Action:   "HOLD",
		Phase:    4,
		ZScore:   zScore,
		TheoProb: theoProb,
	}

	// 死鱼逻辑: 寻找胜率已确定但价格未归位的合约
	// 胜率 > 99% 但价格 < 0.98 → 买入
	// 胜率 < 1% 但价格 > 0.02 → 做空或反向买入

	// 1. UP 方向死鱼: 胜率极高但价格未到位
	// 放宽阈值: 0.99 -> 0.95, 0.98 -> 0.94
	if theoProb > 0.95 && input.PolyPriceUp < 0.94 {
		edge := theoProb - input.PolyPriceUp
		if edge > cfg.DeadFishMinEdge {
			decision.Action = "BUY_UP"
			decision.TargetSide = "UP"
			decision.Edge = edge
			decision.Reason = "dead_fish_up"
			return decision
		}
	}

	// 2. DOWN 方向死鱼: 胜率极低 (UP) → DOWN 胜率极高
	// 放宽阈值: 0.01 -> 0.05
	if theoProb < 0.05 && input.PolyPriceDown < 0.94 {
		theoDown := 1 - theoProb
		edge := theoDown - input.PolyPriceDown
		if edge > cfg.DeadFishMinEdge {
			decision.Action = "BUY_DOWN"
			decision.TargetSide = "DOWN"
			decision.Edge = edge
			decision.Reason = "dead_fish_down"
			return decision
		}
	}

	// 3. 做空死鱼: 胜率极低但价格虚高
	// 放宽阈值: 0.01 -> 0.05, 0.02 -> 0.06
	if theoProb < 0.05 && input.PolyPriceUp > 0.06 {
		edge := input.PolyPriceUp - theoProb
		if edge > cfg.DeadFishMinEdge {
			decision.Action = "SELL_UP"
			decision.TargetSide = "UP"
			decision.Edge = edge
			decision.Reason = "dead_fish_short_up"
			return decision
		}
	}

	decision.Reason = "no_dead_fish"
	return decision
}

// ==================== 止盈判定 (Touch-and-Go) ====================

// ShouldExitPosition 判断是否应该止盈退出
// 返回: 是否应该退出, 退出原因
// entryPolyPrice: 买入时的 Polymarket 合约价格
// currentPolyPrice: 当前 Polymarket 合约卖出价格
func ShouldExitPosition(
	currentZScore float64,
	entryPolyPrice float64,
	currentPolyPrice float64,
	cfg SniperConfig,
) (shouldExit bool, reason string) {

	// 1. Z-Score 回归: |Z| < 1.0
	if math.Abs(currentZScore) < cfg.ZScoreExitThreshold {
		return true, "zscore_reverted"
	}

	// 2. 盈利 > 5% (基于 Polymarket 合约价格)
	if entryPolyPrice > 0 && currentPolyPrice > 0 {
		profitPct := (currentPolyPrice - entryPolyPrice) / entryPolyPrice * 100
		if profitPct > cfg.ProfitExitPct {
			return true, "profit_target_reached"
		}
	}

	return false, ""
}
