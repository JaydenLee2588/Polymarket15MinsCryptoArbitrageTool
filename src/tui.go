package src

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ==================== TUI 套利监控模块 ====================

// TUI 颜色常量
const (
	TUIColorReset  = "\033[0m"
	TUIColorRed    = "\033[1;31m"
	TUIColorGreen  = "\033[1;32m"
	TUIColorYellow = "\033[1;33m"
	TUIColorCyan   = "\033[1;36m"
	TUIColorBold   = "\033[1m"
	TUIClearLine   = "\033[K" // 清除从光标到行尾
)

// TUIMarketData 单个市场的 TUI 数据
type TUIMarketData struct {
	Slug            string
	Symbol          string
	TokenIDUp       string
	TokenIDDown     string
	PolymarketUp    float64
	PolymarketDown  float64
	ProbabilityUp   float64
	ProbabilityDown float64
	EdgeUp          float64
	EdgeDown        float64
	TimeLeft        string
}

// TUISimulatedOrder 模拟订单
type TUISimulatedOrder struct {
	Time     time.Time
	Side     string // "UP" or "DOWN"
	Price    float64
	Edge     float64
	Pattern  string
	ConfigID string
}

// TUIOrderHandler 下单处理器接口 (用于真实/模拟切换)
type TUIOrderHandler interface {
	PlaceOrder(side string, price, edge float64, pattern, configID string) error
}

// SimulatedOrderHandler 模拟下单处理器
type SimulatedOrderHandler struct {
	orders []TUISimulatedOrder
	mu     sync.Mutex
}

func NewSimulatedOrderHandler() *SimulatedOrderHandler {
	return &SimulatedOrderHandler{
		orders: make([]TUISimulatedOrder, 0),
	}
}

func (h *SimulatedOrderHandler) PlaceOrder(side string, price, edge float64, pattern, configID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	order := TUISimulatedOrder{
		Time:     time.Now(),
		Side:     side,
		Price:    price,
		Edge:     edge,
		Pattern:  pattern,
		ConfigID: configID,
	}
	h.orders = append(h.orders, order)

	// 只保留最近 10 条
	if len(h.orders) > 10 {
		h.orders = h.orders[len(h.orders)-10:]
	}

	return nil
}

func (h *SimulatedOrderHandler) GetRecentOrders() []TUISimulatedOrder {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]TUISimulatedOrder, len(h.orders))
	copy(result, h.orders)
	return result
}

// TUIStrategyConfig TUI 使用的策略配置 (从 api.StrategyConfig 简化)
type TUIStrategyConfig struct {
	ID               string
	MinPriceEdge     float64
	SharePriceLevel3 float64 // 最高价格阈值 (高于此不买)
	TradingMode      string  // "simulation" or "real"
}

// TUIMarketInfo TUI 需要的市场信息 (from api.TUIMarketInfo)
type TUIMarketInfo struct {
	UpPrice     float64
	DownPrice   float64
	Slug        string
	Symbol      string
	EndTime     time.Time
	StrikePrice float64 // 基准价 (from Polymarket API)
	HasData     bool
}

// ArbitrageTUI TUI 监控器
type ArbitrageTUI struct {
	mu          sync.RWMutex
	running     bool
	stopChan    chan struct{}
	refreshRate time.Duration
	logger      Logger

	// 外部依赖
	binanceClient      *BinanceRESTClient
	getMarketInfo      func() TUIMarketInfo
	getStrategyConfigs func() []TUIStrategyConfig
	isStrategyRunning  func() bool

	// 下单处理器
	orderHandler TUIOrderHandler

	// 当前显示数据
	currentPrice           float64
	strikePrice            float64   // 行权价
	lastStrikePriceAttempt time.Time // 上次尝试获取 Strike Price 的时间 (用于限流)
	marketSlug             string    // 市场 Slug
	marketEndTime          time.Time
	polymarketUp           float64
	polymarketDown         float64
	probabilityUp          float64
	probabilityDown        float64

	// 最近订单
	recentOrders []TUISimulatedOrder
}

// TUIConfig TUI 配置
type TUIConfig struct {
	RefreshRate        time.Duration
	Logger             Logger
	GetMarketInfo      func() TUIMarketInfo
	GetStrategyConfigs func() []TUIStrategyConfig
	IsStrategyRunning  func() bool
}

