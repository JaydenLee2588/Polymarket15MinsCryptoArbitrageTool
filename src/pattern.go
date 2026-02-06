package src

import (
	"math"
	"time"
)

// ==================== 形态类型定义 ====================

// PatternType 形态类型
type PatternType string

const (
	PatternRocketLaunch   PatternType = "ROCKET_LAUNCH"   // 🚀 火箭发射
	PatternAerialRefuel   PatternType = "AERIAL_REFUEL"   // 🚀 空中加油
	PatternGuillotine     PatternType = "GUILLOTINE"      // 📉 断头铡刀
	PatternUnderwaterDead PatternType = "UNDERWATER_DEAD" // 📉 水下死叉
	PatternBullishDiv     PatternType = "BULLISH_DIV"     // 🎣 底背离
	PatternOversold       PatternType = "OVERSOLD"        // 🎣 极度超卖
	PatternBearishDiv     PatternType = "BEARISH_DIV"     // 🏃 顶背离
	PatternClimaxTop      PatternType = "CLIMAX_TOP"      // 🏃 情绪高潮
	PatternChopping       PatternType = "CHOPPING"        // 🕸️ 织布机
	PatternTrap           PatternType = "TRAP"            // 🕸️ 假突破
	PatternNeutral        PatternType = "NEUTRAL"         // 无明确形态
)

// PatternInfo 形态详细信息
type PatternInfo struct {
	Name       string // 形态名称
	SignalType string // 信号类型 (🚀顺势多, 📉顺势空, 🎣反转多, 🏃反转空, 🕸️震荡)
	Trend      string // 趋势预测
	Action     string // 操作建议 (买涨/买跌/观望)
	Recommend  string // 推荐等级 (强烈推荐/推荐/谨慎/不推荐)
}

// 形态配置表
var patternInfoMap = map[PatternType]PatternInfo{
	PatternRocketLaunch: {
		Name: "火箭发射", SignalType: "顺势多", Trend: "大涨 【大涨】",
		Action: "【买涨】", Recommend: "✅ 强烈推荐",
	},
	PatternAerialRefuel: {
		Name: "空中加油", SignalType: "顺势多", Trend: "续涨 【涨】",
		Action: "【买涨】", Recommend: "✅ 推荐",
	},
	PatternGuillotine: {
		Name: "断头铡刀", SignalType: "顺势空", Trend: "急跌 【大跌】",
		Action: "【买跌】", Recommend: "✅ 强烈推荐",
	},
	PatternUnderwaterDead: {
		Name: "水下死叉", SignalType: "顺势空", Trend: "深跌 【大跌】",
		Action: "【买跌】", Recommend: "✅ 推荐",
	},
	PatternBullishDiv: {
		Name: "底背离", SignalType: "反转多", Trend: "反弹 【涨】",
		Action: "【买涨】", Recommend: "⚠️ 谨慎",
	},
	PatternOversold: {
		Name: "极度超卖", SignalType: "反转多", Trend: "触底 【涨】",
		Action: "【买涨】", Recommend: "✅ 推荐",
	},
	PatternBearishDiv: {
		Name: "顶背离", SignalType: "反转空", Trend: "变盘 【跌】",
		Action: "【买跌】", Recommend: "谨慎",
	},
	PatternClimaxTop: {
		Name: "情绪高潮", SignalType: "反转空", Trend: "暴跌 【大跌】",
		Action: "【买跌】", Recommend: "✅ 强烈推荐",
	},
	PatternChopping: {
		Name: "织布机", SignalType: "震荡", Trend: "无",
		Action: "【观望】", Recommend: "❌ 不推荐",
	},
	PatternTrap: {
		Name: "假突破", SignalType: "震荡", Trend: "回落 【微跌】",
		Action: "【观望】", Recommend: "❌ 不推荐",
	},
	PatternNeutral: {
		Name: "无明确形态", SignalType: "震荡", Trend: "不明",
		Action: "【观望】", Recommend: "❌ 不推荐",
	},
	PatternTrendSurgeUp: {
		Name: "单边上涨", SignalType: "顺势多", Trend: "暴涨 【大涨】",
		Action: "【买涨】", Recommend: "✅ 强烈推荐",
	},
	PatternTrendSurgeDown: {
		Name: "单边下跌", SignalType: "顺势空", Trend: "暴跌 【大跌】",
		Action: "【买跌】", Recommend: "✅ 强烈推荐",
	},
}

