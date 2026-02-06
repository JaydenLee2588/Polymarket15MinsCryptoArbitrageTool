package src

import (
	"math"
)

// ==================== RSI (Relative Strength Index) ====================

// ComputeRSI 计算相对强弱指标
// closes: 收盘价序列
// period: 周期 (通常为 14)
// 返回 RSI 值 (0-100)
func ComputeRSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return 50 // 数据不足，返回中性值
	}

	gains := 0.0
	losses := 0.0

	// 计算最后 period 个周期的涨跌幅
	for i := len(closes) - period; i < len(closes); i++ {
		diff := closes[i] - closes[i-1]
		if diff > 0 {
			gains += diff
		} else {
			losses += -diff
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100 // 全是涨，RSI = 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - 100/(1+rs)

	return clampFloat(rsi, 0, 100)
}

// ComputeRSISeries 计算 RSI 序列
// 返回每个时间点的 RSI 值
func ComputeRSISeries(closes []float64, period int) []float64 {
	series := make([]float64, 0, len(closes))
	for i := 0; i < len(closes); i++ {
		sub := closes[:i+1]
		rsi := ComputeRSI(sub, period)
		series = append(series, rsi)
	}
	return series
}

// SMA 计算简单移动平均线
func SMA(values []float64, period int) float64 {
	if len(values) < period {
		return 0
	}
	slice := values[len(values)-period:]
	sum := 0.0
	for _, v := range slice {
		sum += v
	}
	return sum / float64(period)
}

// SlopeLast 计算最后 N 个点的斜率
func SlopeLast(values []float64, points int) float64 {
	if len(values) < points {
		return 0
	}
	slice := values[len(values)-points:]
	first := slice[0]
	last := slice[len(slice)-1]
	return (last - first) / float64(points-1)
}

// ==================== MACD (Moving Average Convergence Divergence) ====================

// MACDResult MACD 计算结果
type MACDResult struct {
	MACD      float64 // MACD 线 (快线 - 慢线)
	Signal    float64 // 信号线 (MACD 的 EMA)
	Histogram float64 // 柱状图 (MACD - Signal)
	HistDelta float64 // 柱状图变化量
}

// EMA 计算指数移动平均线
func EMA(values []float64, period int) float64 {
	if len(values) < period {
		return 0
	}

	k := 2.0 / float64(period+1)
	prev := values[0]
	for i := 1; i < len(values); i++ {
		prev = values[i]*k + prev*(1-k)
	}
	return prev
}

// ComputeMACD 计算 MACD 指标
// fast: 快线周期 (通常 12)
// slow: 慢线周期 (通常 26)
// signal: 信号线周期 (通常 9)
func ComputeMACD(closes []float64, fast, slow, signal int) *MACDResult {
	if len(closes) < slow+signal {
		return nil
	}

	fastEMA := EMA(closes, fast)
	slowEMA := EMA(closes, slow)
	macdLine := fastEMA - slowEMA

	// 计算 MACD 序列用于信号线
	macdSeries := make([]float64, 0)
	for i := 0; i < len(closes); i++ {
		sub := closes[:i+1]
		f := EMA(sub, fast)
		s := EMA(sub, slow)
		macdSeries = append(macdSeries, f-s)
	}

	signalLine := EMA(macdSeries, signal)
	histogram := macdLine - signalLine

	// 计算柱状图变化量
	var histDelta float64
	if len(macdSeries) >= signal+1 {
		prevSignal := EMA(macdSeries[:len(macdSeries)-1], signal)
		prevHist := macdSeries[len(macdSeries)-2] - prevSignal
		histDelta = histogram - prevHist
	}

	return &MACDResult{
		MACD:      macdLine,
		Signal:    signalLine,
		Histogram: histogram,
		HistDelta: histDelta,
	}
}

// ==================== VWAP (Volume Weighted Average Price) ====================

// ComputeSessionVWAP 计算会话级 VWAP
func ComputeSessionVWAP(klines []HistoricalKline) float64 {
	if len(klines) == 0 {
		return 0
	}

	var pv, v float64
	for _, k := range klines {
		tp := (k.High + k.Low + k.Close) / 3 // 典型价格
		pv += tp * k.Volume
		v += k.Volume
	}

	if v == 0 {
		return 0
	}
	return pv / v
}

// ComputeVWAPSeries 计算累积 VWAP 序列
func ComputeVWAPSeries(klines []HistoricalKline) []float64 {
	series := make([]float64, len(klines))
	for i := range klines {
		sub := klines[:i+1]
		series[i] = ComputeSessionVWAP(sub)
	}
	return series
}

// ==================== Heiken Ashi ====================

// HeikenAshiCandle Heiken Ashi 蜡烛
type HeikenAshiCandle struct {
	Open    float64
	High    float64
	Low     float64
	Close   float64
	IsGreen bool    // 是否为绿色（看涨）
	Body    float64 // 实体大小
}

// ComputeHeikenAshi 将普通 K 线转换为 Heiken Ashi 蜡烛
func ComputeHeikenAshi(klines []HistoricalKline) []HeikenAshiCandle {
	if len(klines) == 0 {
		return nil
	}

	ha := make([]HeikenAshiCandle, len(klines))

	for i, k := range klines {
		haClose := (k.Open + k.High + k.Low + k.Close) / 4

		var haOpen float64
		if i == 0 {
			haOpen = (k.Open + k.Close) / 2
		} else {
			haOpen = (ha[i-1].Open + ha[i-1].Close) / 2
		}

		haHigh := math.Max(k.High, math.Max(haOpen, haClose))
		haLow := math.Min(k.Low, math.Min(haOpen, haClose))

		ha[i] = HeikenAshiCandle{
			Open:    haOpen,
			High:    haHigh,
			Low:     haLow,
			Close:   haClose,
			IsGreen: haClose >= haOpen,
			Body:    math.Abs(haClose - haOpen),
		}
	}

	return ha
}

// CountConsecutive 统计连续同色 Heiken Ashi 蜡烛
// 返回颜色 ("green"/"red") 和连续数量
func CountConsecutive(haCandles []HeikenAshiCandle) (color string, count int) {
	if len(haCandles) == 0 {
		return "", 0
	}

	last := haCandles[len(haCandles)-1]
	targetGreen := last.IsGreen

	if targetGreen {
		color = "green"
	} else {
		color = "red"
	}

	count = 0
	for i := len(haCandles) - 1; i >= 0; i-- {
		if haCandles[i].IsGreen != targetGreen {
			break
		}
		count++
	}

	return color, count
}

// ==================== 辅助函数 ====================

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// CountVWAPCrosses 统计价格穿越 VWAP 的次数
func CountVWAPCrosses(closes, vwapSeries []float64, lookback int) int {
	if len(closes) < lookback || len(vwapSeries) < lookback {
		return 0
	}

	crosses := 0
	for i := len(closes) - lookback + 1; i < len(closes); i++ {
		prev := closes[i-1] - vwapSeries[i-1]
		cur := closes[i] - vwapSeries[i]
		if prev == 0 {
			continue
		}
		// 检测穿越
		if (prev > 0 && cur < 0) || (prev < 0 && cur > 0) {
			crosses++
		}
	}
	return crosses
}

// ==================== TrendSurge 辅助函数 ====================

// ComputeATR 计算平均真实波幅 (Average True Range)
// period: 周期 (通常为 14)
func ComputeATR(klines []HistoricalKline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}

	trSum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		k := klines[i]
		prevClose := klines[i-1].Close
		// 真实波幅 = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
		tr := math.Max(k.High-k.Low, math.Max(math.Abs(k.High-prevClose), math.Abs(k.Low-prevClose)))
		trSum += tr
	}

	return trSum / float64(period)
}