// NewArbitrageTUI 创建 TUI 实例
func NewArbitrageTUI(config TUIConfig) *ArbitrageTUI {
	if config.RefreshRate == 0 {
		config.RefreshRate = 1 * time.Second
	}
	if config.Logger == nil {
		config.Logger = NewSilentLogger()
	}

	return &ArbitrageTUI{
		refreshRate:        config.RefreshRate,
		logger:             config.Logger,
		stopChan:           make(chan struct{}),
		binanceClient:      NewBinanceRESTClient(config.Logger),
		getMarketInfo:      config.GetMarketInfo,
		getStrategyConfigs: config.GetStrategyConfigs,
		isStrategyRunning:  config.IsStrategyRunning,
		orderHandler:       NewSimulatedOrderHandler(),
	}
}

// SetOrderHandler 设置下单处理器 (用于切换真实/模拟)
func (t *ArbitrageTUI) SetOrderHandler(handler TUIOrderHandler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.orderHandler = handler
}

// Start 启动 TUI 刷新循环
func (t *ArbitrageTUI) Start() {
	// 在 Windows 上启用虚拟终端处理
	enableVirtualTerminal()

	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return
	}
	t.running = true
	t.stopChan = make(chan struct{})
	t.mu.Unlock()

	go t.runLoop()
}

// Stop 停止 TUI
func (t *ArbitrageTUI) Stop() {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return
	}
	t.running = false
	close(t.stopChan)
	t.mu.Unlock()
}

// IsRunning 检查是否正在运行
func (t *ArbitrageTUI) IsRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running
}

// runLoop TUI 主循环
func (t *ArbitrageTUI) runLoop() {
	ticker := time.NewTicker(t.refreshRate)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			t.refresh()
		}
	}
}

// refresh 刷新显示
func (t *ArbitrageTUI) refresh() {
	t.updateData()
	t.clearScreen()
	t.renderHeader()
	t.renderTechnicalIndicators()
	t.renderPriceComparison()
	t.renderRecentOrders()
	t.renderFooter()
}

// updateData 更新数据
func (t *ArbitrageTUI) updateData() {
	// 从策略获取实时 Polymarket 市场信息
	if t.getMarketInfo != nil && t.isStrategyRunning != nil && t.isStrategyRunning() {
		info := t.getMarketInfo()
		if info.HasData {
			// 验证价格是否有效 (排除极端值 0, 1)
			if isValidPolymarketPrice(info.UpPrice) && isValidPolymarketPrice(info.DownPrice) {
				t.polymarketUp = info.UpPrice
				t.polymarketDown = info.DownPrice
			}
			// 检测市场 slug 变化，重置 Strike Price
			if t.marketSlug != info.Slug {
				t.strikePrice = 0 // 重置，等待重新获取
			}
			// 更新市场信息
			t.marketSlug = info.Slug
			t.marketEndTime = info.EndTime
			// 使用 API 返回的 StrikePrice (从 GroupItemThreshold 解析)
			if info.StrikePrice > 0 {
				t.strikePrice = info.StrikePrice
			}
		}
	}
}

// isValidPolymarketPrice 验证 Polymarket 价格是否有效
// 有效价格应在 0.01 到 0.99 之间 (排除极端值)
func isValidPolymarketPrice(price float64) bool {
	return price > 0.01 && price < 0.99
}

// parseTimestampFromSlug 从 Slug 解析 Polymarket 市场开始时间戳
// 例如 "btc-updown-15m-1770173100" -> 1770173100
func parseTimestampFromSlug(slug string) int64 {
	parts := strings.Split(slug, "-")
	if len(parts) < 1 {
		return 0
	}
	// 时间戳通常是最后一个部分，且是 Unix 时间戳 (>1000000000)
	lastPart := parts[len(parts)-1]
	var ts int64
	if _, err := fmt.Sscanf(lastPart, "%d", &ts); err == nil {
		// Unix 时间戳范围检查: 2020年到2030年
		if ts > 1577836800 && ts < 1893456000 {
			return ts
		}
	}
	return 0
}

