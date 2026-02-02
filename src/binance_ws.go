package src

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"Polymarket15MinsCryptoArbitrageTool/internal/datarecorder"
)

// Binance 合约 WebSocket URL
const binanceWSBaseURL = "wss://fstream.binance.com/stream?streams="

// KlineData 存储单个币种的 K 线数据
type KlineData struct {
	Symbol        string    // BTCUSDT, ETHUSDT 等
	OpenPrice     float64   // 本周期开盘价
	CurrentPrice  float64   // 当前最新价
	HighPrice     float64   // 最高价
	LowPrice      float64   // 最低价
	ChangePercent float64   // 涨跌幅 (当前价 - 开盘价) / 开盘价
	Trend         string    // "UP" 或 "DOWN"
	KlineStartMs  int64     // K线开始时间 (毫秒)
	KlineEndMs    int64     // K线结束时间 (毫秒)
	ElapsedSec    int64     // K线已经走了多少秒
	IsClosed      bool      // K线是否已完结
	LastUpdateMs  int64     // 最后更新时间
	LastUpdate    time.Time // 最后更新时间 (Go Time)
}

// BinanceKlineClient 管理 Binance WebSocket 连接
type BinanceKlineClient struct {
	conn       *websocket.Conn
	Klines     map[string]*KlineData // Key: Symbol (BTCUSDT, ETHUSDT, etc.)
	mu         sync.RWMutex
	done       chan struct{}
	symbols    []string
	logger     Logger
	timeOffset int64 // Binance 服务器时间与本地时间的偏差 (毫秒): Binance - Local
	calibrated bool  // 是否已完成时间校准

	// 僵死连接检测
	lastMessageTime time.Time    // 最后收到消息的时间
	lastMessageMu   sync.RWMutex // 保护 lastMessageTime
	reconnecting    bool         // 是否正在重连中
	reconnectMu     sync.Mutex   // 防止并发重连

	// 关闭状态 (防止重复关闭 panic)
	closeOnce sync.Once
}

// binanceStreamMsg 币安流式消息外层
type binanceStreamMsg struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

// binanceKlineEvent 币安 K 线事件
type binanceKlineEvent struct {
	E interface{}  `json:"E"` // 事件时间 (可能为 int64 或 string)
	S string       `json:"s"` // 交易对
	K binanceKline `json:"k"` // K线数据
}

// binanceKline K线详情
// binanceKline K线详情
type binanceKline struct {
	T      int64       `json:"t"` // K线开始时间
	TClose int64       `json:"T"` // K线结束时间
	S      string      `json:"s"` // 交易对
	I      string      `json:"i"` // 时间间隔
	O      interface{} `json:"o"` // 开盘价 (string or number)
	C      interface{} `json:"c"` // 收盘价 (string or number)
	H      interface{} `json:"h"` // 最高价 (string or number)
	L      interface{} `json:"l"` // 最低价 (string or number)
	V      interface{} `json:"v"` // 成交量 (string or number)
	N      int         `json:"n"` // 成交笔数
	X      bool        `json:"x"` // K线是否完结
}

// toFloat64 安全地将 interface{} 转换为 float64
func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

// ... unchanged ...

// updateKline 更新 K线数据
func (c *BinanceKlineClient) updateKline(symbol string, k binanceKline) {
	openPrice := toFloat64(k.O)
	currentPrice := toFloat64(k.C)
	highPrice := toFloat64(k.H)
	lowPrice := toFloat64(k.L)

	// 计算涨跌幅
	changePercent := 0.0
	if openPrice > 0 {
		changePercent = (currentPrice - openPrice) / openPrice
	}

	// 时间校准: 使用 Binance K 线的开始时间戳来校准本地时间偏差
	// 只在第一次收到消息时校准
	localNowMs := time.Now().UnixMilli()
	if !c.calibrated && k.T > 0 {
		// K 线开始时间应该是 15 分钟周期内的时间
		// 偏差 = Binance 当前时间估算 - 本地时间
		// Binance 当前时间 ≈ K 线开始 + 已过时间 (但我们没有 Binance 当前时间, 所以用 K 线时间间接判断)
		expectedKlineAge := localNowMs - k.T
		// 如果 K 线年龄不在合理范围 (0-900秒), 可能有时钟偏差
		if expectedKlineAge < 0 || expectedKlineAge > 900000 {
			c.timeOffset = 0 // 偏差太大, 忽略校准
			c.logger.Warn("⚠️ [Binance] 时间偏差异常: K线开始=%d, 本地时间=%d, 差值=%dms",
				k.T, localNowMs, expectedKlineAge)
		} else {
			c.timeOffset = 0 // 正常情况, 偏差很小
			c.logger.Info("🕐 [Binance] 时间校准完成: K线年龄=%dms, 偏差在合理范围内", expectedKlineAge)
		}
		c.calibrated = true
	}

	// 计算 K 线已经走了多久 (使用校准后的本地时间)
	nowMs := localNowMs + c.timeOffset
	elapsedMs := nowMs - k.T
	elapsedSec := elapsedMs / 1000

	// 判断趋势
	trend := "NEUTRAL"
	if changePercent > 0.001 { // > 0.1%
		trend = "UP"
	} else if changePercent < -0.001 { // < -0.1%
		trend = "DOWN"
	}

	c.mu.Lock()
	if data, exists := c.Klines[symbol]; exists {
		data.OpenPrice = openPrice
		data.CurrentPrice = currentPrice
		data.HighPrice = highPrice
		data.LowPrice = lowPrice
		data.ChangePercent = changePercent
		data.Trend = trend
		data.KlineStartMs = k.T
		data.KlineEndMs = k.TClose
		data.ElapsedSec = elapsedSec
		data.IsClosed = k.X
		data.LastUpdateMs = nowMs
		data.LastUpdate = time.Now()
	}
	c.mu.Unlock()

	// Record to Data Recorder
	if recorder := datarecorder.GetInstance(); recorder != nil {
		recorder.RecordKline(symbol, openPrice, highPrice, lowPrice, currentPrice, toFloat64(k.V), k.T)
	}
}

