package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"Polymarket15MinsCryptoArbitrageTool/internal/database"
	"Polymarket15MinsCryptoArbitrageTool/src"
)

// MarketState 市场状态 (简化版，只支持 Leg1)
type MarketState struct {
	ConfigGroup   string
	Slug          string
	Symbol        string
	StartTime     time.Time
	EndTime       time.Time
	TokenIDToSide map[string]string // TokenID -> "UP" / "DOWN"
	SideToTokenID map[string]string // "UP" / "DOWN" -> TokenID
	LatestPrices  map[string]float64

	// Leg1 状态
	Leg1Filled   bool
	Leg1Side     string
	Leg1Price    float64
	Leg1Amount   float64
	Leg1RecordID int64
	Leg1Time     time.Time

	// 跳过标记
	Leg1SkippedHighPrice bool
}

// 全局状态
var (
	simMu               sync.Mutex
	isSimRunning        bool
	binanceClient       *src.BinanceKlineClient
	lastPriceUpdateTime time.Time
	lastPriceUpdateMu   sync.Mutex
	priceUpdateCount    int64
	priceUpdateCountMu  sync.Mutex
)

// StartType2MultiConfigSimulation 启动多配置模拟 (幂等)
func StartType2MultiConfigSimulation(configs []Type2Config) bool {
	simMu.Lock()
	defer simMu.Unlock()

	if isSimRunning {
		log.Println("⚠️ Type2 策略模拟已在运行。")
		return false
	}

	if len(configs) == 0 {
		log.Println("⚠️ 没有配置可运行")
		return false
	}

	isSimRunning = true
	go runSimulationLoop(configs)
	log.Printf("🚀 Type2 策略模拟已启动 - %d 个配置组", len(configs))

	for _, cfg := range configs {
		log.Printf("📝 配置组[%s]: MinTrendUp=%.4f, MinTrendDown=%.4f, MinPriceEdge=%.2f",
			cfg.ID, cfg.MinTrendPercentUp, cfg.MinTrendPercentDown, cfg.MinPriceEdge)
	}

	return true
}

// runSimulationLoop 模拟循环
func runSimulationLoop(configs []Type2Config) {
	// 1. 初始化 Polymarket WebSocket
	sdk := src.New(src.WithLogger(src.NewSilentLogger()))

	if err := sdk.Connect(); err != nil {
		log.Printf("❌ Polymarket WS 连接错误: %v", err)
		isSimRunning = false
		return
	}
	defer sdk.Disconnect()

	log.Println("✅ 已连接到 Polymarket WS")

	// 2. 初始化 Binance WebSocket
	binanceSymbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "XRPUSDT"}
	binanceClient = src.NewBinanceKlineClient(binanceSymbols, src.NewLogger(src.LogLevelInfo))
	if err := binanceClient.Connect(); err != nil {
		log.Printf("⚠️ Binance WS 连接失败 (无法使用趋势检测): %v", err)
	} else {
		defer binanceClient.Close()
		log.Println("✅ 已连接到 Binance WS (监听 15m K线)")
	}

	// 3. 状态管理
	allConfigStates := make(map[string]map[string]*MarketState)
	allPendingResolutions := make(map[string]map[string]*MarketState)

	for _, cfg := range configs {
		allConfigStates[cfg.ID] = make(map[string]*MarketState)
		allPendingResolutions[cfg.ID] = make(map[string]*MarketState)
	}

	// 4. 共享市场发现
	sharedMarketTokens := make(map[string]map[string]string)

	// 5. 目标符号
	targets := []string{"Bitcoin", "Ethereum", "Solana", "XRP"}

	// Gamma 客户端
	gammaClient := src.NewGammaClient(http.DefaultClient)

	// 定时器
	scanInterval := configs[0].ScanIntervalSec
	restTicker := time.NewTicker(time.Duration(scanInterval) * time.Second)
	resTicker := time.NewTicker(60 * time.Second)
	heartbeatTicker := time.NewTicker(60 * time.Second)
	defer restTicker.Stop()
	defer resTicker.Stop()
	defer heartbeatTicker.Stop()

	// 初始扫描
	scanAndSubscribe(sdk, gammaClient, allConfigStates, configs, targets, sharedMarketTokens)

	log.Println("💓 [启动心跳] Type2 策略已启动，进入主循环...")

	for {
		select {
		case <-restTicker.C:
			scanAndSubscribe(sdk, gammaClient, allConfigStates, configs, targets, sharedMarketTokens)

			// 过期检测
			now := time.Now()
			simMu.Lock()
			for configID, states := range allConfigStates {
				for slug, state := range states {
					parts := strings.Split(slug, "-")
					if len(parts) >= 4 {
						ts, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
						if err == nil {
							windowEnd := time.Unix(ts, 0).Add(15 * time.Minute)
							if now.After(windowEnd) {
								if state.Leg1Filled {
									log.Printf("⏰ [%s] 轮次过期，移入等待决议: %s", configID, slug)
									allPendingResolutions[configID][slug] = state
								}
								delete(states, slug)
							}
						}
					}
				}
			}
			simMu.Unlock()

		case <-resTicker.C:
			// 检查决议结果
			for i, cfg := range configs {
				if pending, ok := allPendingResolutions[cfg.ID]; ok && len(pending) > 0 {
					checkPendingResolutions(gammaClient, pending, configs[i])
				}
			}

		case <-heartbeatTicker.C:
			simMu.Lock()
			totalActive := 0
			totalPending := 0
			for _, states := range allConfigStates {
				totalActive += len(states)
			}
			for _, pending := range allPendingResolutions {
				totalPending += len(pending)
			}
			simMu.Unlock()

			binanceStatus := "未连接"
			if binanceClient != nil {
				binanceStatus = "已连接"
			}

			lastPriceUpdateMu.Lock()
			lastUpdateAgo := "N/A"
			if !lastPriceUpdateTime.IsZero() {
				lastUpdateAgo = fmt.Sprintf("%.0f秒前", time.Since(lastPriceUpdateTime).Seconds())
			}
			lastPriceUpdateMu.Unlock()

			priceUpdateCountMu.Lock()
			totalCallbacks := priceUpdateCount
			priceUpdateCountMu.Unlock()

			log.Printf("💓 [心跳] 活跃市场: %d, 等待决议: %d, Binance: %s, 最后更新: %s, 回调数: %d",
				totalActive, totalPending, binanceStatus, lastUpdateAgo, totalCallbacks)
		}
	}
}