// clearScreen 移动光标到起始位置 (不清屏)
func (t *ArbitrageTUI) clearScreen() {
	// 只移动光标到左上角，不清屏
	// \033[H = 光标移动到 (1,1)
	fmt.Print("\033[H")
}

// TUI 显示宽度
const tuiLineWidth = 60

// padLine 将字符串填充到固定宽度
func padLine(s string) string {
	// 计算可见字符长度 (忽略 ANSI 转义序列)
	visibleLen := 0
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
		} else if inEscape && r == 'm' {
			inEscape = false
		} else if !inEscape {
			visibleLen++
		}
	}

	if visibleLen >= tuiLineWidth {
		return s
	}

	// 用空格填充到固定宽度
	padding := strings.Repeat(" ", tuiLineWidth-visibleLen)
	return s + padding
}

// tuiPrint 打印一行 (固定宽度填充)
func tuiPrint(s string) {
	fmt.Println(padLine(s))
}

// tuiPrintf 格式化打印 (固定宽度填充)
func tuiPrintf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	// 移除末尾换行符后填充，再加回换行符
	s = strings.TrimSuffix(s, "\n")
	fmt.Println(padLine(s))
}

// renderHeader 渲染头部
func (t *ArbitrageTUI) renderHeader() {
	strategyStatus := TUIColorRed + "未启动" + TUIColorReset
	if t.isStrategyRunning != nil && t.isStrategyRunning() {
		strategyStatus = TUIColorGreen + "运行中" + TUIColorReset
	}

	tuiPrint("==================================================")
	tuiPrint("   🚀 POLYMARKET ARBITRAGE MONITOR 🚀   ")
	tuiPrint("==================================================")
	tuiPrintf("Time: %s  |  Strategy: %s\n", time.Now().Format("15:04:05"), strategyStatus)
	tuiPrintf("Time Left: %s\n", t.getTimeUntilClose())
	tuiPrint("")
}