// NewBinanceKlineClient 创建新的 Binance K线客户端
// symbols: 要订阅的交易对列表，如 ["BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT"]
func NewBinanceKlineClient(symbols []string, logger Logger) *BinanceKlineClient {
	if logger == nil {
		logger = NewSilentLogger()
	}
	return &BinanceKlineClient{
		Klines:          make(map[string]*KlineData),
		symbols:         symbols,
		done:            make(chan struct{}),
		logger:          logger,
		lastMessageTime: time.Now(),
	}
}

// Connect 连接到 Binance WebSocket 并开始监听
func (c *BinanceKlineClient) Connect() error {
	// 构建订阅流 URL
	// 格式: btcusdt@kline_15m/ethusdt@kline_15m/...
	streams := ""
	for i, symbol := range c.symbols {
		if i > 0 {
			streams += "/"
		}
		streams += fmt.Sprintf("%s@kline_15m", toLower(symbol))
	}

	url := binanceWSBaseURL + streams
	c.logger.Info("🔌 [Binance] 正在连接: %s", url)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("Binance WebSocket 连接失败: %w", err)
	}

	c.conn = conn
	c.logger.Info("✅ [Binance] WebSocket 已连接，监听 %d 个交易对", len(c.symbols))

	// 初始化 Klines map
	c.mu.Lock()
	for _, symbol := range c.symbols {
		c.Klines[symbol] = &KlineData{
			Symbol: symbol,
		}
	}
	c.mu.Unlock()

	// 初始化最后消息时间
	c.lastMessageMu.Lock()
	c.lastMessageTime = time.Now()
	c.lastMessageMu.Unlock()

	// 启动消息处理协程
	go c.readLoop()

	// 启动健康检查协程 (僵死连接检测)
	go c.healthCheck()

	return nil
}

// readLoop 持续读取 WebSocket 消息
func (c *BinanceKlineClient) readLoop() {
	defer func() {
		if c.conn != nil {
			c.conn.Close()
		}
	}()

	for {
		select {
		case <-c.done:
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				c.logger.Warn("⚠️ [Binance] 读取错误: %v", err)
				// 尝试重连
				time.Sleep(5 * time.Second)
				if reconnErr := c.reconnect(); reconnErr != nil {
					c.logger.Error("❌ [Binance] 重连失败: %v", reconnErr)
					return
				}
				continue
			}

			c.processMessage(message)
		}
	}
}

// reconnect 重新连接
func (c *BinanceKlineClient) reconnect() error {
	streams := ""
	for i, symbol := range c.symbols {
		if i > 0 {
			streams += "/"
		}
		streams += fmt.Sprintf("%s@kline_15m", toLower(symbol))
	}

	url := binanceWSBaseURL + streams
	c.logger.Info("🔄 [Binance] 正在重连...")

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	c.conn = conn
	c.logger.Info("✅ [Binance] 重连成功")
	return nil
}

// processMessage 处理收到的消息
func (c *BinanceKlineClient) processMessage(msg []byte) {
	// 更新最后收到消息的时间 (用于僵死连接检测)
	c.lastMessageMu.Lock()
	c.lastMessageTime = time.Now()
	c.lastMessageMu.Unlock()

	var streamMsg binanceStreamMsg
	if err := json.Unmarshal(msg, &streamMsg); err != nil {
		c.logger.Warn("⚠️ [Binance] JSON 解析失败: %v", err)
		return
	}

	var event binanceKlineEvent
	if err := json.Unmarshal(streamMsg.Data, &event); err != nil {
		c.logger.Warn("⚠️ [Binance] K线事件解析失败: %v", err)
		return
	}

	c.updateKline(event.S, event.K)
}

