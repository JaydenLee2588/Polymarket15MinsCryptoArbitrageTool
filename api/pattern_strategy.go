package api

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"Polymarket15MinsCryptoArbitrageTool/internal/database"
	"Polymarket15MinsCryptoArbitrageTool/src"
)

// ==================== Pattern 策略状态 ====================

// PatternPosition 持仓状态
type PatternPosition struct {
	ConfigID      string
	MarketSlug    string
	Symbol        string
	Side          string    // "UP" or "DOWN"
	TokenID       string    // 持仓 Token ID
	BuyPrice      float64   // 买入价格
	Amount        float64   // 持仓数量
	Cost          float64   // 成本 (BuyPrice * Amount)
	BuyTime       time.Time // 买入时间
	RecordID      int64     // 数据库记录 ID
	Pending       bool      // 是否正在下单中 (防止重复下单)
	Resolved      bool      // 是否已决议
	SellFailCount int       // 卖出失败次数 (用于重试限制)

	// 买入时的指标快照 (用于交易历史日志)
	BuyIndicators struct {
		VWAPDist      float64
		MACDLine      float64
		MACDSignal    float64
		MACDHistogram float64
		MACDHistDelta float64
		RSI           float64
		RSISlope      float64
	}

	// 市场开始时间 (计算开局后几分钟)
	MarketStartTime time.Time
}

// PatternMarketState 市场状态
type PatternMarketState struct {
	Slug          string
	Symbol        string
	StartTime     time.Time
	EndTime       time.Time
	TokenIDToSide map[string]string
	SideToTokenID map[string]string
	LatestPrices  map[string]float64
	StrikePrice   float64

	// 持仓
	Positions map[string]*PatternPosition // configID -> Position

	// Pattern 检测缓存 (避免重复检测)
	LastPatternCheck map[string]time.Time       // configID -> 上次检测时间
	LastPattern      map[string]src.PatternType // symbol -> 上次检测到的 Pattern

	// Sniper 冷却期追踪
	LastSellTime map[string]time.Time // "configID_side" -> 上次卖出时间
}

// BuyIndicatorSnapshot 买入时的指标快照
type BuyIndicatorSnapshot struct {
	VWAPDist      float64
	MACDLine      float64
	MACDSignal    float64
	MACDHistogram float64
	MACDHistDelta float64
	RSI           float64
	RSISlope      float64
}

// Pattern 策略全局状态
var (
	patternStrategyMu      sync.Mutex
	patternStrategyRunning bool
	patternStopChan        chan struct{}
	patternDoneChan        chan struct{}

	// 市场状态 (复用主策略的 MarketManager)
	patternMarketStates   map[string]*PatternMarketState // slug -> state
	patternMarketStatesMu sync.RWMutex
)

// ==================== 策略启动/停止 ====================

// StartPatternStrategy 启动 Pattern 策略
func StartPatternStrategy(configs []PatternStrategyConfig) bool {
	patternStrategyMu.Lock()
	defer patternStrategyMu.Unlock()

	if patternStrategyRunning {
		log.Println("⚠️ Pattern 策略已在运行")
		return false
	}

	if len(configs) == 0 {
		log.Println("⚠️ 没有 Pattern 策略配置")
		return false
	}

	patternStopChan = make(chan struct{})
	patternDoneChan = make(chan struct{})
	patternMarketStates = make(map[string]*PatternMarketState)
	patternStrategyRunning = true

	// ===== 恢复数据库中的持仓 =====
	RestorePatternPositions()

	// ===== 确保 Binance WebSocket 客户端已初始化 =====
	initPatternBinanceClient()

	// ===== 初始化交易历史日志 =====
	if err := InitTradeHistoryLogger(); err != nil {
		log.Printf("⚠️ [Pattern] 交易历史日志初始化失败: %v", err)
	}

	go runPatternStrategyLoop(configs)

	log.Printf("🎯 Pattern 策略已启动 - %d 个配置组", len(configs))
	for _, cfg := range configs {
		patternNames := make([]string, len(cfg.Patterns))
		for i, p := range cfg.Patterns {
			patternNames[i] = string(p)
		}
		log.Printf("   📝 [%s] Patterns: %v, Symbols: %v, SellThreshold: %.2f",
			cfg.ID, patternNames, cfg.Symbols, cfg.SellThreshold)
	}

	return true
}

// StopPatternStrategy 停止 Pattern 策略
func StopPatternStrategy() bool {
	patternStrategyMu.Lock()

	if !patternStrategyRunning {
		patternStrategyMu.Unlock()
		log.Println("⚠️ Pattern 策略未在运行")
		return false
	}

	patternStrategyRunning = false
	close(patternStopChan)
	doneChan := patternDoneChan
	patternStrategyMu.Unlock()

	// 等待退出
	select {
	case <-doneChan:
		log.Println("✅ Pattern 策略已停止")
		return true
	case <-time.After(10 * time.Second):
		log.Println("⚠️ Pattern 策略停止超时")
		return true
	}
}

// IsPatternStrategyRunning 检查 Pattern 策略是否运行
func IsPatternStrategyRunning() bool {
	patternStrategyMu.Lock()
	defer patternStrategyMu.Unlock()
	return patternStrategyRunning
}

// ==================== 策略主循环 ====================

// patternCheckTask 用于传递检测任务 (避免在锁内执行耗时操作)
type patternCheckTask struct {
	State  *PatternMarketState
	Config PatternStrategyConfig
	IsSell bool // true=检查卖出, false=检查买入
}

func runPatternStrategyLoop(configs []PatternStrategyConfig) {
	defer close(patternDoneChan)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("💓 [Pattern] 策略循环已启动")

	for {
		select {
		case <-patternStopChan:
			log.Println("🛑 [Pattern] 收到停止信号")
			return

		case <-ticker.C:
			// 同步市场状态
			//log.Println("[DEBUG] Pattern tick - 开始同步市场状态...")
			syncPatternMarketStates()
			//log.Println("[DEBUG] Pattern tick - 市场状态同步完成")

			// ===== 修复死锁: 先收集任务，释放锁后再执行 =====
			var tasks []patternCheckTask

			patternMarketStatesMu.RLock()
			for _, state := range patternMarketStates {
				for _, cfg := range configs {
					// 检查是否支持该币种 (使用前缀匹配)
					if !cfg.ContainsSymbolPrefix(state.Symbol) {
						continue
					}

					elapsed := time.Since(state.StartTime).Seconds()

					// 检查是否已有持仓 (包括 pending 状态)
					if state.Positions != nil {
						if pos, ok := state.Positions[cfg.ID]; ok && pos != nil {
							// 已有持仓，检查卖出条件 (卖出不受时间窗口限制)
							tasks = append(tasks, patternCheckTask{
								State:  state,
								Config: cfg,
								IsSell: true,
							})
							continue
						}
					}

					// 买入检测需要在时间窗口内
					if elapsed < float64(cfg.TrendWindowSecStart) || elapsed > float64(cfg.TrendWindowSecEnd) {
						continue
					}

					// 检测 Pattern (5秒内不重复检测同一配置)
					if state.LastPatternCheck != nil {
						if lastCheck, ok := state.LastPatternCheck[cfg.ID]; ok {
							if time.Since(lastCheck) < 5*time.Second {
								continue
							}
						}
					}

					tasks = append(tasks, patternCheckTask{
						State:  state,
						Config: cfg,
						IsSell: false,
					})
				}
			}
			patternMarketStatesMu.RUnlock()

			// ===== 检查过期持仓并移入决议队列 =====
			checkPatternPositionExpiry()

			// ===== 在锁外执行检测任务 =====
			for _, task := range tasks {
				if task.IsSell {
					// 卖出检测
					patternMarketStatesMu.RLock()
					pos := task.State.Positions[task.Config.ID]
					patternMarketStatesMu.RUnlock()
					if pos != nil && !pos.Pending {
						checkPatternSellCondition(task.State, pos, task.Config)
					}
				} else {
					// 买入检测
					checkPatternBuySignal(task.State, task.Config)
				}
			}
		}
	}
}

// ==================== 市场状态同步 ====================