var handlersRegistered bool

// scanAndSubscribe 扫描市场并订阅
func scanAndSubscribe(
	sdk *src.WsClientWrapper,
	gammaClient *src.Client,
	allConfigStates map[string]map[string]*MarketState,
	configs []Type2Config,
	targets []string,
	sharedMarketTokens map[string]map[string]string,
) {
	windowMin := 60
	newMarkets := scanNewMarkets(gammaClient, sharedMarketTokens, targets, windowMin)

	if len(newMarkets) == 0 {
		return
	}

	simMu.Lock()
	var newTokenIDs []string

	for _, market := range newMarkets {
		if _, exists := sharedMarketTokens[market.Slug]; !exists {
			sharedMarketTokens[market.Slug] = make(map[string]string)
			for tid, side := range market.TokenIDToSide {
				sharedMarketTokens[market.Slug][tid] = side
				newTokenIDs = append(newTokenIDs, tid)
			}
		}

		for _, cfg := range configs {
			if _, exists := allConfigStates[cfg.ID][market.Slug]; exists {
				continue
			}

			newState := &MarketState{
				ConfigGroup:   cfg.ID,
				Slug:          market.Slug,
				Symbol:        market.Symbol,
				StartTime:     market.StartTime,
				EndTime:       market.EndTime,
				TokenIDToSide: make(map[string]string),
				SideToTokenID: make(map[string]string),
				LatestPrices:  make(map[string]float64),
			}

			for tid, side := range market.TokenIDToSide {
				newState.TokenIDToSide[tid] = side
				newState.SideToTokenID[side] = tid
			}

			allConfigStates[cfg.ID][market.Slug] = newState
		}
	}
	simMu.Unlock()

	// 订阅
	if len(newTokenIDs) > 0 {
		filter := &src.CLOBMarketFilter{TokenIDs: newTokenIDs}

		if !handlersRegistered {
			log.Printf("🔌 首次订阅 %d 个资产...", len(newTokenIDs))

			err := sdk.SubscribeToCLOBMarketLastTradePrice(filter, func(price src.LastTradePrice) error {
				priceUpdateCountMu.Lock()
				priceUpdateCount++
				priceUpdateCountMu.Unlock()

				lastPriceUpdateMu.Lock()
				lastPriceUpdateTime = time.Now()
				lastPriceUpdateMu.Unlock()

				simMu.Lock()
				defer simMu.Unlock()

				for i, cfg := range configs {
					states := allConfigStates[cfg.ID]
					for _, state := range states {
						if side, ok := state.TokenIDToSide[price.AssetID]; ok {
							p, _ := parsePrice(price.Price.String())
							if state.LatestPrices == nil {
								state.LatestPrices = make(map[string]float64)
							}
							state.LatestPrices[side] = p
							checkBinanceArbitrage(state, configs[i])
						}
					}
				}
				return nil
			})

			if err != nil {
				log.Printf("❌ 订阅 LastTradePrice 失败: %v", err)
			}

			handlersRegistered = true
		} else {
			log.Printf("🔌 订阅 %d 个新资产", len(newTokenIDs))
			sdk.SubscribeToCLOBMarketLastTradePrice(filter, nil)
		}
	}
}