// healthCheck 定期检查连接健康状态，如果长时间没有收到消息则触发重连
func (c *BinanceKlineClient) healthCheck() {
	const (
		checkInterval  = 30 * time.Second // 检查间隔
		staleThreshold = 60 * time.Second // 超过60秒没收到消息视为僵死
		reconnectDelay = 3 * time.Second  // 重连前等待时间
	)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.lastMessageMu.RLock()
			lastTime := c.lastMessageTime
			c.lastMessageMu.RUnlock()

			sinceLastMessage := time.Since(lastTime)

			// 如果超过阈值时间没有收到消息，触发重连
			if sinceLastMessage > staleThreshold {
				c.logger.Warn("⚠️ [Binance] 连接僵死，%.0f秒未收到消息，正在触发重连...", sinceLastMessage.Seconds())

				// 防止并发重连
				c.reconnectMu.Lock()
				if c.reconnecting {
					c.reconnectMu.Unlock()
					c.logger.Info("🔄 [Binance] 已有重连进行中，跳过")
					continue
				}
				c.reconnecting = true
				c.reconnectMu.Unlock()

				// 关闭旧连接
				if c.conn != nil {
					c.conn.Close()
				}

				// 等待一下再重连
				time.Sleep(reconnectDelay)

				// 尝试重连
				if err := c.reconnect(); err != nil {
					c.logger.Error("❌ [Binance] 健康检查触发的重连失败: %v", err)
				} else {
					c.logger.Info("✅ [Binance] 健康检查触发的重连成功")
					// 重置最后消息时间
					c.lastMessageMu.Lock()
					c.lastMessageTime = time.Now()
					c.lastMessageMu.Unlock()
				}

				c.reconnectMu.Lock()
				c.reconnecting = false
				c.reconnectMu.Unlock()
			}
		}
	}
}

// GetTrend 获取指定交易对的当前趋势
// 返回 KlineData 和是否存在
func (c *BinanceKlineClient) GetTrend(symbol string) (*KlineData, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, exists := c.Klines[symbol]
	if !exists || data.LastUpdateMs == 0 {
		return nil, false
	}

	// 复制一份返回，避免数据竞争
	result := &KlineData{
		Symbol:        data.Symbol,
		OpenPrice:     data.OpenPrice,
		CurrentPrice:  data.CurrentPrice,
		HighPrice:     data.HighPrice,
		LowPrice:      data.LowPrice,
		ChangePercent: data.ChangePercent,
		Trend:         data.Trend,
		KlineStartMs:  data.KlineStartMs,
		KlineEndMs:    data.KlineEndMs,
		ElapsedSec:    data.ElapsedSec,
		IsClosed:      data.IsClosed,
		LastUpdateMs:  data.LastUpdateMs,
		LastUpdate:    data.LastUpdate,
	}
	return result, true
}

// GetAllTrends 获取所有交易对的趋势
func (c *BinanceKlineClient) GetAllTrends() map[string]*KlineData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*KlineData)
	for symbol, data := range c.Klines {
		if data.LastUpdateMs == 0 {
			continue
		}
		result[symbol] = &KlineData{
			Symbol:        data.Symbol,
			OpenPrice:     data.OpenPrice,
			CurrentPrice:  data.CurrentPrice,
			HighPrice:     data.HighPrice,
			LowPrice:      data.LowPrice,
			ChangePercent: data.ChangePercent,
			Trend:         data.Trend,
			KlineStartMs:  data.KlineStartMs,
			KlineEndMs:    data.KlineEndMs,
			ElapsedSec:    data.ElapsedSec,
			IsClosed:      data.IsClosed,
			LastUpdateMs:  data.LastUpdateMs,
			LastUpdate:    data.LastUpdate,
		}
	}
	return result
}

// Close 关闭连接 (幂等，可多次调用)
func (c *BinanceKlineClient) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.conn != nil {
			c.conn.Close()
		}
		c.logger.Info("🔌 [Binance] WebSocket 已关闭")
	})
}

// toLower 将字符串转为小写 (简单实现，避免引入 strings 包)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// SymbolToBinance 将 Polymarket 的符号转换为 Binance 交易对
func SymbolToBinance(symbol string) string {
	switch symbol {
	case "Bitcoin", "BTC":
		return "BTCUSDT"
	case "Ethereum", "ETH":
		return "ETHUSDT"
	case "Solana", "SOL":
		return "SOLUSDT"
	case "XRP":
		return "XRPUSDT"
	default:
		return ""
	}
}

// SimpleKline 简化的K线数据结构
type SimpleKline struct {
	Open  float64
	Close float64
	High  float64
	Low   float64
}

// GetLatestKline 获取指定交易对的最新 K 线数据
func (c *BinanceKlineClient) GetLatestKline(symbol string) *SimpleKline {
	data, ok := c.GetTrend(symbol)
	if !ok || data == nil {
		return nil
	}
	return &SimpleKline{
		Open:  data.OpenPrice,
		Close: data.CurrentPrice,
		High:  data.HighPrice,
		Low:   data.LowPrice,
	}
}