// PatternDetectInput 形态检测输入
type PatternDetectInput struct {
	VWAPDist float64     // 价格相对 VWAP 的距离 (正=上方)
	MACD     *MACDResult // MACD 结果
	RSI      float64     // RSI 当前值
	RSISlope float64     // RSI 斜率
	PrevRSI  float64     // 前一个 RSI 值 (用于背离检测)
}

// PatternResult 形态检测结果
type PatternResult struct {
	PatternName      string // 形态名称
	PatternType      PatternType
	SignalType       string  // 信号类型
	VWAPPosition     string  // VWAP 位置描述
	MACDStatus       string  // MACD 状态描述
	RSIStatus        string  // RSI 状态描述
	PredictedTrend   string  // 预测趋势
	ActionSuggestion string  // 操作建议
	RecommendEntry   string  // 入场推荐
	Confidence       float64 // 置信度 (0-1)
}

// PatternState 迟滞回线状态机状态
type PatternState struct {
	State      string    // "IDLE" 或 "TRENDING"
	Direction  string    // "UP" 或 "DOWN" (仅在 TRENDING 时有效)
	LastScore  float64   // 上次评分 (0-100)
	EntryTime  time.Time // 进入 TRENDING 的时间
	EntryPrice float64   // 进入时的价格
	MarketSlug string    // 关联的市场 Slug (用于检测市场变化)
}

// ==================== 辅助函数 ====================

// GetPatternChineseName 获取 Pattern 的中文名称
func GetPatternChineseName(pt PatternType) string {
	if info, ok := patternInfoMap[pt]; ok {
		return info.Name
	}
	return "无明确形态"
}

// getVWAPPosition 获取价格相对 VWAP 的位置描述
func getVWAPPosition(vwapDist float64) string {
	pct := math.Abs(vwapDist * 100)

	if pct < 0.05 {
		return "缠绕粘合"
	}
	if vwapDist > 0.003 {
		return "极度远离(上方)"
	}
	if vwapDist > 0.001 {
		return "远离(上方)"
	}
	if vwapDist > 0 {
		return "站稳上方"
	}
	if vwapDist < -0.003 {
		return "极度远离(下方)"
	}
	if vwapDist < -0.001 {
		return "远离(下方)"
	}
	return "跌破下方"
}

// getMACDStatus 获取 MACD 状态描述
func getMACDStatus(macd *MACDResult) string {
	if macd == nil {
		return "-"
	}

	isGoldenCross := macd.Histogram > 0 && macd.HistDelta > 0
	isDeadCross := macd.Histogram < 0 && macd.HistDelta < 0
	isNearZero := math.Abs(macd.Histogram) < 0.5

	if isNearZero && math.Abs(macd.HistDelta) < 0.2 {
		return "零轴粘合"
	}
	if isGoldenCross {
		return "金叉/红柱放大"
	}
	if isDeadCross {
		return "死叉/绿柱放大"
	}
	if macd.Histogram > 0 && macd.HistDelta < 0 {
		return "红柱缩短"
	}
	if macd.Histogram < 0 && macd.HistDelta > 0 {
		return "绿柱缩短"
	}
	if macd.Histogram < 0 && macd.MACD < 0 {
		return "零轴下方"
	}
	if macd.Histogram > 0 {
		return "看涨"
	}
	return "看跌"
}

// getRSIStatus 获取 RSI 状态描述
func getRSIStatus(rsi, rsiSlope float64) string {
	slopeDir := ""
	if rsiSlope > 0.2 {
		slopeDir = "↑"
	} else if rsiSlope < -0.2 {
		slopeDir = "↓"
	}

	if rsi > 85 {
		return ">85 极度超买" + slopeDir
	}
	if rsi > 70 {
		return ">70 超买" + slopeDir
	}
	if rsi > 55 {
		return "突破55" + slopeDir
	}
	if rsi >= 45 && rsi <= 55 {
		return "45-55 中性" + slopeDir
	}
	if rsi < 20 {
		return "<20 极度超卖" + slopeDir
	}
	if rsi < 30 {
		return "<30 超卖" + slopeDir
	}
	if rsi < 45 {
		return "跌破45" + slopeDir
	}
	return "中性"
}