// renderTechnicalIndicators 渲染技术指标
func (t *ArbitrageTUI) renderTechnicalIndicators() {
	// 获取 BTC 实时数据
	symbol := "BTCUSDT"
	klines, err := t.binanceClient.FetchKlines(symbol, "1m", 240)
	if err != nil {
		tuiPrintf("📉 Technical Indicators: Error fetching data\n")
		tuiPrint("")
		return
	}

	closes := GetCloses(klines)
	if len(closes) < 50 {
		tuiPrintf("📉 Technical Indicators: Waiting for data...\n")
		tuiPrint("")
		return
	}

	currentPrice := closes[len(closes)-1]
	t.currentPrice = currentPrice

	rsi14 := ComputeRSI(closes, 14)
	rsiSlope := SlopeLast(ComputeRSISeries(closes, 14), 3)
	macd := ComputeMACD(closes, 12, 26, 9)
	vwap := ComputeSessionVWAP(klines)
	vwapDist := (currentPrice - vwap) / vwap
	ha := ComputeHeikenAshi(klines)
	haColor, haCount := CountConsecutive(ha)

	// 计算概率
	// 使用 Strike Price（如果有）或 Current Price
	priceToBeat := currentPrice

	// 如果 API 没有返回 Strike Price，从 Polymarket 市场开始时间获取 Binance 价格
	if t.strikePrice == 0 && t.binanceClient != nil && t.marketSlug != "" {
		// 限流: 失败后等待 10 秒再重试，避免 API 请求过多
		if time.Since(t.lastStrikePriceAttempt) > 10*time.Second {
			t.lastStrikePriceAttempt = time.Now()

			// 从 slug 解析 Polymarket 市场开始时间戳 (例如 "btc-updown-15m-1770173100" -> 1770173100)
			startTs := parseTimestampFromSlug(t.marketSlug)
			if startTs > 0 {
				// 获取该时间点的 Binance 1m K线数据
				kline, err := t.binanceClient.FetchKlineAtTime(symbol, startTs)
				if err == nil && kline != nil {
					t.strikePrice = kline.Open // Polymarket 市场开始时的 Binance 价格
				} else {
					// 记录错误失败日志 (仅在 Debug 模式或 TUI 底部显示可能不太合适，这里记录到 Logger)
					if t.logger != nil && err != nil {
						t.logger.Warn("⚠️ 无法获取 Strike Price (slug=%s, ts=%d): %v", t.marketSlug, startTs, err)
					}
				}
			}
		}
	}

	if t.strikePrice > 0 {
		priceToBeat = t.strikePrice
	}

	probInput := ProbabilityInput{
		Price:       currentPrice,
		PriceToBeat: priceToBeat,
		MinutesLeft: t.getFractionalMinutesLeft(),
		VWAP:        vwap,
		RSI:         rsi14,
		RSISlope:    rsiSlope,
		MACD:        macd,
		HeikenColor: haColor,
		HeikenCount: haCount,
	}
	prob := ScoreDirection(probInput)
	t.probabilityUp = prob.RawUp
	t.probabilityDown = prob.RawDown

	tuiPrintf("📊 Market Data (%s)\n", symbol)
	tuiPrintf("   Current Price:  %s%.2f%s\n", TUIColorBold, currentPrice, TUIColorReset)
	if t.strikePrice > 0 {
		tuiPrintf("   Strike Price:   %s%.2f%s (from Binance 15m Open)\n", TUIColorCyan, t.strikePrice, TUIColorReset)
	} else {
		tuiPrint("   Strike Price:   --  (等待市场数据)")
	}
	if t.marketSlug != "" {
		// 显示市场 Slug (用于调试)
		tuiPrintf("   Market Slug:    %s\n", t.marketSlug)
	}
	tuiPrint("")

	tuiPrint("📉 Technical Indicators (1m)")

	// RSI
	rsiColor := TUIColorReset
	if rsiSlope > 0 {
		rsiColor = TUIColorGreen
	} else if rsiSlope < 0 {
		rsiColor = TUIColorRed
	}
	tuiPrintf("   RSI (14):       %.2f  (Slope: %s%.4f%s)\n", rsi14, rsiColor, rsiSlope, TUIColorReset)

	// MACD
	tuiPrintf("   MACD趋势:       %s\n", t.getMACDDescription(macd))

	// VWAP
	vwapColor := TUIColorReset
	if vwapDist > 0 {
		vwapColor = TUIColorRed
	} else if vwapDist < 0 {
		vwapColor = TUIColorGreen
	}
	tuiPrintf("   VWAP:           %.2f  (Dist: %s%.4f%%%s)\n", vwap, vwapColor, vwapDist*100, TUIColorReset)

	// Heiken Ashi
	haText := haColor
	haTextColor := TUIColorReset
	if haColor == "green" {
		haText = "连涨"
		haTextColor = TUIColorGreen
	} else if haColor == "red" {
		haText = "连跌"
		haTextColor = TUIColorRed
	}
	tuiPrintf("   Heiken Ashi:    %s%s x %d%s\n", haTextColor, haText, haCount, TUIColorReset)
	tuiPrint("")

	// 形态检测
	prevRSI := 50.0
	if len(closes) > 15 {
		prevRSI = ComputeRSI(closes[:len(closes)-1], 14)
	}
	pattern := DetectPattern(PatternDetectInput{
		VWAPDist: vwapDist,
		MACD:     macd,
		RSI:      rsi14,
		RSISlope: rsiSlope,
		PrevRSI:  prevRSI,
	})

	actionColor := TUIColorReset
	if strings.Contains(pattern.ActionSuggestion, "买涨") {
		actionColor = TUIColorGreen
	} else if strings.Contains(pattern.ActionSuggestion, "买跌") {
		actionColor = TUIColorRed
	}
	recommendColor := TUIColorReset
	if strings.Contains(pattern.RecommendEntry, "推荐") {
		recommendColor = TUIColorYellow
	}

	tuiPrint("🧩 Pattern Detection")
	tuiPrintf("   Result:         %s\n", pattern.PatternName)
	tuiPrintf("   Signal:         %s\n", pattern.SignalType)
	tuiPrintf("   Action:         %s%s%s\n", actionColor, pattern.ActionSuggestion, TUIColorReset)
	tuiPrintf("   Recommend:      %s%s%s\n", recommendColor, pattern.RecommendEntry, TUIColorReset)
	tuiPrint("")

	// 检查是否应该模拟下单
	t.checkAndSimulateOrder(pattern)
}