// syncPatternMarketStates 从主策略同步市场状态
func syncPatternMarketStates() {
	// 获取主策略的市场状态
	//log.Println("[DEBUG] syncPatternMarketStates - 准备调用 GetActiveMarketStates...")
	mainStates := GetActiveMarketStates()
	//log.Println("[DEBUG] syncPatternMarketStates - GetActiveMarketStates 返回完成")
	if mainStates == nil {
		//log.Println("[DEBUG] syncPatternMarketStates - mainStates 为 nil，跳过同步")
		return
	}

	patternMarketStatesMu.Lock()
	defer patternMarketStatesMu.Unlock()

	// 遍历主策略的所有配置组
	for _, configStates := range mainStates {
		for slug, mainState := range configStates {
			if mainState == nil {
				continue
			}

			// 检查是否已存在
			if _, exists := patternMarketStates[slug]; !exists {
				// 创建新的 Pattern 市场状态
				patternMarketStates[slug] = &PatternMarketState{
					Slug:             mainState.Slug,
					Symbol:           mainState.Symbol,
					StartTime:        mainState.StartTime,
					EndTime:          mainState.EndTime,
					TokenIDToSide:    mainState.TokenIDToSide,
					SideToTokenID:    mainState.SideToTokenID,
					LatestPrices:     make(map[string]float64),
					StrikePrice:      mainState.StrikePrice,
					Positions:        make(map[string]*PatternPosition),
					LastPatternCheck: make(map[string]time.Time),
					LastSellTime:     make(map[string]time.Time), // Sniper 冷却期追踪
				}
			}

			// 同步价格 (需要持有 simMu 锁，因为 mainState.LatestPrices 可能被并发写入)
			state := patternMarketStates[slug]
			simMu.Lock()
			// 同步 StrikePrice (可能在初始化后才获取到)
			if mainState.StrikePrice > 0 {
				state.StrikePrice = mainState.StrikePrice
			}
			for side, price := range mainState.LatestPrices {
				state.LatestPrices[side] = price
			}
			simMu.Unlock()
		}
	}

	// 清理过期市场 (保护有未决议持仓的状态)
	// 先收集要删除的 slugs，避免在迭代中删除 (会导致 concurrent map iteration panic)
	now := time.Now()
	var expiredSlugs []string
	for slug, state := range patternMarketStates {
		// 只有在市场过期 AND 没有持仓时才删除
		if now.After(state.EndTime.Add(5*time.Minute)) && len(state.Positions) == 0 {
			expiredSlugs = append(expiredSlugs, slug)
		}
	}

	// 安全删除过期市场
	for _, slug := range expiredSlugs {
		delete(patternMarketStates, slug)
	}
}

// ==================== Pattern 检测与买入 ====================