// ==================== 核心检测函数 ====================

// DetectPattern 检测当前形态
func DetectPattern(input PatternDetectInput) PatternResult {
	vwapPos := getVWAPPosition(input.VWAPDist)
	macdStatus := getMACDStatus(input.MACD)
	rsiStatus := getRSIStatus(input.RSI, input.RSISlope)

	safeRSISlope := input.RSISlope // 已在 Go 中自动初始化为 0

	var patternType PatternType
	var confidence float64

	// 1. 🚀 火箭发射: VWAP上方 + MACD金叉/红柱放大 + RSI>55↑
	if input.VWAPDist > 0 &&
		input.MACD != nil && input.MACD.Histogram > 0 && input.MACD.HistDelta > 0 &&
		input.RSI > 55 && safeRSISlope > 0 {
		patternType = PatternRocketLaunch
		confidence = 0.9
	} else if input.VWAPDist > -0.001 && input.VWAPDist < 0.002 &&
		input.MACD != nil && input.MACD.Histogram < 0 && input.MACD.HistDelta > 0 &&
		input.RSI >= 45 && input.RSI <= 55 && safeRSISlope > 0 {
		// 2. 🚀 空中加油: 回踩VWAP不破 + 绿柱缩短 + RSI 45-50拐头
		patternType = PatternAerialRefuel
		confidence = 0.8
	} else if input.VWAPDist < 0 &&
		input.MACD != nil && input.MACD.Histogram < 0 && input.MACD.HistDelta < 0 &&
		input.RSI < 45 && safeRSISlope < 0 {
		// 3. 📉 断头铡刀: VWAP下方 + MACD死叉/绿柱放大 + RSI<45↓
		patternType = PatternGuillotine
		confidence = 0.9
	} else if input.VWAPDist < 0 && input.VWAPDist > -0.002 &&
		input.MACD != nil && input.MACD.MACD < 0 && input.MACD.Histogram < 0 && input.MACD.HistDelta < 0 &&
		input.RSI < 60 && safeRSISlope < 0 {
		// 4. 📉 水下死叉: 反弹不过VWAP + 零轴下死叉 + RSI<60拐头
		patternType = PatternUnderwaterDead
		confidence = 0.8
	} else if input.VWAPDist < -0.001 &&
		input.MACD != nil && input.MACD.Histogram < 0 && input.MACD.HistDelta > 0 &&
		input.PrevRSI > 0 && input.RSI > input.PrevRSI {
		// 5. 🎣 底背离: 远离VWAP下方 + 绿柱缩短 + RSI抬高
		patternType = PatternBullishDiv
		confidence = 0.7
	} else if input.VWAPDist < -0.001 &&
		input.MACD != nil && input.MACD.Histogram > 0 && input.MACD.HistDelta > 0 &&
		input.RSI < 20 {
		// 6. 🎣 极度超卖: 远离VWAP下方 + 低位金叉 + RSI<20
		patternType = PatternOversold
		confidence = 0.75
	} else if input.VWAPDist > 0.001 &&
		input.MACD != nil && input.MACD.Histogram > 0 && input.MACD.HistDelta < 0 &&
		input.PrevRSI > 0 && input.RSI < input.PrevRSI {
		// 7. 🏃 顶背离: 远离VWAP上方 + 红柱缩短 + RSI走低
		patternType = PatternBearishDiv
		confidence = 0.7
	} else if input.VWAPDist > 0.003 &&
		input.MACD != nil && input.MACD.Histogram > 0 && math.Abs(input.MACD.HistDelta) < 0.3 &&
		input.RSI > 85 {
		// 8. 🏃 情绪高潮: 极度远离VWAP + 红柱停滞 + RSI>85
		patternType = PatternClimaxTop
		confidence = 0.85
	} else if math.Abs(input.VWAPDist) < 0.0005 &&
		input.MACD != nil && math.Abs(input.MACD.Histogram) < 0.5 &&
		input.RSI >= 45 && input.RSI <= 55 {
		// 9. 🕸️ 织布机: VWAP粘合 + 零轴粘合 + RSI 45-55
		patternType = PatternChopping
		confidence = 0.8
	} else if math.Abs(input.VWAPDist) < 0.001 &&
		input.MACD != nil && math.Abs(input.MACD.HistDelta) < 0.2 &&
		input.RSI >= 48 && input.RSI <= 52 {
		// 10. 🕸️ 假突破
		patternType = PatternTrap
		confidence = 0.6
	} else {
		// 默认：基于方向给出判断
		safeRSI := input.RSI
		if safeRSI == 0 {
			safeRSI = 50
		}

		if input.VWAPDist > 0 && safeRSI > 50 {
			patternType = PatternNeutral // 偏多震荡
		} else if input.VWAPDist < 0 && safeRSI < 50 {
			patternType = PatternNeutral // 偏空震荡
		} else {
			patternType = PatternNeutral
		}
		confidence = 0.5
	}

	// 获取形态信息
	info, ok := patternInfoMap[patternType]
	if !ok {
		info = patternInfoMap[PatternNeutral]
	}

	return PatternResult{
		PatternName:      info.Name,
		PatternType:      patternType,
		SignalType:       info.SignalType,
		VWAPPosition:     vwapPos,
		MACDStatus:       macdStatus,
		RSIStatus:        rsiStatus,
		PredictedTrend:   info.Trend,
		ActionSuggestion: info.Action,
		RecommendEntry:   info.Recommend,
		Confidence:       confidence,
	}
}