// checkAndSimulateOrder 检查并执行模拟下单
func (t *ArbitrageTUI) checkAndSimulateOrder(pattern PatternResult) {
	// 获取策略配置
	var config TUIStrategyConfig
	if t.getStrategyConfigs != nil {
		configs := t.getStrategyConfigs()
		if len(configs) > 0 {
			config = configs[0]
		}
	}

	// 默认配置
	if config.MinPriceEdge == 0 {
		config.MinPriceEdge = 0.05 // 默认 5% Edge
	}
	if config.SharePriceLevel3 == 0 {
		config.SharePriceLevel3 = 0.70 // 默认最高价格
	}
	if config.ID == "" {
		config.ID = "TUI-Default"
	}

	// 计算 Edge
	edgeUp := t.probabilityUp - t.polymarketUp
	edgeDown := t.probabilityDown - t.polymarketDown

	// 检查下单条件
	if pattern.IsStrongRecommend() && t.orderHandler != nil {
		if pattern.IsBuyUp() && edgeUp >= config.MinPriceEdge && t.polymarketUp > 0 && t.polymarketUp < config.SharePriceLevel3 {
			t.orderHandler.PlaceOrder("UP", t.polymarketUp, edgeUp, pattern.PatternName, config.ID)
		} else if pattern.IsBuyDown() && edgeDown >= config.MinPriceEdge && t.polymarketDown > 0 && t.polymarketDown < config.SharePriceLevel3 {
			t.orderHandler.PlaceOrder("DOWN", t.polymarketDown, edgeDown, pattern.PatternName, config.ID)
		}
	}

	// 获取最近订单
	if handler, ok := t.orderHandler.(*SimulatedOrderHandler); ok {
		t.recentOrders = handler.GetRecentOrders()
	}
}

// renderPriceComparison 渲染价格对比
func (t *ArbitrageTUI) renderPriceComparison() {
	// 计算 Edge
	edgeUp := t.probabilityUp - t.polymarketUp
	edgeDown := t.probabilityDown - t.polymarketDown

	tuiPrint("🎲 Probability Price (Model):")
	tuiPrintf("   涨：%s%.2f%s\n", TUIColorGreen, t.probabilityUp, TUIColorReset)
	tuiPrintf("   跌：%s%.2f%s\n", TUIColorRed, t.probabilityDown, TUIColorReset)
	tuiPrint("")

	tuiPrint("💰 Polymarket Price (Market):")
	if t.polymarketUp > 0 || t.polymarketDown > 0 {
		tuiPrintf("   涨：%s%.2f%s\n", TUIColorGreen, t.polymarketUp, TUIColorReset)
		tuiPrintf("   跌：%s%.2f%s\n", TUIColorRed, t.polymarketDown, TUIColorReset)
	} else {
		tuiPrintf("   涨：%s--%s  (等待策略启动)\n", TUIColorGreen, TUIColorReset)
		tuiPrintf("   跌：%s--%s  (等待策略启动)\n", TUIColorRed, TUIColorReset)
	}
	tuiPrint("")

	tuiPrint("📈 Edge (Prob - Poly):")
	if t.polymarketUp > 0 || t.polymarketDown > 0 {
		edgeUpColor := TUIColorReset
		if edgeUp > 0 {
			edgeUpColor = TUIColorGreen
		} else if edgeUp < 0 {
			edgeUpColor = TUIColorRed
		}
		edgeDownColor := TUIColorReset
		if edgeDown > 0 {
			edgeDownColor = TUIColorGreen
		} else if edgeDown < 0 {
			edgeDownColor = TUIColorRed
		}
		tuiPrintf("   涨：%s%+.2f%s\n", edgeUpColor, edgeUp, TUIColorReset)
		tuiPrintf("   跌：%s%+.2f%s\n", edgeDownColor, edgeDown, TUIColorReset)
	} else {
		tuiPrintf("   涨：%s--%s\n", TUIColorGreen, TUIColorReset)
		tuiPrintf("   跌：%s--%s\n", TUIColorRed, TUIColorReset)
	}
	tuiPrint("")
}

