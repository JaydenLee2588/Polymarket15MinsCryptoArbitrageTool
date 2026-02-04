package src

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Binance REST API 配置
const (
	BinanceRESTBaseURL = "https://api.binance.com"
	BinanceKlinesPath  = "/api/v3/klines"
	BinancePricePath   = "/api/v3/ticker/price"
)

// HistoricalKline 表示一根K线数据
type HistoricalKline struct {
	OpenTime  int64   // K线开盘时间 (毫秒)
	Open      float64 // 开盘价
	High      float64 // 最高价
	Low       float64 // 最低价
	Close     float64 // 收盘价
	Volume    float64 // 成交量
	CloseTime int64   // K线收盘时间 (毫秒)
}

// BinanceRESTClient Binance REST API 客户端
type BinanceRESTClient struct {
	httpClient *http.Client
	baseURL    string
	logger     Logger
}

// NewBinanceRESTClient 创建 Binance REST 客户端
func NewBinanceRESTClient(logger Logger) *BinanceRESTClient {
	if logger == nil {
		logger = NewSilentLogger()
	}
	return &BinanceRESTClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    BinanceRESTBaseURL,
		logger:     logger,
	}
}

// FetchKlines 获取K线历史数据
// symbol: 交易对，如 "BTCUSDT"
// interval: K线周期，如 "1m", "5m", "15m"
// limit: 获取的K线数量，最大 1500
func (c *BinanceRESTClient) FetchKlines(symbol, interval string, limit int) ([]HistoricalKline, error) {
	if limit <= 0 {
		limit = 240 // 默认 240 根
	}
	if limit > 1500 {
		limit = 1500 // Binance API 最大限制
	}

	url := fmt.Sprintf("%s%s?symbol=%s&interval=%s&limit=%d",
		c.baseURL, BinanceKlinesPath, symbol, interval, limit)

	c.logger.Debug("🌐 [Binance REST] 获取 K 线: %s", url)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Binance API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	var rawData [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	klines := make([]HistoricalKline, 0, len(rawData))
	for _, k := range rawData {
		if len(k) < 7 {
			continue
		}
		klines = append(klines, HistoricalKline{
			OpenTime:  int64(toFloat64(k[0])),
			Open:      toFloat64(k[1]),
			High:      toFloat64(k[2]),
			Low:       toFloat64(k[3]),
			Close:     toFloat64(k[4]),
			Volume:    toFloat64(k[5]),
			CloseTime: int64(toFloat64(k[6])),
		})
	}

	c.logger.Debug("✅ [Binance REST] 获取 %d 根 %s K 线", len(klines), interval)
	return klines, nil
}

// FetchKlineAtTime 获取指定时间戳的 K 线数据
// 用于获取 Polymarket 市场开始时刻的价格
// timestamp: Unix 时间戳 (秒)
func (c *BinanceRESTClient) FetchKlineAtTime(symbol string, timestamp int64) (*HistoricalKline, error) {
	// 将 Unix 秒转换为毫秒
	startTimeMs := timestamp * 1000
	// 获取该分钟及之后 1 分钟的 K 线
	endTimeMs := startTimeMs + 60*1000

	url := fmt.Sprintf("%s%s?symbol=%s&interval=1m&startTime=%d&endTime=%d&limit=1",
		c.baseURL, BinanceKlinesPath, symbol, startTimeMs, endTimeMs)

	c.logger.Debug("🌐 [Binance REST] 获取历史 K 线: %s", url)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Binance API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	var rawData [][]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	if len(rawData) == 0 {
		return nil, fmt.Errorf("未找到该时间戳的 K 线数据")
	}

	k := rawData[0]
	if len(k) < 7 {
		return nil, fmt.Errorf("K 线数据格式错误")
	}

	kline := &HistoricalKline{
		OpenTime:  int64(toFloat64(k[0])),
		Open:      toFloat64(k[1]),
		High:      toFloat64(k[2]),
		Low:       toFloat64(k[3]),
		Close:     toFloat64(k[4]),
		Volume:    toFloat64(k[5]),
		CloseTime: int64(toFloat64(k[6])),
	}

	c.logger.Debug("✅ [Binance REST] 获取历史价格: ts=%d, open=%.2f", timestamp, kline.Open)
	return kline, nil
}

// FetchLastPrice 获取指定交易对的最新价格
func (c *BinanceRESTClient) FetchLastPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s%s?symbol=%s", c.baseURL, BinancePricePath, symbol)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return 0, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("Binance API 错误 (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("JSON 解析失败: %w", err)
	}

	price, err := strconv.ParseFloat(result.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("价格转换失败: %w", err)
	}

	return price, nil
}

// FetchKlinesMultiInterval 同时获取多个周期的 K 线数据
// 返回一个 map，key 为 interval，value 为对应的 K 线数组
func (c *BinanceRESTClient) FetchKlinesMultiInterval(symbol string, intervals []string, limit int) (map[string][]HistoricalKline, error) {
	result := make(map[string][]HistoricalKline)

	for _, interval := range intervals {
		klines, err := c.FetchKlines(symbol, interval, limit)
		if err != nil {
			c.logger.Warn("⚠️ [Binance REST] 获取 %s K 线失败: %v", interval, err)
			continue
		}
		result[interval] = klines
	}

	return result, nil
}

// GetCloses 从 K 线数据中提取收盘价序列
func GetCloses(klines []HistoricalKline) []float64 {
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}
	return closes
}