// IsStrongRecommend 判断是否为强烈推荐
func (r *PatternResult) IsStrongRecommend() bool {
	return r.RecommendEntry == "✅ 强烈推荐"
}

// IsBuyUp 判断是否建议买涨
func (r *PatternResult) IsBuyUp() bool {
	return r.ActionSuggestion == "【买涨】"
}

// IsBuyDown 判断是否建议买跌
func (r *PatternResult) IsBuyDown() bool {
	return r.ActionSuggestion == "【买跌】"
}

// ShouldHold 判断是否应该观望
func (r *PatternResult) ShouldHold() bool {
	return r.ActionSuggestion == "【观望】"
}

// ==================== Hysteresis State Machine ====================

// DetectPatternWithHysteresis 使用迟滞回线机制检测形态
// 双阈值: 80分入场, 65分退出
// VWAP 否决: 价格破位立即退出
func DetectPatternWithHysteresis(
	input PatternDetectInput,
	currentState *PatternState,
	currentPrice float64,
	vwap float64,
	marketSlug string, // 市场 Slug (用于追踪状态所属市场)
) (result PatternResult, newState *PatternState) {
	// 初始化状态
	if currentState == nil {
		currentState = &PatternState{
			State: "IDLE",
		}
	}

	// 1. 计算原始形态和评分
	basePattern := DetectPattern(input)
	score := CalculatePatternScore(basePattern)

	// 2. VWAP 否决检查 (仅在 TRENDING 状态且 VWAP 有效时)
	if currentState.State == "TRENDING" && vwap > 0 {
		vetoTriggered := false

		// 方向破位检查
		if currentState.Direction == "UP" && currentPrice < vwap {
			vetoTriggered = true
		} else if currentState.Direction == "DOWN" && currentPrice > vwap {
			vetoTriggered = true
		}

		if vetoTriggered {
			// 强制退出到 IDLE
			newState = &PatternState{
				State:      "IDLE",
				LastScore:  score,
				MarketSlug: marketSlug,
			}

			// 返回退出信号
			result = basePattern
			result.ActionSuggestion = "【观望】"
			result.RecommendEntry = "❌ VWAP破位退出"
			return result, newState
		}
	}

	// 3. 状态机转换逻辑
	switch currentState.State {
	case "IDLE":
		// IDLE -> TRENDING: 需要 Score >= 80
		if score >= 80 {
			direction := ""
			if basePattern.IsBuyUp() {
				direction = "UP"
			} else if basePattern.IsBuyDown() {
				direction = "DOWN"
			}

			if direction != "" {
				// 转换到 TRENDING
				newState = &PatternState{
					State:      "TRENDING",
					Direction:  direction,
					LastScore:  score,
					EntryTime:  time.Now(),
					EntryPrice: currentPrice,
					MarketSlug: marketSlug,
				}
				result = basePattern // 保持原始建议 (买涨/买跌)
			} else {
				// 分数够但没有明确方向，保持 IDLE
				newState = &PatternState{
					State:      "IDLE",
					LastScore:  score,
					MarketSlug: marketSlug,
				}
				result = basePattern
				result.ActionSuggestion = "【观望】"
				result.RecommendEntry = "❌ 无明确方向"
			}
		} else {
			// 保持 IDLE
			newState = &PatternState{
				State:      "IDLE",
				LastScore:  score,
				MarketSlug: marketSlug,
			}
			result = basePattern
			result.ActionSuggestion = "【观望】"
			result.RecommendEntry = "❌ 评分不足 (需要80)"
		}

	case "TRENDING":
		// TRENDING -> IDLE: 需要 Score < 65
		if score < 65 {
			// 转换到 IDLE
			newState = &PatternState{
				State:      "IDLE",
				LastScore:  score,
				MarketSlug: marketSlug,
			}
			result = basePattern
			result.ActionSuggestion = "【观望】"
			result.RecommendEntry = "❌ 动能衰竭退出"
		} else {
			// 保持 TRENDING
			newState = &PatternState{
				State:      "TRENDING",
				Direction:  currentState.Direction,
				LastScore:  score,
				EntryTime:  currentState.EntryTime,
				EntryPrice: currentState.EntryPrice,
				MarketSlug: marketSlug,
			}

			// 保持原方向的建议
			result = basePattern
			if currentState.Direction == "UP" {
				result.ActionSuggestion = "【买涨】"
				result.RecommendEntry = "✅ 持有 (趋势中)"
			} else {
				result.ActionSuggestion = "【买跌】"
				result.RecommendEntry = "✅ 持有 (趋势中)"
			}
		}

	default:
		// 未知状态，重置为 IDLE
		newState = &PatternState{
			State:      "IDLE",
			LastScore:  score,
			MarketSlug: marketSlug,
		}
		result = basePattern
	}

	return result, newState
}