// MarketInfo 市场信息
type MarketInfo struct {
	Slug          string
	Symbol        string
	StartTime     time.Time
	EndTime       time.Time
	TokenIDToSide map[string]string
}

// scanNewMarkets 扫描新市场
func scanNewMarkets(
	client *src.Client,
	existingMarkets map[string]map[string]string,
	targets []string,
	windowMin int,
) []MarketInfo {
	var newMarkets []MarketInfo

	for _, target := range targets {
		query := fmt.Sprintf("%s Up or Down", target)
		markets, err := client.SearchMarkets(query, src.MarketFilterActive, true)
		if err != nil {
			continue
		}

		for _, m := range markets {
			tokens := m.GetTokens()
			if m.Closed || !m.Active || len(tokens) < 2 {
				continue
			}

			// 检查是否是15分钟窗口市场
			if !strings.Contains(m.Slug, "-") {
				continue
			}

			parts := strings.Split(m.Slug, "-")
			if len(parts) < 4 {
				continue
			}

			ts, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
			if err != nil {
				continue
			}

			windowStart := time.Unix(ts, 0)
			windowEnd := windowStart.Add(15 * time.Minute)
			now := time.Now()

			// 检查时间窗口
			if now.After(windowEnd) {
				continue
			}
			if now.Before(windowStart.Add(-time.Duration(windowMin) * time.Minute)) {
				continue
			}

			// 检查是否已存在
			if _, exists := existingMarkets[m.Slug]; exists {
				continue
			}

			// 解析 Token 方向
			tokenIDToSide := make(map[string]string)
			for _, token := range tokens {
				outcome := strings.ToUpper(token.Outcome)
				if outcome == "UP" || outcome == "DOWN" {
					tokenIDToSide[token.TokenID] = outcome
				}
			}

			if len(tokenIDToSide) != 2 {
				continue
			}

			symbol := extractSymbol(target)
			market := MarketInfo{
				Slug:          m.Slug,
				Symbol:        symbol,
				StartTime:     windowStart,
				EndTime:       windowEnd,
				TokenIDToSide: tokenIDToSide,
			}
			newMarkets = append(newMarkets, market)

			log.Printf("📊 发现新市场: %s [%s] 结束于 %s", m.Slug, symbol, windowEnd.Format("15:04"))
		}
	}

	return newMarkets
}

// extractSymbol 提取符号
func extractSymbol(target string) string {
	switch target {
	case "Bitcoin":
		return "BTC"
	case "Ethereum":
		return "ETH"
	case "Solana":
		return "SOL"
	case "XRP":
		return "XRP"
	default:
		return target
	}
}