// renderRecentOrders 渲染最近订单
func (t *ArbitrageTUI) renderRecentOrders() {
	tuiPrint("📋 Simulated Orders (Recent):")
	if len(t.recentOrders) == 0 {
		tuiPrint("   (无模拟订单)")
	} else {
		for _, order := range t.recentOrders {
			sideColor := TUIColorGreen
			sideText := "涨"
			if order.Side == "DOWN" {
				sideColor = TUIColorRed
				sideText = "跌"
			}
			tuiPrintf("   %s | %s%s%s @ %.2f | Edge: %.2f | %s\n",
				order.Time.Format("15:04:05"),
				sideColor, sideText, TUIColorReset,
				order.Price, order.Edge, order.Pattern)
		}
	}
	tuiPrint("")
}

// renderFooter 渲染底部
func (t *ArbitrageTUI) renderFooter() {
	// 显示策略配置
	var configInfo string
	if t.getStrategyConfigs != nil {
		configs := t.getStrategyConfigs()
		if len(configs) > 0 {
			cfg := configs[0]
			configInfo = fmt.Sprintf("Config: %s | MinEdge: %.2f | MaxPrice: %.2f",
				cfg.ID, cfg.MinPriceEdge, cfg.SharePriceLevel3)
		}
	}
	if configInfo == "" {
		configInfo = "Config: Default (TUI)"
	}

	tuiPrint("==================================================")
	tuiPrintf("%s\n", configInfo)
	tuiPrint("Press Ctrl+C to exit. Refreshing in 1s...")
}

// getMACDDescription 获取 MACD 描述
func (t *ArbitrageTUI) getMACDDescription(macd *MACDResult) string {
	if macd == nil {
		return "数据不足"
	}

	desc := ""
	color := ""

	if macd.Histogram > 0 && macd.HistDelta > 0 {
		desc = "看涨 (增强)"
		color = TUIColorGreen
	} else if macd.Histogram < 0 && macd.HistDelta < 0 {
		desc = "看跌 (增强)"
		color = TUIColorRed
	} else if macd.Histogram > 0 && macd.HistDelta < 0 {
		desc = "看涨 (减弱)"
		color = TUIColorGreen
	} else if macd.Histogram < 0 && macd.HistDelta > 0 {
		desc = "看跌 (减弱)"
		color = TUIColorRed
	} else {
		desc = "震荡"
		color = TUIColorYellow
	}

	if macd.Histogram > 0 && macd.HistDelta > 0 && macd.Histogram < 5 {
		desc += " [金叉初期]"
	} else if macd.Histogram < 0 && macd.HistDelta < 0 && macd.Histogram > -5 {
		desc += " [死叉初期]"
	}

	return color + desc + TUIColorReset
}

// getTimeUntilClose 计算距结算时间
func (t *ArbitrageTUI) getTimeUntilClose() string {
	mins := t.getMinutesLeft()
	secs := t.getSecondsLeft() % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

// getMinutesLeft 获取剩余分钟
func (t *ArbitrageTUI) getMinutesLeft() int {
	return int(t.getFractionalMinutesLeft())
}

// getFractionalMinutesLeft 获取剩余分钟 (浮点数，包含秒)
func (t *ArbitrageTUI) getFractionalMinutesLeft() float64 {
	now := time.Now()
	minute := now.Minute()
	second := now.Second()

	// 计算当前 15 分钟周期的结束时间
	// 0-14 -> 15
	// 15-29 -> 30
	// 30-44 -> 45
	// 45-59 -> 60 (0)
	nextSettlementMinute := (minute/15 + 1) * 15

	// 计算剩余总秒数
	// 如果是 45-59 分，nextSettlementMinute 是 60，相当于下一小时的 0 分
	// 剩余分钟 = nextSettlementMinute - minute - 1
	// 剩余秒数 = 60 - second

	minutesRemaining := nextSettlementMinute - minute - 1
	secondsRemaining := 60 - second

	return float64(minutesRemaining) + float64(secondsRemaining)/60.0
}

// getSecondsLeft 获取剩余秒数
func (t *ArbitrageTUI) getSecondsLeft() int {
	now := time.Now()
	second := now.Second()
	return 60 - second
}

// ==================== 独立运行入口 (演示用) ====================

// RunTUIDemo 独立运行 TUI 演示
func RunTUIDemo() {
	tui := NewArbitrageTUI(TUIConfig{
		RefreshRate: 1 * time.Second,
	})

	tui.Start()

	// 等待中断
	select {}
}
