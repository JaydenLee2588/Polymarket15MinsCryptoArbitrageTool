package src

import (
	"math"
	"testing"
)

// ==================== RSI Tests ====================

func TestComputeRSI_InsufficientData(t *testing.T) {
	closes := []float64{100, 101, 102}
	rsi := ComputeRSI(closes, 14)
	if rsi != 50 {
		t.Errorf("Expected 50 for insufficient data, got %f", rsi)
	}
}

func TestComputeRSI_AllGains(t *testing.T) {
	// 15个连续上涨的价格
	closes := make([]float64, 16)
	for i := range closes {
		closes[i] = float64(100 + i)
	}
	rsi := ComputeRSI(closes, 14)
	if rsi != 100 {
		t.Errorf("Expected 100 for all gains, got %f", rsi)
	}
}

func TestComputeRSI_AllLosses(t *testing.T) {
	// 15个连续下跌的价格
	closes := make([]float64, 16)
	for i := range closes {
		closes[i] = float64(115 - i)
	}
	rsi := ComputeRSI(closes, 14)
	if rsi != 0 {
		t.Errorf("Expected 0 for all losses, got %f", rsi)
	}
}

func TestComputeRSI_Neutral(t *testing.T) {
	// 交替涨跌
	closes := []float64{100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100, 101, 100}
	rsi := ComputeRSI(closes, 14)
	if rsi != 50 {
		t.Errorf("Expected 50 for alternating gains/losses, got %f", rsi)
	}
}

func TestSMA(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	sma := SMA(values, 5)
	expected := 8.0 // (6+7+8+9+10)/5
	if sma != expected {
		t.Errorf("Expected SMA=%f, got %f", expected, sma)
	}
}

func TestSlopeLast(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5}
	slope := SlopeLast(values, 3)
	expected := 1.0 // (5-3)/2
	if slope != expected {
		t.Errorf("Expected slope=%f, got %f", expected, slope)
	}
}

// ==================== MACD Tests ====================

func TestComputeMACD_InsufficientData(t *testing.T) {
	closes := make([]float64, 10)
	for i := range closes {
		closes[i] = float64(100 + i)
	}
	result := ComputeMACD(closes, 12, 26, 9)
	if result != nil {
		t.Errorf("Expected nil for insufficient data, got %+v", result)
	}
}

func TestComputeMACD_ValidData(t *testing.T) {
	// 生成足够的数据
	closes := make([]float64, 50)
	for i := range closes {
		closes[i] = float64(100 + i)
	}
	result := ComputeMACD(closes, 12, 26, 9)
	if result == nil {
		t.Error("Expected valid MACD result, got nil")
		return
	}
	// 在持续上涨趋势中，MACD 应该为正
	if result.MACD <= 0 {
		t.Errorf("Expected positive MACD in uptrend, got %f", result.MACD)
	}
}

func TestEMA(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ema := EMA(values, 5)
	// EMA 应该偏向最近的值
	if ema <= 5.0 {
		t.Errorf("Expected EMA > 5.0, got %f", ema)
	}
}

// ==================== VWAP Tests ====================

func TestComputeSessionVWAP_Empty(t *testing.T) {
	vwap := ComputeSessionVWAP(nil)
	if vwap != 0 {
		t.Errorf("Expected 0 for empty input, got %f", vwap)
	}
}

func TestComputeSessionVWAP_SingleCandle(t *testing.T) {
	klines := []HistoricalKline{
		{High: 110, Low: 90, Close: 100, Volume: 1000},
	}
	vwap := ComputeSessionVWAP(klines)
	expected := (110 + 90 + 100) / 3.0 // 典型价格
	if vwap != expected {
		t.Errorf("Expected VWAP=%f, got %f", expected, vwap)
	}
}

func TestComputeSessionVWAP_MultipleCandles(t *testing.T) {
	klines := []HistoricalKline{
		{High: 110, Low: 90, Close: 100, Volume: 1000},
		{High: 120, Low: 100, Close: 110, Volume: 2000},
	}
	vwap := ComputeSessionVWAP(klines)

	tp1 := (110 + 90 + 100) / 3.0
	tp2 := (120 + 100 + 110) / 3.0
	expected := (tp1*1000 + tp2*2000) / 3000.0

	if math.Abs(vwap-expected) > 0.001 {
		t.Errorf("Expected VWAP=%f, got %f", expected, vwap)
	}
}

func TestComputeVWAPSeries(t *testing.T) {
	klines := []HistoricalKline{
		{High: 110, Low: 90, Close: 100, Volume: 1000},
		{High: 120, Low: 100, Close: 110, Volume: 2000},
		{High: 130, Low: 110, Close: 120, Volume: 1500},
	}
	series := ComputeVWAPSeries(klines)
	if len(series) != 3 {
		t.Errorf("Expected 3 VWAP values, got %d", len(series))
	}
}

// ==================== Heiken Ashi Tests ====================

func TestComputeHeikenAshi_Empty(t *testing.T) {
	ha := ComputeHeikenAshi(nil)
	if ha != nil {
		t.Errorf("Expected nil for empty input, got %v", ha)
	}
}

func TestComputeHeikenAshi_SingleCandle(t *testing.T) {
	klines := []HistoricalKline{
		{Open: 100, High: 110, Low: 90, Close: 105},
	}
	ha := ComputeHeikenAshi(klines)
	if len(ha) != 1 {
		t.Fatalf("Expected 1 HA candle, got %d", len(ha))
	}

	// haClose = (O+H+L+C)/4 = (100+110+90+105)/4 = 101.25
	expectedClose := (100.0 + 110 + 90 + 105) / 4
	if ha[0].Close != expectedClose {
		t.Errorf("Expected HA close=%f, got %f", expectedClose, ha[0].Close)
	}
}

func TestCountConsecutive_AllGreen(t *testing.T) {
	ha := []HeikenAshiCandle{
		{IsGreen: true},
		{IsGreen: true},
		{IsGreen: true},
	}
	color, count := CountConsecutive(ha)
	if color != "green" || count != 3 {
		t.Errorf("Expected green/3, got %s/%d", color, count)
	}
}

func TestCountConsecutive_MixedEndsRed(t *testing.T) {
	ha := []HeikenAshiCandle{
		{IsGreen: true},
		{IsGreen: true},
		{IsGreen: false},
		{IsGreen: false},
	}
	color, count := CountConsecutive(ha)
	if color != "red" || count != 2 {
		t.Errorf("Expected red/2, got %s/%d", color, count)
	}
}

func TestCountConsecutive_Empty(t *testing.T) {
	color, count := CountConsecutive(nil)
	if color != "" || count != 0 {
		t.Errorf("Expected empty/0, got %s/%d", color, count)
	}
}

// ==================== VWAP Crosses Test ====================

func TestCountVWAPCrosses(t *testing.T) {
	closes := []float64{100, 102, 98, 103, 97, 105}
	vwap := []float64{100, 100, 100, 100, 100, 100}

	crosses := CountVWAPCrosses(closes, vwap, 5)
	// 从 102(上) -> 98(下) = 1次
	// 从 98(下) -> 103(上) = 1次
	// 从 103(上) -> 97(下) = 1次
	// 从 97(下) -> 105(上) = 1次
	// 总共 4 次穿越
	if crosses != 4 {
		t.Errorf("Expected 4 crosses, got %d", crosses)
	}
}