func checkPatternBuySignal(state *PatternMarketState, cfg PatternStrategyConfig) {
	binanceSymbol := state.Symbol + "USDT"

	// ===== Sniper 模式分发 =====
	if cfg.SniperMode {
		checkSniperBuySignal(state, cfg, binanceSymbol)
		return
	}

	// ===== 复用主策略的 Binance WebSocket K线数据 =====
	if binanceClient == nil {
		return
	}

	klines := binanceClient.GetHistoricalKlines(binanceSymbol, 240)
	if len(klines) < 50 {
		return
	}

	closes := src.GetCloses(klines)
	currentPrice := closes[len(closes)-1]

	// 计算技术指标
	rsi14 := src.ComputeRSI(closes, 14)
	rsiSlope := src.SlopeLast(src.ComputeRSISeries(closes, 14), 3)
	macd := src.ComputeMACD(closes, 12, 26, 9)
	vwap := src.ComputeSessionVWAP(klines)
	vwapDist := (currentPrice - vwap) / vwap

	// 计算前一个 RSI
	prevRSI := 50.0
	if len(closes) > 15 {
		prevRSI = src.ComputeRSI(closes[:len(closes)-1], 14)
	}

	// ===== TrendSurge 检测 =====
	// 计算额外指标
	atr := src.ComputeATR(klines, 14)
	volumes := src.GetVolumes(klines)
	var volSlope float64
	if len(volumes) >= 5 {
		volSlope = src.LinearRegressionSlope(volumes[len(volumes)-5:])
	}

	// 计算波动率
	var sigma15m, sigma30s float64
	if len(closes) >= 15 {
		sigma15m = src.ComputeStdDev(closes[len(closes)-15:])
	}
	if len(closes) >= 5 {
		sigma30s = src.ComputeStdDev(closes[len(closes)-5:]) // 约 5 分钟 (5根 1m K线)
	}

	// Heiken Ashi
	haCandles := src.ComputeHeikenAshi(klines)
	var recentHA []src.HeikenAshiCandle
	if len(haCandles) >= 5 {
		recentHA = haCandles[len(haCandles)-5:]
	} else {
		recentHA = haCandles
	}

	// 计算剩余时间
	timeRemaining := int(time.Until(state.EndTime).Seconds())
	if timeRemaining < 0 {
		timeRemaining = 0
	}

	// 获取 15m 开盘价 (使用 15 分钟前的 K 线开盘价)
	openPrice := 0.0
	if len(klines) >= 15 {
		openPrice = klines[len(klines)-15].Open
	} else if len(klines) > 0 {
		openPrice = klines[0].Open // 数据不足时使用最早的 K 线
	}

	// 获取 Polymarket 价格 (用于 Minimum Edge 计算)
	polyPriceUp := state.LatestPrices["UP"]
	polyPriceDown := state.LatestPrices["DOWN"]

	// 构建 TrendSurge 输入
	trendSurgeInput := src.TrendSurgeInput{
		TimeRemaining: timeRemaining,
		CurrentPrice:  currentPrice,
		OpenPrice:     openPrice,
		StrikePrice:   state.StrikePrice,
		VWAP:          vwap,
		PolyPrice:     polyPriceUp, // 默认检测做多
		ATR:           atr,
		Sigma15m:      sigma15m,
		Sigma30s:      sigma30s,
		MACD:          macd,
		RSI:           rsi14,
		RSISlope:      rsiSlope,
		VolSlope:      volSlope,
		HeikenAshi:    recentHA,
		PriceVelocity: 0, // 暂不支持价格速度，需要历史价格缓存
	}

	// 分别检测做多和做空，选择评分最高的
	trendSurgeInputLong := trendSurgeInput
	trendSurgeInputLong.PolyPrice = polyPriceUp

	trendSurgeInputShort := trendSurgeInput
	trendSurgeInputShort.PolyPrice = polyPriceDown

	// 检测做多
	trendSurgeResultLong := src.DetectTrendSurge(trendSurgeInputLong)

	// 检测做空
	trendSurgeResultShort := src.DetectTrendSurge(trendSurgeInputShort)

	// 选择评分最高的结果
	var trendSurgeResult src.TrendSurgeResult
	if trendSurgeResultLong.Score > trendSurgeResultShort.Score {
		trendSurgeResult = trendSurgeResultLong
	} else {
		trendSurgeResult = trendSurgeResultShort
	}

	// 调试日志: 记录被否决的信号 (仅在某些条件下，或完全注释掉以减少刷屏)
	// if trendSurgeResult.VetoReason != "" {
	// 	log.Printf("🚫 [TrendSurge][%s] 被否决: %s | Phase=%d, Score=%d, Side=%s | 市场=%s",
	// 		cfg.ID, trendSurgeResult.VetoReason, trendSurgeResult.Phase, trendSurgeResult.Score, trendSurgeResult.Side, state.Slug)
	// }

	// 如果 TrendSurge 触发，检查是否在配置的 Pattern 列表中
	if trendSurgeResult.IsTriggered {
		var tsPatternType src.PatternType
		if trendSurgeResult.Side == "UP" {
			tsPatternType = src.PatternTrendSurgeUp
		} else {
			tsPatternType = src.PatternTrendSurgeDown
		}

		if cfg.ContainsPattern(tsPatternType) {
			log.Printf("📈 [TrendSurge][%s] 触发! Phase=%d, Score=%d, Side=%s, Z=%.2f | 市场=%s",
				cfg.ID, trendSurgeResult.Phase, trendSurgeResult.Score, trendSurgeResult.Side, trendSurgeResult.ZScore, state.Slug)

			// 获取价格和执行买入
			side := trendSurgeResult.Side
			price, ok := state.LatestPrices[side]
			if ok && price > 0 {
				amount := cfg.GetShareAmount(price)
				tokenID, tokenOK := state.SideToTokenID[side]

				if amount > 0 && tokenOK && tokenID != "" {
					// 防止重复下单
					patternMarketStatesMu.Lock()
					if state.Positions == nil {
						state.Positions = make(map[string]*PatternPosition)
					}
					if existing, exists := state.Positions[cfg.ID]; exists && existing != nil {
						patternMarketStatesMu.Unlock()
						// 已有持仓，跳过
					} else {
						state.Positions[cfg.ID] = &PatternPosition{ConfigID: cfg.ID, Pending: true}
						patternMarketStatesMu.Unlock()

						patternName := "TrendSurge-" + trendSurgeResult.Side
						buyIndicators := BuyIndicatorSnapshot{
							VWAPDist:      vwapDist,
							RSI:           rsi14,
							RSISlope:      rsiSlope,
							MACDLine:      macd.MACD,
							MACDSignal:    macd.Signal,
							MACDHistogram: macd.Histogram,
							MACDHistDelta: macd.HistDelta,
						}
						executePatternBuy(state, cfg, side, tokenID, price, amount, patternName, buyIndicators)
					}
				} else {
					// 明确记录不下单的原因
					if amount <= 0 {
						log.Printf("⚠️ [TrendSurge][%s] 触发但因【价格超限】跳过: Price=%.4f > MaxLimit (Amount=0)", cfg.ID, price)
					} else if !tokenOK || tokenID == "" {
						log.Printf("⚠️ [TrendSurge][%s] 触发但因【Token无效】跳过: TokenID=%s", cfg.ID, tokenID)
					}
				}
			}
			return // TrendSurge 触发后不再检测传统 Pattern
		}
	}

	// ===== 传统 Pattern 检测 =====
	// 调用 Pattern 检测
	patternInput := src.PatternDetectInput{
		VWAPDist: vwapDist,
		MACD:     macd,
		RSI:      rsi14,
		RSISlope: rsiSlope,
		PrevRSI:  prevRSI,
	}
	result := src.DetectPattern(patternInput)

	// ===== Pattern 变化日志 =====
	patternMarketStatesMu.Lock()
	if state.LastPatternCheck == nil {
		state.LastPatternCheck = make(map[string]time.Time)
	}
	if state.LastPattern == nil {
		state.LastPattern = make(map[string]src.PatternType)
	}
	state.LastPatternCheck[cfg.ID] = time.Now()

	// 检测 Pattern 是否变化 (使用 Slug 作为 key)
	patternKey := state.Slug
	prevPattern, hasPrev := state.LastPattern[patternKey]

	// 日志去重: 只有当 Pattern 发生变化时才记录
	// 并且为了防止多策略组同时打印，我们可以检查是否是第一个检测到的策略组
	// 但最简单的是：只要 Pattern 变了就打
	shouldLog := !hasPrev || prevPattern != result.PatternType

	if shouldLog {
		confidenceDesc := getConfidenceDescription(result.Confidence)
		if hasPrev && prevPattern != result.PatternType {
			prevName := src.GetPatternChineseName(prevPattern)
			log.Printf("🔄 [Pattern] %s 形态变化: %s → %s (置信度=%.2f %s)",
				state.Symbol, prevName, result.PatternName, result.Confidence, confidenceDesc)
		} else if !hasPrev && result.PatternType != src.PatternNeutral {
			log.Printf("📊 [Pattern] %s 首次检测: %s (置信度=%.2f %s)",
				state.Symbol, result.PatternName, result.Confidence, confidenceDesc)
		}
		// 更新状态
		state.LastPattern[patternKey] = result.PatternType
	}
	patternMarketStatesMu.Unlock()

	// 检查是否匹配配置的 Pattern
	if !cfg.ContainsPattern(result.PatternType) {
		return
	}

	// 确定买入方向
	var side string
	if result.IsBuyUp() {
		side = "UP"
	} else if result.IsBuyDown() {
		side = "DOWN"
	} else {
		return // 不是买入信号
	}

	// 获取价格
	price, ok := state.LatestPrices[side]
	if !ok || price <= 0 {
		return
	}

	// 检查价格范围
	amount := cfg.GetShareAmount(price)
	if amount <= 0 {
		return
	}

	// 获取 TokenID
	tokenID, ok := state.SideToTokenID[side]
	if !ok || tokenID == "" {
		return
	}

	// ===== 防止重复下单: 先标记为 pending =====
	patternMarketStatesMu.Lock()
	if state.Positions == nil {
		state.Positions = make(map[string]*PatternPosition)
	}
	if existing, ok := state.Positions[cfg.ID]; ok && existing != nil {
		patternMarketStatesMu.Unlock()
		return // 已有持仓或正在下单
	}
	// 设置 pending 占位
	state.Positions[cfg.ID] = &PatternPosition{
		ConfigID: cfg.ID,
		Pending:  true,
	}
	patternMarketStatesMu.Unlock()

	log.Printf("🎯 [Pattern][%s] 检测到形态: %s, 市场=%s, 方向=%s, 价格=%.4f",
		cfg.ID, result.PatternName, state.Slug, side, price)

	// 构建指标快照用于日志
	buyIndicators := BuyIndicatorSnapshot{
		VWAPDist: vwapDist,
		RSI:      rsi14,
		RSISlope: rsiSlope,
	}
	if macd != nil {
		buyIndicators.MACDLine = macd.MACD
		buyIndicators.MACDSignal = macd.Signal
		buyIndicators.MACDHistogram = macd.Histogram
		buyIndicators.MACDHistDelta = macd.HistDelta
	}

	// 执行买入
	executePatternBuy(state, cfg, side, tokenID, price, amount, result.PatternName, buyIndicators)
}