// CalculatePatternScore 计算形态评分 (0-100)
// 综合考虑: 形态强度、推荐等级、方向明确性
func CalculatePatternScore(pattern PatternResult) float64 {
	// 基础分: 形态强度 (40分)
	// 强势形态 (火箭、断头等) = 40分
	// 普通形态 (空中加油等) = 30分
	// 震荡形态 = 10分
	patternScore := 10.0
	switch pattern.SignalType {
	case "顺势多", "顺势空":
		patternScore = 40.0 // 顺势信号最强
	case "反转多", "反转空":
		patternScore = 30.0 // 反转信号次之
	case "震荡":
		patternScore = 10.0 // 震荡最弱
	}

	// 推荐分 (30分)
	recommendScore := 0.0
	switch pattern.RecommendEntry {
	case "✅ 强烈推荐":
		recommendScore = 30.0
	case "✅ 推荐":
		recommendScore = 20.0
	case "⚠️ 谨慎":
		recommendScore = 10.0
	case "❌ 不推荐":
		recommendScore = 0.0
	}

	// 置信度分 (30分)
	confidenceScore := pattern.Confidence * 30.0

	// 方向明确性加分 (额外 10分)
	directionBonus := 0.0
	if pattern.ActionSuggestion == "【买涨】" || pattern.ActionSuggestion == "【买跌】" {
		directionBonus = 10.0
	}

	totalScore := patternScore + recommendScore + confidenceScore + directionBonus

	// 限制在 0-100 范围
	if totalScore > 100 {
		totalScore = 100
	}
	if totalScore < 0 {
		totalScore = 0
	}

	return totalScore
}

// ==================== TrendSurge (单边加速) ====================