// ComputeStdDev 计算标准差
func ComputeStdDev(values []float64) float64 {
	n := len(values)
	if n < 2 {
		return 0
	}

	// 计算均值
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	// 计算方差
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(n)

	return math.Sqrt(variance)
}

// LinearRegressionSlope 计算线性回归斜率
// 返回归一化斜率 (斜率 / 均值)，便于跨币种比较
func LinearRegressionSlope(values []float64) float64 {
	n := len(values)
	if n < 2 {
		return 0
	}

	// 计算 x̄, ȳ
	sumX, sumY := 0.0, 0.0
	for i, v := range values {
		sumX += float64(i)
		sumY += v
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	// 如果均值为 0，无法归一化
	if meanY == 0 {
		return 0
	}

	// 计算斜率: Σ(xi - x̄)(yi - ȳ) / Σ(xi - x̄)²
	numerator, denominator := 0.0, 0.0
	for i, v := range values {
		xDiff := float64(i) - meanX
		yDiff := v - meanY
		numerator += xDiff * yDiff
		denominator += xDiff * xDiff
	}

	if denominator == 0 {
		return 0
	}

	slope := numerator / denominator
	// 归一化
	return slope / meanY
}

// HasNoReverseShadow 检查 Heiken Ashi 蜡烛是否无反向影线
// isUptrend: true = 检查下影线 (做多时), false = 检查上影线 (做空时)
// tolerance: 容错率 (推荐 0.05 = 5%)
func HasNoReverseShadow(ha HeikenAshiCandle, isUptrend bool, tolerance float64) bool {
	body := ha.Body
	if body == 0 {
		return false // 无实体，无法判断
	}

	var shadowLen float64
	if isUptrend {
		// 上涨时检查下影线
		shadowLen = math.Min(ha.Open, ha.Close) - ha.Low
	} else {
		// 下跌时检查上影线
		shadowLen = ha.High - math.Max(ha.Open, ha.Close)
	}

	// 影线长度 <= 实体长度 * 容错率
	return shadowLen <= body*tolerance
}

// GetVolumes 从 K 线数据中提取成交量序列
func GetVolumes(klines []HistoricalKline) []float64 {
	volumes := make([]float64, len(klines))
	for i, k := range klines {
		volumes[i] = k.Volume
	}
	return volumes
}

// ==================== Sniper v4.4 辅助函数 ====================

// Get1MinHighLow 获取最近1分钟的高低点
// 用于突破确认判定
func Get1MinHighLow(klines []HistoricalKline) (high, low float64) {
	if len(klines) == 0 {
		return 0, 0
	}

	// 取最后一根 K 线 (最近1分钟)
	last := klines[len(klines)-1]
	return last.High, last.Low
}

// Get1MinHighLowFromLast2 获取前一根K线的高低点 (排除当前未完成K线)
// 用于更准确的突破判定
func Get1MinHighLowFromLast2(klines []HistoricalKline) (high, low float64) {
	if len(klines) < 2 {
		if len(klines) == 1 {
			return klines[0].High, klines[0].Low
		}
		return 0, 0
	}

	// 取倒数第二根 K 线 (已完成的最近1分钟)
	prev := klines[len(klines)-2]
	return prev.High, prev.Low
}

// ComputeRealtimeSigma 计算实时波动率
// 使用最近 N 根 K 线的收盘价标准差
func ComputeRealtimeSigma(klines []HistoricalKline, period int) float64 {
	if len(klines) < period {
		period = len(klines)
	}
	if period == 0 {
		return 0
	}

	// 提取最近 period 根 K 线的收盘价
	closes := make([]float64, period)
	startIdx := len(klines) - period
	for i := 0; i < period; i++ {
		closes[i] = klines[startIdx+i].Close
	}

	return ComputeStdDev(closes)
}

// ComputeVolumeAverage 计算平均成交量
func ComputeVolumeAverage(klines []HistoricalKline, period int) float64 {
	if len(klines) == 0 {
		return 0
	}
	if period > len(klines) {
		period = len(klines)
	}

	var sum float64
	startIdx := len(klines) - period
	for i := startIdx; i < len(klines); i++ {
		sum += klines[i].Volume
	}
	return sum / float64(period)
}

// IsVolumeSpike 检测成交量是否暴增 (事件屏蔽用)
// 当前成交量 > 平均值 * multiplier 时返回 true
func IsVolumeSpike(klines []HistoricalKline, period int, multiplier float64) bool {
	if len(klines) < 2 {
		return false
	}

	avg := ComputeVolumeAverage(klines[:len(klines)-1], period) // 排除最后一根
	lastVol := klines[len(klines)-1].Volume

	return avg > 0 && lastVol > avg*multiplier
}