// executePatternBuy 执行 Pattern 策略买入
func executePatternBuy(state *PatternMarketState, cfg PatternStrategyConfig, side, tokenID string, price, amount float64, patternName string, indicators BuyIndicatorSnapshot) {
	now := time.Now()
	cost := price * amount
	filledPrice := price
	filledAmount := amount
	realOrderID := ""

	// 准备日志条目 (无论成功失败都要记录)
	logEntry := &TradeLogEntry{
		BuyTime:         now,
		ConfigID:        cfg.ID,
		MarketSlug:      state.Slug,
		MarketStartTime: state.StartTime,
		Side:            side,
		Amount:          amount,
		BuyPrice:        price,
		TotalCost:       cost,
		VWAPDist:        indicators.VWAPDist,
		MACDLine:        indicators.MACDLine,
		MACDSignal:      indicators.MACDSignal,
		MACDHistogram:   indicators.MACDHistogram,
		MACDHistDelta:   indicators.MACDHistDelta,
		RSI:             indicators.RSI,
		RSISlope:        indicators.RSISlope,
	}

	// ========== 模拟交易 - 增加现实性验证 ==========
	if cfg.TradingMode != "real" {
		// 1. 模拟执行延迟 (100-300ms)
		simDelay := time.Duration(100+rand.Intn(200)) * time.Millisecond
		time.Sleep(simDelay)

		// 2. 检查订单簿深度 (需要 ClobOrderClient)
		clobClientMu.Lock()
		client := clobOrderClient
		clobClientMu.Unlock()

		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			depthResult, depthErr := client.GetOrderbookDepth(ctx, tokenID, price, "BUY")
			cancel()

			if depthErr != nil {
				log.Printf("⚠️ [Pattern][%s][模拟] 订单簿查询失败: %v", cfg.ID, depthErr)
				// 订单簿查询失败不阻断模拟，只记录警告
			} else if depthResult.AvailableSize < amount {
				log.Printf("❌ [Pattern][%s][模拟] 订单簿深度不足 | 需要=%.2f | 可用=%.2f | 模拟失败",
					cfg.ID, amount, depthResult.AvailableSize)
				// 清除 pending 状态
				patternMarketStatesMu.Lock()
				delete(state.Positions, cfg.ID)
				patternMarketStatesMu.Unlock()
				LogBuyFailure(logEntry, fmt.Sprintf("模拟失败: 订单簿深度不足 (需要=%.2f, 可用=%.2f)", amount, depthResult.AvailableSize))
				return
			} else {
				log.Printf("✅ [Pattern][%s][模拟] 订单簿验证通过 | 需要=%.2f | 可用=%.2f",
					cfg.ID, amount, depthResult.AvailableSize)
			}
		}
	}

	// ========== 真实交易模式 ==========
	if cfg.TradingMode == "real" {
		clobClientMu.Lock()
		client := clobOrderClient
		clobClientMu.Unlock()

		if client == nil {
			log.Printf("❌ [Pattern][%s] 真实交易模式但 ClobOrderClient 未初始化", cfg.ID)
			// 清除 pending 状态
			patternMarketStatesMu.Lock()
			delete(state.Positions, cfg.ID)
			patternMarketStatesMu.Unlock()
			LogBuyFailure(logEntry, "ClobOrderClient 未初始化")
			return
		}

		// 确保 Cost 有效
		if cost <= 0 {
			cost = price * amount
			if cost <= 0 {
				log.Printf("❌ [Pattern][%s] 成本计算错误: cost=%.4f (price=%.4f, amount=%.4f)",
					cfg.ID, cost, price, amount)
				// 清除 pending 状态
				patternMarketStatesMu.Lock()
				delete(state.Positions, cfg.ID)
				patternMarketStatesMu.Unlock()
				LogBuyFailure(logEntry, "成本计算错误")
				return
			}
		}

		// 下单
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		finalPrice := price + cfg.MaxSlippage

		var orderResult *src.ClobOrderResult
		var orderErr error

		if cfg.OrderType == "GTC" {
			orderResult, orderErr = client.PlaceLimitBuyOrder(ctx, tokenID, finalPrice, amount)
		} else if cfg.OrderType == "FAK" {
			orderResult, orderErr = client.PlaceMarketBuyOrderFAK(ctx, tokenID, finalPrice, amount, 30)
		} else {
			orderResult, orderErr = client.PlaceMarketBuyOrder(ctx, tokenID, finalPrice, amount, 30)
		}
		cancel()

		if orderErr != nil {
			log.Printf("❌ [Pattern][%s] 下单失败: %v", cfg.ID, orderErr)
			// 清除 pending 状态
			patternMarketStatesMu.Lock()
			delete(state.Positions, cfg.ID)
			patternMarketStatesMu.Unlock()
			LogBuyFailure(logEntry, fmt.Sprintf("下单失败: %v", orderErr))
			return
		}

		if !orderResult.Success {
			log.Printf("❌ [Pattern][%s] 下单被拒绝: %s", cfg.ID, orderResult.ErrorMsg)
			// 清除 pending 状态
			patternMarketStatesMu.Lock()
			delete(state.Positions, cfg.ID)
			patternMarketStatesMu.Unlock()
			LogBuyFailure(logEntry, fmt.Sprintf("下单被拒绝: %s", orderResult.ErrorMsg))
			return
		}

		realOrderID = orderResult.OrderID
		log.Printf("✅ [Pattern][%s] 真实下单成功: OrderID=%s", cfg.ID, realOrderID)
	}

	// 记录日志
	modeTag := "模拟"
	if cfg.TradingMode == "real" {
		modeTag = "💰真实"
	}

	log.Printf("✅ [Pattern][%s][%s] 买入成功 | 市场=%s | 方向=%s | Pattern=%s | 价格=%.4f | 数量=%.2f | 成本=$%.2f",
		modeTag, cfg.ID, state.Slug, side, patternName, filledPrice, filledAmount, cost)

	// 保存到数据库 (包含 TokenID 和 Symbol 用于恢复)
	trade := &database.SimulationTrade{
		ConfigGroup: "Pattern-" + cfg.ID,
		MarketSlug:  state.Slug,
		Stage:       "Leg1",
		Side:        side,
		Price:       filledPrice,
		Amount:      filledAmount,
		Cost:        cost,
		TokenID:     tokenID,
		Symbol:      state.Symbol,
	}

	id, err := database.InsertTrade(trade)
	if err != nil {
		log.Printf("❌ [Pattern] 保存交易记录失败: %v", err)
	}

	// 更新持仓 (替换 pending 占位)
	patternMarketStatesMu.Lock()
	pos := &PatternPosition{
		ConfigID:        cfg.ID,
		MarketSlug:      state.Slug,
		Symbol:          state.Symbol,
		Side:            side,
		TokenID:         tokenID,
		BuyPrice:        filledPrice,
		Amount:          filledAmount,
		Cost:            cost, // 用于 PnL 计算
		BuyTime:         now,
		RecordID:        id,
		Pending:         false,
		MarketStartTime: state.StartTime,
	}
	// 保存指标快照
	pos.BuyIndicators.VWAPDist = indicators.VWAPDist
	pos.BuyIndicators.MACDLine = indicators.MACDLine
	pos.BuyIndicators.MACDSignal = indicators.MACDSignal
	pos.BuyIndicators.MACDHistogram = indicators.MACDHistogram
	pos.BuyIndicators.MACDHistDelta = indicators.MACDHistDelta
	pos.BuyIndicators.RSI = indicators.RSI
	pos.BuyIndicators.RSISlope = indicators.RSISlope

	state.Positions[cfg.ID] = pos
	patternMarketStatesMu.Unlock()

	// 记录交易历史日志 (买入成功)
	LogBuyEntry(logEntry)

	// ===== 真实交易模式下，异步同步实际余额 =====
	if cfg.TradingMode == "real" {
		go syncPositionBalance(state.Slug, cfg.ID, tokenID)
	}
}

// ==================== 卖出检测 ====================

func checkPatternSellCondition(state *PatternMarketState, pos *PatternPosition, cfg PatternStrategyConfig) {
	// 跳过已失败多次的持仓 (等待决议)
	if pos.SellFailCount >= 5 {
		return
	}

	// 获取当前价格
	currentPrice, ok := state.LatestPrices[pos.Side]
	if !ok || currentPrice <= 0 {
		return
	}

	// 1. 检查时间止损 (优先于盈利检查，防止最后归零)
	// 注意: 如果 EndTime 为零值 (恢复的持仓可能没有 EndTime)，跳过时间止损
	if cfg.EnableStopLoss && !state.EndTime.IsZero() {
		timeLeft := time.Until(state.EndTime)
		stopLossDuration := time.Duration(cfg.StopLossMinutesBeforeClose) * time.Minute
		if stopLossDuration <= 0 {
			stopLossDuration = 3 * time.Minute // 默认 3 分钟
		}

		if timeLeft < stopLossDuration && timeLeft > -1*time.Hour {
			// timeLeft > -1*time.Hour 确保不会对已过期很久的市场触发
			log.Printf("⏰ [Pattern][%s] 时间止损触发! 剩余 %.1f 分钟 (阈值 %d 分) | 买入=%.4f | 当前=%.4f",
				cfg.ID, timeLeft.Minutes(), cfg.StopLossMinutesBeforeClose, pos.BuyPrice, currentPrice)
			executePatternSellWithReason(state, pos, cfg, currentPrice, "时间止损")
			return
		}
	}

	// 2. Sniper 模式: Touch-and-Go 止盈逻辑
	if cfg.SniperMode && state.StrikePrice > 0 {
		binanceSymbol := state.Symbol + "USDT"
		if binanceClient != nil {
			klines := binanceClient.GetHistoricalKlines(binanceSymbol, 60)
			if len(klines) >= 10 {
				closes := src.GetCloses(klines)
				binancePrice := closes[len(closes)-1]
				sigma := src.ComputeRealtimeSigma(klines, 10)

				// 动态保底: 避免 Sigma 过小导致 Z-Score 爆炸，但也避免使用硬编码的 50 (原逻辑错误)
				minSigma := binancePrice * 0.001 // 0.1% 的价格作为最小波动率
				if sigma < minSigma {
					sigma = minSigma
				}

				// 计算 Z-Score
				zScore := src.CalculateZScore(binancePrice, state.StrikePrice, sigma)

				// 计算价格收益 (使用 sell_threshold 作为止盈阈值)
				priceGain := currentPrice - pos.BuyPrice

				// Touch-and-Go 条件:
				// 1. Z-Score 回归 (|Z| < zscore_exit_threshold)
				// 2. 价格收益达到阈值 (使用 sell_threshold)
				shouldExit := false
				reason := ""

				if math.Abs(zScore) < cfg.SniperConfig.ZScoreExitThreshold {
					shouldExit = true
					reason = "zscore_reverted"
				} else if priceGain >= cfg.SellThreshold {
					shouldExit = true
					reason = "profit_target_reached"
				}

				if shouldExit {
					log.Printf("🎯 [Sniper][%s] Touch-and-Go 止盈! Z=%.2f | 收益=%.4f (阈值=%.4f) | 原因=%s | 市场=%s",
						cfg.ID, zScore, priceGain, cfg.SellThreshold, reason, state.Slug)
					executePatternSellWithReason(state, pos, cfg, currentPrice, "Sniper:"+reason)
					return
				}
			}
		}
	}

	// 3. 传统 Pattern 模式: 检查是否达到卖出阈值 (止盈)
	priceGain := currentPrice - pos.BuyPrice
	if priceGain < cfg.SellThreshold {
		return
	}

	log.Printf("💰 [Pattern][%s] 达到卖出阈值! 市场=%s | 买入=%.4f | 当前=%.4f | 收益=%.4f",
		cfg.ID, state.Slug, pos.BuyPrice, currentPrice, priceGain)

	// 执行卖出
	executePatternSell(state, pos, cfg, currentPrice)
}

