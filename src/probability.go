package src

import (
	"math"
)

// ==================== Z-Score 概率计算引擎 ====================

// NormalCDF 计算标准正态分布的累积分布函数 (CDF)
// 使用 Abramowitz and Stegun 近似公式
func NormalCDF(x float64) float64 {
	t := 1 / (1 + 0.2316419*math.Abs(x))
	d := 0.3989422804014337 * math.Exp(-x*x/2)
	prob := d * t * (0.319381530 + t*(-0.356563782+t*(1.781477937+t*(-1.821255978+t*1.330274429))))

	if x > 0 {
		return 1 - prob
	}
	return prob
}

// CalculateTheoreticalProb 计算理论胜率
// 基于价格距离行权价的 Z-Score 模型
// currentPrice: 当前价格
// strikePrice: 行权价 (Price to Beat)
// minutesLeft: 剩余时间(分钟)
// volatilityPerMin: 每分钟波动率 (单位与价格相同)
func CalculateTheoreticalProb(currentPrice, strikePrice, minutesLeft, volatilityPerMin float64) float64 {
	if currentPrice == 0 || strikePrice == 0 || minutesLeft <= 0 {
		return 0.5 // 数据不足，返回中性
	}

	// 波动率随时间衰减：Sigma = UnitVol * sqrt(Time)
	sigma := volatilityPerMin * math.Sqrt(minutesLeft)

	// 避免除以零
	if sigma < 0.001 {
		sigma = 0.001
	}

	// Z = (Price - Strike) / Sigma
	zScore := (currentPrice - strikePrice) / sigma

	return NormalCDF(zScore)
}

// ProbabilityInput 概率计算的输入参数
type ProbabilityInput struct {
	Price         float64     // 当前价格 (Binance)
	PriceToBeat   float64     // 行权价
	MinutesLeft   float64     // 剩余时间 (分钟)
	VWAP          float64     // VWAP
	VWAPSlope     float64     // VWAP 斜率
	RSI           float64     // RSI 值
	RSISlope      float64     // RSI 斜率
	MACD          *MACDResult // MACD 结果
	HeikenColor   string      // Heiken Ashi 颜色 ("green" / "red")
	HeikenCount   int         // 连续同色数量
	FailedReclaim bool        // 是否 VWAP 收复失败
}

// ProbabilityResult 概率计算结果
type ProbabilityResult struct {
	UpScore   float64 // 看涨得分 (兼容旧接口)
	DownScore float64 // 看跌得分
	RawUp     float64 // 原始看涨概率 (0-1)
	RawDown   float64 // 原始看跌概率 (0-1)
}

// ScoreDirection 综合计算方向概率
// 结合 Z-Score 基础概率和技术指标修正
func ScoreDirection(input ProbabilityInput) ProbabilityResult {
	// 1. 动态波动率：根据剩余时间调整
	// BTC 当前价格约 78000，基础波动率 110
	volatility := 110.0

	if input.MinutesLeft <= 5 {
		volatility = 90.0 // 尾盘降低，体现 Gamma
	} else if input.MinutesLeft <= 10 {
		volatility = 97.0
	}

	// 2. 计算基础概率 (Z-Score)
	baseProb := 0.5
	if input.Price > 0 && input.PriceToBeat > 0 {
		baseProb = CalculateTheoreticalProb(input.Price, input.PriceToBeat, input.MinutesLeft, volatility)
	}

	// 3. 技术面修正 (权重较小，避免过度干扰)
	techScore := 0.0

	// VWAP 位置
	if input.Price > 0 && input.VWAP > 0 {
		if input.Price > input.VWAP {
			techScore += 0.02
		} else {
			techScore -= 0.02
		}
	}

	// VWAP 斜率
	if input.VWAPSlope > 0 {
		techScore += 0.02
	} else if input.VWAPSlope < 0 {
		techScore -= 0.02
	}

	// RSI + RSI斜率
	if input.RSI > 55 && input.RSISlope > 0 {
		techScore += 0.02
	} else if input.RSI < 45 && input.RSISlope < 0 {
		techScore -= 0.02
	}

	// RSI 超买超卖反转
	if input.RSI > 75 {
		techScore -= 0.03
	}
	if input.RSI < 25 {
		techScore += 0.03
	}

	// MACD
	if input.MACD != nil {
		if input.MACD.Histogram > 0 {
			techScore += 0.02
		} else {
			techScore -= 0.02
		}

		if input.MACD.HistDelta > 0 {
			techScore += 0.01
		} else {
			techScore -= 0.01
		}
	}

	// Heiken Ashi
	if input.HeikenColor == "green" && input.HeikenCount >= 2 {
		techScore += 0.02
	} else if input.HeikenColor == "red" && input.HeikenCount >= 2 {
		techScore -= 0.02
	}

	// VWAP 收复失败
	if input.FailedReclaim {
		techScore -= 0.05
	}

	// 4. 合成最终概率，限制在 0.01 - 0.99
	rawUp := clampFloat(baseProb+techScore, 0.01, 0.99)
	rawDown := 1 - rawUp

	return ProbabilityResult{
		UpScore:   rawUp * 10, // 兼容旧接口
		DownScore: rawDown * 10,
		RawUp:     rawUp,
		RawDown:   rawDown,
	}
}

// TimeAwarenessResult 时间感知调整结果
type TimeAwarenessResult struct {
	TimeDecay    float64
	AdjustedUp   float64
	AdjustedDown float64
}

// ApplyTimeAwareness 应用时间感知调整
// 在 Z-Score 模型中，时间衰减已通过 sigma 体现，此处保持透传
func ApplyTimeAwareness(rawUp, remainingMinutes, windowMinutes float64) TimeAwarenessResult {
	// Z-Score 模型已包含时间衰减，直接返回
	return TimeAwarenessResult{
		TimeDecay:    0,
		AdjustedUp:   rawUp,
		AdjustedDown: 1 - rawUp,
	}
}

// ComputeEdge 计算 Edge (模型概率 - 市场价格)
func ComputeEdge(modelUp, modelDown, marketUp, marketDown float64) (edgeUp, edgeDown float64) {
	if marketUp > 0 {
		edgeUp = modelUp - marketUp
	}
	if marketDown > 0 {
		edgeDown = modelDown - marketDown
	}
	return
}