// TrendSurge Pattern 类型
const (
	PatternTrendSurgeUp   PatternType = "TREND_SURGE_UP"   // 📈 单边上涨
	PatternTrendSurgeDown PatternType = "TREND_SURGE_DOWN" // 📉 单边下跌
)

// TrendSurgeInput 单边加速检测输入
type TrendSurgeInput struct {
	TimeRemaining int     // 剩余秒数
	CurrentPrice  float64 // 当前价格
	OpenPrice     float64 // 15m 开盘价
	StrikePrice   float64 // 行权价
	VWAP          float64 // 成交量加权平均价
	PolyPrice     float64 // Polymarket 合约价格 (0-1)
	ATR           float64 // 14 周期 ATR
	Sigma15m      float64 // 15m 波动率 (StdDev)
	Sigma30s      float64 // 30s 波动率 (突刺检测)

	// 技术指标
	MACD     *MACDResult // MACD 结果
	RSI      float64     // RSI 当前值
	RSISlope float64     // RSI 斜率
	VolSlope float64     // 成交量斜率 (归一化)

	// Heiken Ashi
	HeikenAshi []HeikenAshiCandle // 最近 3-5 根 HA 蜡烛

	// Phase 3 专用
	PriceVelocity float64 // 平滑后的价格速度 (每秒)
}

// TrendSurgeResult 单边加速检测结果
type TrendSurgeResult struct {
	Phase       int     // 阶段 (1, 2, 3, 4)
	Score       int     // 评分 (0-100)
	Side        string  // 方向 ("UP", "DOWN", "NONE")
	VetoReason  string  // 被否决原因 (如适用)
	ZScore      float64 // Z-Score (Phase 3)
	IsTriggered bool    // 是否触发 (Score >= 80 且无 Veto)
}

// DetectTrendSurge 检测单边加速形态
func DetectTrendSurge(input TrendSurgeInput) TrendSurgeResult {
	result := TrendSurgeResult{
		Side: "NONE",
	}

	// ========== 1. 阶段判定 ==========
	if input.TimeRemaining < 60 {
		result.Phase = 4
	} else if input.TimeRemaining < 180 {
		result.Phase = 3
	} else if input.TimeRemaining < 600 {
		result.Phase = 2
	} else {
		result.Phase = 1
	}

	// ========== 2. 硬性过滤 (Hard Vetoes) ==========

	// 2.1 Phase 4 禁区
	if result.Phase == 4 {
		result.VetoReason = "Phase 4 禁区 (剩余 < 60s)"
		return result
	}

	// 计算基础指标
	priceChange := 0.0
	if input.OpenPrice > 0 {
		priceChange = (input.CurrentPrice - input.OpenPrice) / input.OpenPrice
	}
	vwapDist := 0.0
	if input.VWAP > 0 {
		vwapDist = (input.CurrentPrice - input.VWAP) / input.VWAP
	}

	// 判断方向
	isUptrend := priceChange > 0

	// 2.2 趋势反转 (VWAP 位置检查)
	if isUptrend && input.CurrentPrice < input.VWAP {
		result.VetoReason = "趋势反转: 做多但价格 < VWAP"
		return result
	}
	if !isUptrend && input.CurrentPrice > input.VWAP {
		result.VetoReason = "趋势反转: 做空但价格 > VWAP"
		return result
	}

	// 2.3 动能背离 (Phase 1 & 2)
	if result.Phase <= 2 {
		if isUptrend && input.RSISlope < -0.1 { // 严格: -0.1
			result.VetoReason = "动能背离: 价格涨但 RSI 下降 (顶背离)"
			return result
		}
		if !isUptrend && input.RSISlope > 0.1 { // 严格: 0.1
			result.VetoReason = "动能背离: 价格跌但 RSI 上升 (底背离)"
			return result
		}
	}

	// 2.4 VWAP 过热
	maxVWAPDist := 0.01 // 严格限制在 1.0%
	if math.Abs(vwapDist) > maxVWAPDist {
		result.VetoReason = "VWAP 过热: 偏离度 > 1.0%"
		return result
	}

	// 2.5 量价背离 (Phase 1 特有)
	// [已放宽] 在启动阶段，允许缩量上涨 (惜售)，只要 VolSlope 不是剧烈下降即可
	if result.Phase == 1 && isUptrend && input.VolSlope < -0.1 {
		result.VetoReason = "量价背离: 严重缩量上涨"
		return result
	}

	// 2.6 波动率突刺 (Phase 3 特有)
	if result.Phase == 3 && input.Sigma15m > 0 {
		if input.Sigma30s > 2*input.Sigma15m {
			result.VetoReason = "波动率突刺: 黑天鹅预警"
			return result
		}
	}

	// 2.7 行权价缺失 (Phase 3 特有)
	if result.Phase == 3 && input.StrikePrice <= 0 {
		result.VetoReason = "行权价缺失: Phase 3 强制观察模式"
		return result
	}

	// 2.7.1 行权价异常检查 (Phase 3 特有)
	if result.Phase == 3 && input.StrikePrice > 0 && input.CurrentPrice > 0 {
		priceDiff := math.Abs(input.StrikePrice-input.CurrentPrice) / input.CurrentPrice
		if priceDiff > 0.2 { // 偏离超过 20%
			result.VetoReason = "行权价异常: 偏离当前价格过大"
			return result
		}
	}

	// 2.8 获利期望不足 (Minimum Edge)
	if input.PolyPrice > 0 && input.PolyPrice < 1 {
		var edge float64
		if isUptrend {
			// 做多: 买 YES，期望收益 = 1 - 买入价
			edge = 1 - input.PolyPrice
		} else {
			// 做空: 买 NO，期望收益 = 买入价 (input.PolyPrice 应该是 NO 的价格)
			edge = input.PolyPrice
		}
		if edge < 0.05 {
			result.VetoReason = "获利期望不足: 预期利润 < 5%"
			return result
		}
	}

	// ========== 3. 分阶段评分 ==========
	score := 0

	switch result.Phase {
	case 1:
		score = scoreTrendSurgePhase1(input, priceChange, isUptrend)
	case 2:
		score = scoreTrendSurgePhase2(input, priceChange, vwapDist, isUptrend, maxVWAPDist)
	case 3:
		score, result.ZScore = scoreTrendSurgePhase3(input, vwapDist, isUptrend)
	}

	result.Score = score

	// 设置方向
	if isUptrend {
		result.Side = "UP"
	} else {
		result.Side = "DOWN"
	}

	// 判断是否触发
	result.IsTriggered = score >= 80 // [严格] 80分触发

	return result
}