// executePatternSell 执行 Pattern 策略卖出
func executePatternSell(state *PatternMarketState, pos *PatternPosition, cfg PatternStrategyConfig, sellPrice float64) {
	executePatternSellWithReason(state, pos, cfg, sellPrice, "止盈")
}

// executePatternSellWithReason 执行卖出并指定原因
func executePatternSellWithReason(state *PatternMarketState, pos *PatternPosition, cfg PatternStrategyConfig, sellPrice float64, sellReason string) {
	originalAmount := pos.Amount
	filledAmount := pos.Amount // 默认假设全部成交 (模拟模式)
	actualSellPrice := sellPrice

	// ========== 模拟交易 - 增加现实性验证 ==========
	if cfg.TradingMode != "real" {
		// 1. 模拟执行延迟 (100-300ms)
		simDelay := time.Duration(100+rand.Intn(200)) * time.Millisecond
		time.Sleep(simDelay)

		// 2. 检查订单簿深度 (需要 ClobOrderClient)
		clobClientMu.Lock()
		client := clobOrderClient
		clobClientMu.Unlock()

		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			depthResult, depthErr := client.GetOrderbookDepth(ctx, pos.TokenID, sellPrice, "SELL")
			cancel()

			if depthErr != nil {
				log.Printf("⚠️ [Pattern][%s][模拟卖出] 订单簿查询失败: %v", cfg.ID, depthErr)
				// 订单簿查询失败不阻断模拟，只记录警告
			} else if depthResult.AvailableSize < pos.Amount {
				log.Printf("❌ [Pattern][%s][模拟卖出] 买盘深度不足 | 需要=%.2f | 可用=%.2f | 模拟失败",
					cfg.ID, pos.Amount, depthResult.AvailableSize)
				pos.SellFailCount++
				// 记录模拟卖出失败到交易历史
				UpdateSellEntry(state.Slug, cfg.ID,
					fmt.Sprintf("模拟失败:买盘深度不足(需要=%.2f,可用=%.2f)", pos.Amount, depthResult.AvailableSize),
					sellPrice, 0, 0, 0)
				return
			} else {
				log.Printf("✅ [Pattern][%s][模拟卖出] 订单簿验证通过 | 需要=%.2f | 可用=%.2f",
					cfg.ID, pos.Amount, depthResult.AvailableSize)
			}
		}
	}

	// ========== 真实交易模式 ==========
	if cfg.TradingMode == "real" {
		clobClientMu.Lock()
		client := clobOrderClient
		clobClientMu.Unlock()

		if client == nil {
			log.Printf("❌ [Pattern][%s] 真实交易模式但 ClobOrderClient 未初始化", cfg.ID)
			return
		}

		// 卖出 (以略低于当前价格挂单)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		finalPrice := sellPrice - cfg.MaxSlippage
		if finalPrice < 0.01 {
			finalPrice = 0.01
		}

		var orderResult *src.ClobOrderResult
		var orderErr error

		// 使用 FAK 卖单 (立即成交能成交的部分，剩余取消)
		orderResult, orderErr = client.PlaceMarketSellOrderFAK(ctx, pos.TokenID, finalPrice, pos.Amount, 30)
		cancel()

		if orderErr != nil {
			pos.SellFailCount++
			if pos.SellFailCount >= 5 {
				log.Printf("❌ [Pattern][%s] 卖出失败超过5次，停止重试: %v", cfg.ID, orderErr)
			} else {
				log.Printf("⚠️ [Pattern][%s] 卖出失败 (%d/5): %v", cfg.ID, pos.SellFailCount, orderErr)
			}
			return
		}

		if !orderResult.Success {
			// 检查是否是"余额不足"错误 - 这意味着持仓已经被卖出(可能上次成功但没返回正确的FilledSize)
			if strings.Contains(orderResult.ErrorMsg, "not enough balance") ||
				strings.Contains(orderResult.ErrorMsg, "allowance") {
				log.Printf("⚠️ [Pattern][%s] 余额不足错误，持仓可能已被卖出，清除持仓状态", cfg.ID)

				// 清除持仓 (假设已经卖出)
				patternMarketStatesMu.Lock()
				delete(state.Positions, cfg.ID)
				patternMarketStatesMu.Unlock()

				// 更新数据库为 SOLD (PnL 未知，设为 0)
				if pos.RecordID > 0 {
					if err := database.UpdateTradeResolution(pos.RecordID, "SOLD_BALANCE_ERROR", 0); err != nil {
						log.Printf("❌ [Pattern] 更新交易记录失败: %v", err)
					}
				}
				return
			}

			pos.SellFailCount++
			if pos.SellFailCount >= 5 {
				log.Printf("❌ [Pattern][%s] 卖出被拒绝超过5次，停止重试: %s", cfg.ID, orderResult.ErrorMsg)
			} else {
				log.Printf("⚠️ [Pattern][%s] 卖出被拒绝 (%d/5): %s", cfg.ID, pos.SellFailCount, orderResult.ErrorMsg)
			}
			return
		}

		// 检查实际成交数量
		filledAmount = orderResult.FilledSize
		if orderResult.FilledPrice > 0 {
			actualSellPrice = orderResult.FilledPrice
		}

		// 检查是否部分成交
		if filledAmount <= 0 {
			// FilledSize 为 0 但订单成功 - 可能是 API 没有返回正确的成交数量
			// 假设全部成交，继续处理
			log.Printf("⚠️ [Pattern][%s] 卖单成功但成交数量为0，假设全部成交", cfg.ID)
			filledAmount = originalAmount
		}

		if filledAmount < originalAmount {
			// 部分成交：更新剩余持仓
			remaining := originalAmount - filledAmount
			log.Printf("⚠️ [Pattern][%s] 部分成交! 卖出 %.2f/%.2f 股，剩余 %.2f 股",
				cfg.ID, filledAmount, originalAmount, remaining)

			// 更新持仓数量为剩余数量
			patternMarketStatesMu.Lock()
			pos.Amount = remaining
			pos.Cost = pos.BuyPrice * remaining // 更新剩余成本
			pos.SellFailCount = 0               // 重置失败计数，继续尝试卖出剩余
			patternMarketStatesMu.Unlock()

			// 计算已成交部分的收益
			partialProfit := (actualSellPrice - pos.BuyPrice) * filledAmount
			log.Printf("✅ [Pattern][💰真实][%s] 部分卖出 | 市场=%s | 成交=%.2f股 | 价格=%.4f | 收益=$%.2f",
				cfg.ID, state.Slug, filledAmount, actualSellPrice, partialProfit)

			// 不删除持仓，不更新数据库为SOLD（因为还有剩余）
			return
		}

		log.Printf("✅ [Pattern][%s] 真实卖出成功: OrderID=%s | 成交=%.2f股", cfg.ID, orderResult.OrderID, filledAmount)
	}

	// 计算收益 (使用实际成交数量和价格)
	profit := (actualSellPrice - pos.BuyPrice) * filledAmount
	revenue := actualSellPrice * filledAmount

	// 记录日志
	modeTag := "模拟"
	if cfg.TradingMode == "real" {
		modeTag = "💰真实"
	}

	log.Printf("✅ [Pattern][%s][%s] 卖出成功 | 市场=%s | 买入=%.4f | 卖出=%.4f | 数量=%.2f | 收益=$%.2f",
		modeTag, cfg.ID, state.Slug, pos.BuyPrice, actualSellPrice, filledAmount, profit)

	// 更新数据库
	if pos.RecordID > 0 {
		resolution := "SOLD"
		if err := database.UpdateTradeResolution(pos.RecordID, resolution, profit); err != nil {
			log.Printf("❌ [Pattern] 更新交易记录失败: %v", err)
		}
	}

	// 记录交易历史日志 (卖出)
	UpdateSellEntry(state.Slug, cfg.ID, sellReason, actualSellPrice, filledAmount, revenue, profit)

	// 清除持仓并更新冷却期时间戳
	patternMarketStatesMu.Lock()
	// 记录卖出时间 (用于冷却期检查)
	cooldownKey := cfg.ID + "_" + pos.Side
	if state.LastSellTime == nil {
		state.LastSellTime = make(map[string]time.Time)
	}
	state.LastSellTime[cooldownKey] = time.Now()

	delete(state.Positions, cfg.ID)
	patternMarketStatesMu.Unlock()
}

