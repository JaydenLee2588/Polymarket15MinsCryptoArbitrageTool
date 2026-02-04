package src

import (
	"math"
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