// scoreTrendSurgePhase1 Phase 1 评分 (趋势确立期)
func scoreTrendSurgePhase1(input TrendSurgeInput, priceChange float64, isUptrend bool) int {
	score := 0

	// 1. Price vs Open (20分)
	if math.Abs(priceChange) > 0.0008 { // 0.08%
		if (isUptrend && priceChange > 0) || (!isUptrend && priceChange < 0) {
			score += 20
		}
	}

	// 2. Heiken Ashi (30分) - [严格] 要求3连同色
	if len(input.HeikenAshi) >= 3 {
		color, count := CountConsecutive(input.HeikenAshi)
		if count >= 3 {
			// 检查最新一根是否无反向影线 (5%容错)
			latest := input.HeikenAshi[len(input.HeikenAshi)-1]
			isGreen := color == "green"
			if HasNoReverseShadow(latest, isGreen, 0.05) {
				score += 30
			} else {
				score += 15 // 有轻微影线，只给一半分
			}
		}
	}

	// 3. MACD (30分) - [严格] 要求柱状图同向 + 增强中
	if input.MACD != nil {
		if isUptrend && input.MACD.Histogram > 0 && input.MACD.HistDelta > 0 {
			score += 30
		} else if !isUptrend && input.MACD.Histogram < 0 && input.MACD.HistDelta < 0 {
			score += 30
		}
	}

	// 4. Vol Slope (10分) - [严格] 要求量增
	if input.VolSlope > 0 {
		score += 10
	}

	// 5. RSI Slope (10分)
	if math.Abs(input.RSISlope) > 0.2 {
		if (isUptrend && input.RSISlope > 0) || (!isUptrend && input.RSISlope < 0) {
			score += 10
		}
	}

	// 扣分项: 缩量上涨/下跌 (-20分)
	if input.VolSlope < 0 {
		score -= 20
	}

	if score < 0 {
		score = 0
	}

	return score
}