// ==================== 余额同步 ====================

// syncPositionBalance 从 Polymarket API 同步实际持仓数量
// 在买入成功后调用，确保记录的数量与实际持仓一致
func syncPositionBalance(marketSlug, configID, tokenID string) {
	// 等待一小段时间让订单完全确认
	time.Sleep(2 * time.Second)

	// 获取钱包地址
	walletAddr := os.Getenv("PROXY_WALLET_ADDRESS")
	if walletAddr == "" {
		walletAddr = os.Getenv("MAKER_ADDRESS")
	}
	if walletAddr == "" {
		log.Printf("⚠️ [Pattern] 余额同步跳过: 未配置钱包地址")
		return
	}

	// 创建 Data API 客户端
	dataClient := src.NewDataClient(http.DefaultClient)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	positions, err := dataClient.GetPositions(ctx, &src.GetPositionsParams{
		User:  walletAddr,
		Limit: 100,
	})
	if err != nil {
		log.Printf("⚠️ [Pattern][%s] 余额同步失败: %v", configID, err)
		return
	}

	// 查找匹配的持仓
	for _, pos := range positions {
		if pos.Asset == tokenID {
			actualAmount := pos.Size.InexactFloat64()

			// 更新本地记录
			patternMarketStatesMu.Lock()
			state, exists := patternMarketStates[marketSlug]
			if exists && state.Positions != nil {
				if position, ok := state.Positions[configID]; ok {
					if actualAmount != position.Amount {
						log.Printf("📊 [Pattern][%s] 持仓数量校正: 记录=%.2f → 实际=%.2f",
							configID, position.Amount, actualAmount)
						position.Amount = actualAmount
						position.Cost = position.BuyPrice * actualAmount
					} else {
						log.Printf("✅ [Pattern][%s] 持仓数量确认: %.2f 股", configID, actualAmount)
					}
				}
			}
			patternMarketStatesMu.Unlock()
			return
		}
	}

	log.Printf("⚠️ [Pattern][%s] 余额同步: 未找到 TokenID=%s 的持仓", configID, tokenID[:min(12, len(tokenID))])
}

// ==================== 辅助函数 ====================

// parseEndTimeFromSlug 从 slug 解析结束时间
// 格式: btc-updown-15m-1770194700 (最后一部分是市场开始的 Unix 时间戳)
// 注意: slug 中的时间戳是开始时间，结束时间需要加 15 分钟
func parseEndTimeFromSlug(slug string) time.Time {
	parts := strings.Split(slug, "-")
	if len(parts) >= 4 {
		// 最后一部分应该是时间戳 (市场开始时间)
		timestamp, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if err == nil && timestamp > 1000000000 { // 基本验证
			// 市场结束时间 = 开始时间 + 15 分钟
			return time.Unix(timestamp, 0).Add(15 * time.Minute)
		}
	}
	return time.Time{}
}

// GetPatternMarketInfo 获取 Pattern 策略的市场信息 (供 TUI 使用)
func GetPatternMarketInfo(symbol string) *PatternMarketState {
	patternMarketStatesMu.RLock()
	defer patternMarketStatesMu.RUnlock()

	for _, state := range patternMarketStates {
		if state.Symbol == symbol || strings.Contains(strings.ToLower(state.Slug), strings.ToLower(symbol)) {
			return state
		}
	}
	return nil
}

// initPatternBinanceClient 初始化 Binance WebSocket 客户端 (如果尚未初始化)
func initPatternBinanceClient() {
	simMu.Lock()
	defer simMu.Unlock()

	// 如果已经初始化，直接返回
	if binanceClient != nil {
		log.Println("ℹ️ [Pattern] Binance WebSocket 客户端已存在，复用")
		return
	}

	// 初始化 Binance WebSocket 客户端
	binanceSymbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT"}
	binanceClient = src.NewBinanceKlineClient(binanceSymbols, src.NewLogger(src.LogLevelInfo))

	if err := binanceClient.Connect(); err != nil {
		log.Printf("⚠️ [Pattern] Binance WS 连接失败: %v", err)
		binanceClient = nil
		return
	}

	log.Println("✅ [Pattern] Binance WebSocket 客户端已初始化 (15m + 1m K线)")
}

// getConfidenceDescription 获取置信度的中文描述
func getConfidenceDescription(confidence float64) string {
	if confidence >= 0.90 {
		return "非常确定"
	}
	if confidence >= 0.70 {
		return "较为确定"
	}
	if confidence >= 0.50 {
		return "一般确定"
	}
	return "不太确定"
}