// checkBinanceArbitrage 检查 Binance 套利机会 (Leg1)
func checkBinanceArbitrage(state *MarketState, config Type2Config) {
	// 已经有持仓，跳过
	if state.Leg1Filled {
		return
	}

	// 检查时间窗口
	now := time.Now()
	elapsed := now.Sub(state.StartTime)
	if elapsed.Seconds() < float64(config.TrendWindowSecStart) || elapsed.Seconds() > float64(config.TrendWindowSecEnd) {
		return
	}

	// 获取 Binance 趋势
	if binanceClient == nil {
		return
	}

	binanceSymbol := state.Symbol + "USDT"
	kline := binanceClient.GetLatestKline(binanceSymbol)
	if kline == nil {
		return
	}

	// 计算趋势
	trendPercent := (kline.Close - kline.Open) / kline.Open

	// 判断方向
	var targetSide string
	if trendPercent >= config.MinTrendPercentUp {
		targetSide = "UP"
	} else if trendPercent <= -config.MinTrendPercentDown {
		targetSide = "DOWN"
	} else {
		return // 趋势不够明显
	}

	// 获取 Polymarket 价格
	price, ok := state.LatestPrices[targetSide]
	if !ok || price <= 0 {
		return
	}

	// 计算公允价格
	fairPrice := 0.5 + trendPercent*config.FairPriceTrendFactor
	if fairPrice < 0.1 {
		fairPrice = 0.1
	}
	if fairPrice > 0.9 {
		fairPrice = 0.9
	}

	// 检查价格优势
	priceEdge := fairPrice - price
	if priceEdge < config.MinPriceEdge {
		return
	}

	// 检查价格范围
	if price < config.Leg0SharePrice0 {
		return // 价格太低，可能有问题
	}
	if price > config.SharePriceLevel3 {
		if !state.Leg1SkippedHighPrice {
			log.Printf("⏭️ [%s][%s] 价格 %.3f 超过阈值 %.2f，跳过", state.ConfigGroup, state.Slug, price, config.SharePriceLevel3)
			state.Leg1SkippedHighPrice = true
		}
		return
	}

	// 确定购买数量
	var amount float64
	if price <= config.SharePriceLevel1 {
		amount = config.ShareAmount1
	} else if price <= config.SharePriceLevel2 {
		amount = config.ShareAmount2
	} else {
		amount = config.ShareAmount3
	}

	// 执行 Leg1 买入 (模拟)
	cost := price * amount
	log.Printf("📈 [%s][%s] Leg1 买入 %s: 价格=%.3f, 数量=%.0f, 成本=$%.2f, Binance趋势=%.4f, 公允价=%.3f",
		state.ConfigGroup, state.Slug, targetSide, price, amount, cost, trendPercent, fairPrice)

	// 记录到数据库
	trade := &database.SimulationTrade{
		StrategyType: "type2",
		ConfigGroup:  state.ConfigGroup,
		MarketSlug:   state.Slug,
		Symbol:       state.Symbol,
		Stage:        "Leg1",
		Side:         targetSide,
		Price:        price,
		Amount:       amount,
		Cost:         cost,
		BinancePrice: kline.Close,
		TrendPercent: trendPercent,
	}

	id, err := database.InsertTrade(trade)
	if err != nil {
		log.Printf("❌ 保存交易记录失败: %v", err)
		return
	}

	// 更新状态
	state.Leg1Filled = true
	state.Leg1Side = targetSide
	state.Leg1Price = price
	state.Leg1Amount = amount
	state.Leg1RecordID = id
	state.Leg1Time = now
}

// checkPendingResolutions 检查等待决议的交易
func checkPendingResolutions(client *src.Client, pending map[string]*MarketState, config Type2Config) {
	for slug, state := range pending {
		if !state.Leg1Filled {
			delete(pending, slug)
			continue
		}

		// 查询市场决议
		markets, err := client.SearchMarkets(slug, "", true)
		if err != nil || len(markets) == 0 {
			continue
		}

		market := markets[0]
		if !market.Closed {
			continue // 市场尚未决议
		}

		// 获取决议结果
		var resolution string
		marketTokens := market.GetTokens()
		for _, token := range marketTokens {
			if token.Winner {
				resolution = strings.ToUpper(token.Outcome)
				break
			}
		}

		if resolution == "" {
			continue
		}

		// 计算 PnL
		var pnl float64
		if resolution == state.Leg1Side {
			// 胜利: 获得 amount * 1 - cost
			pnl = state.Leg1Amount - (state.Leg1Price * state.Leg1Amount)
		} else {
			// 失败: 损失全部 cost
			pnl = -(state.Leg1Price * state.Leg1Amount)
		}

		log.Printf("✅ [%s][%s] 决议: %s, 持仓: %s, PnL: $%.2f",
			state.ConfigGroup, slug, resolution, state.Leg1Side, pnl)

		// 更新数据库
		if err := database.UpdateTradeResolution(state.Leg1RecordID, resolution, pnl); err != nil {
			log.Printf("❌ 更新决议失败: %v", err)
		}

		delete(pending, slug)
	}
}

// parsePrice 解析价格字符串
func parsePrice(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