// scoreTrendSurgePhase2 Phase 2 评分 (趋势中继期)
func scoreTrendSurgePhase2(input TrendSurgeInput, priceChange, vwapDist float64, isUptrend bool, maxVWAPDist float64) int {
	score := 0

	// 1. Price vs Open (30分) - [严格] 必须脱离震荡区
	if math.Abs(priceChange) > 0.0015 { // 0.15%
		if (isUptrend && priceChange > 0) || (!isUptrend && priceChange < 0) {
			score += 30
		}
	}

	// 2. VWAP Dist (20分) - [严格] 健康偏离 0.1% - 0.8%
	absVWAPDist := math.Abs(vwapDist)
	if absVWAPDist > 0.001 && absVWAPDist < 0.008 { // 0.1% - 0.8%
		score += 20
	}

	// 3. Heiken Ashi (20分) - [严格] 要求3连同色
	if len(input.HeikenAshi) >= 3 {
		_, count := CountConsecutive(input.HeikenAshi)
		if count >= 3 {
			score += 20
		}
	}

	// 4. MACD (20分)
	if input.MACD != nil {
		if isUptrend && input.MACD.Histogram > 0 {
			score += 20
		} else if !isUptrend && input.MACD.Histogram < 0 {
			score += 20
		}
	}

	// 5. Vol Slope (10分)
	if input.VolSlope > -0.05 {
		score += 10
	}

	// 扣分项: 严重缩量 (-20分)
	if input.VolSlope < -0.1 {
		score -= 20
	}

	if score < 0 {
		score = 0
	}

	return score
}

// scoreTrendSurgePhase3 Phase 3 评分 (决议锁定期)
func scoreTrendSurgePhase3(input TrendSurgeInput, vwapDist float64, isUptrend bool) (int, float64) {
	score := 0

	// 计算 Z-Score
	zScore := 0.0
	if input.Sigma15m > 0 && input.StrikePrice > 0 {
		safetyMargin := input.CurrentPrice - input.StrikePrice
		if !isUptrend {
			safetyMargin = input.StrikePrice - input.CurrentPrice
		}
		zScore = safetyMargin / input.Sigma15m
	}

	// 1. Z-Score (60分)
	if math.Abs(zScore) > 2.5 {
		score += 60
	} else if math.Abs(zScore) > 1.5 {
		score += 30 // 部分分数
	}

	// 2. VWAP Support (20分)
	if isUptrend && vwapDist > 0 {
		score += 20
	} else if !isUptrend && vwapDist < 0 {
		score += 20
	}

	// 3. Heiken Ashi 未变色 (20分)
	if len(input.HeikenAshi) >= 1 {
		latest := input.HeikenAshi[len(input.HeikenAshi)-1]
		if (isUptrend && latest.IsGreen) || (!isUptrend && !latest.IsGreen) {
			score += 20
		}
	}

	// 4. 速度风险判定 (扣分项)
	if input.StrikePrice > 0 && input.PriceVelocity != 0 {
		safetyMargin := math.Abs(input.CurrentPrice - input.StrikePrice)
		timeRemainingSec := float64(input.TimeRemaining)
		projectedImpact := math.Abs(input.PriceVelocity) * timeRemainingSec

		// 检查速度方向是否指向行权价
		velocityTowardsStrike := false
		if isUptrend && input.PriceVelocity < 0 {
			velocityTowardsStrike = true
		} else if !isUptrend && input.PriceVelocity > 0 {
			velocityTowardsStrike = true
		}

		if velocityTowardsStrike && projectedImpact > 0.8*safetyMargin {
			score = 0 // 速度风险，直接归零
		}
	}

	return score, zScore
}

// GetTrendSurgeSideString 获取 TrendSurge 方向的中文描述
func GetTrendSurgeSideString(side string) string {
	switch side {
	case "UP":
		return "📈 买涨"
	case "DOWN":
		return "📉 买跌"
	default:
		return "🔍 观望"
	}
}