// checkPatternPositionExpiry 检查 Pattern 持仓过期并处理决议
func checkPatternPositionExpiry() {
	patternMarketStatesMu.Lock() // 修复: 添加缺失的锁
	now := time.Now()

	// ===== 修复死锁: 先收集任务，释放锁后再执行 =====
	type resolutionTask struct {
		State      *PatternMarketState
		Position   *PatternPosition
		ConfigID   string
		MarketSlug string
	}

	var tasks []resolutionTask

	for slug, state := range patternMarketStates {
		if len(state.Positions) == 0 {
			continue
		}

		// 检查市场是否过期 (结束时间 + 1分钟缓冲)
		if now.Before(state.EndTime.Add(1 * time.Minute)) {
			continue
		}

		// 市场已过期，收集未决议的持仓
		for configID, pos := range state.Positions {
			if pos == nil || pos.Pending || pos.Resolved {
				continue
			}

			// 标记为等待决议 (避免重复处理)
			pos.Resolved = true

			tasks = append(tasks, resolutionTask{
				State:      state,
				Position:   pos,
				ConfigID:   configID,
				MarketSlug: slug,
			})
		}
	}
	patternMarketStatesMu.Unlock()

	// ===== 在锁外执行决议查询 =====
	for _, task := range tasks {
		go func(t resolutionTask) {
			// 1. 获取市场信息
			client := src.NewGammaClient(http.DefaultClient)
			market, err := client.GetMarketBySlug(context.Background(), t.MarketSlug, nil)

			// 2. 处理查询失败 -> 重试机制
			if err != nil {
				log.Printf("⚠️ [Pattern] 查询决议失败 %s: %v (将重试)", t.MarketSlug, err)
				// 重置 Resolved 以便重试
				patternMarketStatesMu.Lock()
				t.Position.Resolved = false
				patternMarketStatesMu.Unlock()
				return
			}

			// 3. 检查市场是否决议
			if !market.Closed {
				// 市场未关闭，稍后再查
				patternMarketStatesMu.Lock()
				t.Position.Resolved = false
				patternMarketStatesMu.Unlock()
				return
			}

			// 4. 解析决议结果
			prices := market.OutcomePrices
			resolution := ""
			for i, pStr := range prices {
				price, _ := parsePrice(pStr)
				if price > 0.99 {
					if i == 0 {
						resolution = "UP"
					} else {
						resolution = "DOWN"
					}
					break
				}
			}

			if resolution == "" {
				log.Printf("⚠️ [Pattern][%s] 决议未确定 (Prices: %v)", t.MarketSlug, prices)
				// 重置以便重试 (可能只是数据暂未更新)
				patternMarketStatesMu.Lock()
				t.Position.Resolved = false
				patternMarketStatesMu.Unlock()
				return
			}

			// 5. 计算 PnL
			var pnl float64
			if resolution == t.Position.Side {
				// 获胜: amount - cost (纯利)
				pnl = t.Position.Amount - t.Position.Cost
			} else {
				// 失败: 损失全部 cost
				pnl = -t.Position.Cost
			}

			log.Printf("✅ [Pattern][%s][%s] 决议: %s, 持仓: %s, PnL: $%.2f",
				t.ConfigID, t.MarketSlug, resolution, t.Position.Side, pnl)

			// 6. 更新数据库
			if t.Position.RecordID > 0 {
				if err := database.UpdateTradeResolution(t.Position.RecordID, resolution, pnl); err != nil {
					log.Printf("❌ [Pattern] 更新决议失败: %v", err)
				}
			}

			// 7. 记录交易历史日志 (决议赎回)
			UpdateResolutionForRedemption(t.MarketSlug, t.ConfigID, resolution, pnl)

			// 8. 计划自动赎回 (仅获胜方)
			redeemerMu.Lock()
			red := redeemer
			redeemerMu.Unlock()

			if red != nil && resolution == t.Position.Side {
				// 获取 conditionId
				conditionID := market.ConditionID

				// 查找获胜 TokenID
				winningTokenID := ""
				for tid, side := range t.State.TokenIDToSide {
					if side == resolution {
						winningTokenID = tid
						break
					}
				}

				if conditionID != "" && winningTokenID != "" {
					// 获取用户地址和代理钱包地址
					clobClientMu.Lock()
					signerAddr := ""
					proxyAddr := ""
					if clobOrderClient != nil {
						signerAddr = clobOrderClient.GetSignerAddress()
						proxyAddr = clobOrderClient.GetAddress()
					}
					clobClientMu.Unlock()

					if signerAddr != "" && proxyAddr != "" {
						// 15分钟加密货币市场通常不是 NegRisk 市场
						isNegRisk := false
						log.Printf("💰 [Pattern][%s] 计划自动赎回: %s (Condition: %s)", t.MarketSlug, resolution, conditionID)
						red.ScheduleRedemption(t.MarketSlug, conditionID, winningTokenID, signerAddr, proxyAddr, t.Position.Amount, true, isNegRisk, t.ConfigID)
					}
				}
			}

			// 9. 清除持仓
			patternMarketStatesMu.Lock()
			delete(t.State.Positions, t.ConfigID)
			patternMarketStatesMu.Unlock()
		}(task)
	}
}

// RestorePatternPositions 从数据库恢复未决议的 Pattern 持仓
func RestorePatternPositions() {
	unresolvedTrades, err := database.GetUnresolvedPatternTrades()
	if err != nil {
		log.Printf("⚠️ [Pattern] 恢复持仓失败: %v", err)
		return
	}

	if len(unresolvedTrades) == 0 {
		log.Println("ℹ️ [Pattern] 没有需要恢复的持仓")
		return
	}

	log.Printf("📋 启动检查: 发现 %d 条未决议交易", len(unresolvedTrades))

	patternMarketStatesMu.Lock()
	defer patternMarketStatesMu.Unlock()

	restoredCount := 0
	skippedCount := 0
	for _, trade := range unresolvedTrades {
		configID := strings.TrimPrefix(trade.ConfigGroup, "Pattern-")

		// 检查 TokenID 是否存在 (旧记录可能没有)
		if trade.TokenID == "" {
			log.Printf("⚠️ [启动恢复][%s] %s: TokenID 为空，无法恢复 (旧记录)",
				trade.ConfigGroup, trade.MarketSlug)
			skippedCount++
			continue
		}

		state, exists := patternMarketStates[trade.MarketSlug]
		if !exists {
			// 从 slug 解析 EndTime (格式: btc-updown-15m-1770194700)
			endTime := parseEndTimeFromSlug(trade.MarketSlug)

			state = &PatternMarketState{
				Slug:         trade.MarketSlug,
				Symbol:       trade.Symbol, // 从数据库恢复 Symbol
				EndTime:      endTime,      // 从 slug 解析 EndTime
				Positions:    make(map[string]*PatternPosition),
				LatestPrices: make(map[string]float64),
			}
			patternMarketStates[trade.MarketSlug] = state

			if !endTime.IsZero() {
				log.Printf("📅 [启动恢复] %s: EndTime=%v", trade.MarketSlug, endTime.Format("15:04:05"))
			}
		}

		if state.Positions == nil {
			state.Positions = make(map[string]*PatternPosition)
		}

		// 如果 state 的 Symbol 为空，从 trade 恢复
		if state.Symbol == "" && trade.Symbol != "" {
			state.Symbol = trade.Symbol
		}

		position := &PatternPosition{
			ConfigID:   configID,
			MarketSlug: trade.MarketSlug,
			Symbol:     trade.Symbol, // 从数据库恢复
			Side:       trade.Side,
			TokenID:    trade.TokenID, // 从数据库恢复 (关键!)
			BuyPrice:   trade.Price,
			Amount:     trade.Amount,
			Cost:       trade.Cost,
			RecordID:   trade.ID,
			Pending:    false,
		}

		state.Positions[configID] = position
		restoredCount++

		log.Printf("🔄 [启动恢复][%s] %s: 方向=%s | Token=%s... | 价格=%.4f | 数量=%.2f | 成本=$%.2f",
			trade.ConfigGroup, trade.MarketSlug, trade.Side,
			trade.TokenID[:min(8, len(trade.TokenID))],
			trade.Price, trade.Amount, trade.Cost)
	}

	if skippedCount > 0 {
		log.Printf("⚠️ [Pattern] 跳过 %d 个持仓 (缺少 TokenID)", skippedCount)
	}
	log.Printf("✅ [Pattern] 成功恢复 %d 个持仓", restoredCount)
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ==================== Sniper v4.4 买入检测 ====================

func checkSniperBuySignal(state *PatternMarketState, cfg PatternStrategyConfig, binanceSymbol string) {
	// ===== Debug日志控制 (每30秒一次) =====
	debugKey := cfg.ID + "_" + state.Slug + "_trace"
	patternMarketStatesMu.Lock()
	lastDebug, hasDebug := state.LastPatternCheck[debugKey]
	shouldDebug := !hasDebug || time.Since(lastDebug) > 30*time.Second
	if shouldDebug {
		state.LastPatternCheck[debugKey] = time.Now()
	}
	patternMarketStatesMu.Unlock()

	// 1. 入口检查日志
	// if shouldDebug {
	// 	log.Printf("🔍 [Sniper TRACE][%s] Checking %s | Symbol=%s", cfg.ID, state.Slug, binanceSymbol)
	// }

	// ===== 复用主策略的 Binance WebSocket K线数据 =====
	if binanceClient == nil {
		if shouldDebug {
			log.Printf("⚠️ [Sniper TRACE][%s] BinanceClient is NIL", cfg.ID)
		}
		return
	}

	klines := binanceClient.GetHistoricalKlines(binanceSymbol, 240)
	if len(klines) < 50 {
		if shouldDebug {
			log.Printf("⚠️ [Sniper TRACE][%s] K-Lines insufficient: %d < 50 | Symbol=%s", cfg.ID, len(klines), binanceSymbol)
		}
		return
	}

	closes := src.GetCloses(klines)
	currentPrice := closes[len(closes)-1]

	// 计算剩余时间
	timeLeftSec := time.Until(state.EndTime).Seconds()
	if timeLeftSec < 0 {
		timeLeftSec = 0
	}

	// StrikePrice 验证 (数据未就绪时跳过)
	if state.StrikePrice == 0 {
		// if shouldDebug {
		// 	log.Printf("⚠️ [Sniper TRACE][%s] StrikePrice is 0 | Market=%s", cfg.ID, state.Slug)
		// }
		return
	}

	// 计算实时波动率 (最近10根K线)
	sigma := src.ComputeRealtimeSigma(klines, 10)
	if sigma <= 1e-6 {
		// 动态保底: 如果计算出的波动率为0（例如数据不足），使用当前价格的 0.5% 作为预估波动率
		if len(klines) > 0 {
			lastPrice := klines[len(klines)-1].Close
			sigma = lastPrice * 0.005
		} else {
			sigma = 1.0 // 无法获取价格时的兜底
		}
	}

	// 获取1分钟高低点 (使用前一根已完成K线)
	high1m, low1m := src.Get1MinHighLowFromLast2(klines)

	// 获取 Polymarket 价格
	polyPriceUp := state.LatestPrices["UP"]
	polyPriceDown := state.LatestPrices["DOWN"]

	// 计算价差 (用于滑点熔断)
	polySpread := 0.0
	if polyPriceUp > 0 && polyPriceDown > 0 {
		polySpread = 1.0 - (polyPriceUp + polyPriceDown)
		if polySpread < 0 {
			polySpread = -polySpread // 取绝对值
		}
	}

	// 构建 Sniper 输入
	sniperInput := src.SniperInput{
		CurrentPrice:  currentPrice,
		StrikePrice:   state.StrikePrice,
		TimeLeftSec:   timeLeftSec,
		Sigma:         sigma,
		PolyPriceUp:   polyPriceUp,
		PolyPriceDown: polyPriceDown,
		Last1MinHigh:  high1m,
		Last1MinLow:   low1m,
		PolySpread:    polySpread,
	}

	// 执行 Sniper 决策
	decision := src.EvaluateSniper(sniperInput, cfg.SniperConfig)

	// ===== DEBUG: 只能日志控制 (避免刷屏) =====
	// 策略:
	// 1. 关键时刻 (|Z| > 1.5): 每 120 秒输出一次
	// 2. 平静时刻: 每 5 分钟 (300秒) 输出一次
	logInterval := 300 * time.Second
	if math.Abs(decision.ZScore) > 1.5 {
		logInterval = 120 * time.Second
	}

	decisionDebugKey := cfg.ID + "_" + state.Slug
	patternMarketStatesMu.Lock()
	lastDecisionDebug, hasDecisionDebug := state.LastPatternCheck[decisionDebugKey+"_debug"]
	shouldDecisionDebug := !hasDecisionDebug || time.Since(lastDecisionDebug) > logInterval
	if shouldDecisionDebug {
		state.LastPatternCheck[decisionDebugKey+"_debug"] = time.Now()
	}
	patternMarketStatesMu.Unlock()

	if shouldDecisionDebug {
		// 计算额外调试信息
		isLongBreakout, isShortBreakout := src.CheckBreakout(currentPrice, high1m, low1m)
		// theoProb := src.NormalCDF(decision.ZScore)
		// edgeUp := theoProb - polyPriceUp
		// edgeDown := (1 - theoProb) - polyPriceDown

		_ = isLongBreakout
		_ = isShortBreakout

		// log.Printf("🔍 [Sniper 调试][%s] 市场=%s | 阶段=%d | 剩余时间=%.0fs",
		// 	cfg.ID, state.Slug, decision.Phase, timeLeftSec)
		// log.Printf("   ├─ 价格: 币安=%.2f | 行权价=%.2f | 波动率=%.2f | Z值=%.3f (阈值=%.1f)",
		// 	currentPrice, state.StrikePrice, sigma, decision.ZScore, cfg.SniperConfig.ZScoreEntryThreshold)
		// log.Printf("   ├─ Poly: 上涨=%.4f | 下跌=%.4f | 价差=%.4f (阈值=%.4f)",
		// 	polyPriceUp, polyPriceDown, polySpread, cfg.SniperConfig.SpreadMax)
		// log.Printf("   ├─ 优势(Edge): 看涨=%.4f | 看跌=%.4f (阈值=%.4f)",
		// 	edgeUp, edgeDown, cfg.SniperConfig.EdgeThreshold)
		// log.Printf("   ├─ 突破: 看涨突破=%v | 看跌突破=%v | 1分高点=%.2f | 1分低点=%.2f",
		// 	isLongBreakout, isShortBreakout, high1m, low1m)
		// log.Printf("   └─ 决策: 动作=%s | 原因=%s",
		// 	decision.Action, decision.Reason)
	}

	// 日志: 阶段和决策 (仅非HOLD时) - 暂时注释掉以减少刷屏，只由下单逻辑打印
	// if decision.Action != "HOLD" {
	// 	log.Printf("🎯 [Sniper][%s] Phase=%d | Z=%.2f | Prob=%.2f | Edge=%.3f | Action=%s (%s)",
	// 		cfg.ID, decision.Phase, decision.ZScore, decision.TheoProb, decision.Edge,
	// 		decision.Action, decision.Reason)
	// }

	// 处理决策结果
	if decision.Action == "HOLD" {
		return
	}

	// 确定买入方向和价格
	var side string
	var price float64

	switch decision.Action {
	case "BUY_UP":
		side = "UP"
		price = polyPriceUp
	case "BUY_DOWN":
		side = "DOWN"
		price = polyPriceDown
	case "SELL_UP", "SELL_DOWN":
		// 做空逻辑暂不实现 (需要持有对应头寸)
		log.Printf("⚠️ [Sniper][%s] 做空信号暂不支持: %s", cfg.ID, decision.Action)
		return
	case "EXIT":
		// 退出信号由 sell 逻辑处理
		return
	default:
		return
	}

	// 检查价格有效性
	if price <= 0 {
		return
	}

	// ===== 冷却期检查 (30秒) =====
	cooldownKey := cfg.ID + "_" + side
	if state.LastSellTime != nil {
		if lastSell, ok := state.LastSellTime[cooldownKey]; ok {
			if time.Since(lastSell) < 30*time.Second {
				// 冷却期内，跳过买入
				return
			}
		}
	}

	// 获取买入数量
	amount := cfg.GetShareAmount(price)
	if amount <= 0 {
		return
	}

	// 获取 TokenID
	tokenID, ok := state.SideToTokenID[side]
	if !ok || tokenID == "" {
		return
	}

	// ===== 防止重复下单 =====
	patternMarketStatesMu.Lock()
	if state.Positions == nil {
		state.Positions = make(map[string]*PatternPosition)
	}
	if existing, ok := state.Positions[cfg.ID]; ok && existing != nil {
		patternMarketStatesMu.Unlock()
		return
	}
	// 设置 pending 占位
	state.Positions[cfg.ID] = &PatternPosition{
		ConfigID: cfg.ID,
		Pending:  true,
	}
	patternMarketStatesMu.Unlock()

	log.Printf("🎯 [Sniper][%s] 触发买入: 市场=%s | 方向=%s | 价格=%.4f | 原因=%s",
		cfg.ID, state.Slug, side, price, decision.Reason)

	// 构建指标快照 (简化版)
	buyIndicators := BuyIndicatorSnapshot{}

	// 执行买入 (复用现有函数)
	executePatternBuy(state, cfg, side, tokenID, price, amount, "Sniper:"+decision.Reason, buyIndicators)
}
