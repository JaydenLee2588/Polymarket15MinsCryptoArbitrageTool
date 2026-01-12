package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
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

	// 真实交易客户端
	clobOrderClient *src.ClobOrderClient
	clobClientMu    sync.Mutex
)

// StartSimulation 启动多配置模拟 (幂等)
func StartSimulation(configs []StrategyConfig) bool {
	simMu.Lock()
	defer simMu.Unlock()

	if isSimRunning {
		log.Println("⚠️ 策略模拟已在运行。")
		return false
	}

	if len(configs) == 0 {
		log.Println("⚠️ 没有配置可运行")
		return false
	}

	// 检查是否有配置使用真实交易模式
	hasRealMode := false
	for _, cfg := range configs {
		if cfg.TradingMode == "real" {
			hasRealMode = true
			break
		}
	}

	// 如果有真实交易模式，初始化 ClobOrderClient
	if hasRealMode {
		privateKey := os.Getenv("POLYMARKET_PRIVATE_KEY")
		if privateKey == "" {
			log.Println("❌ 真实交易模式需要 POLYMARKET_PRIVATE_KEY 环境变量")
			return false
		}

		// 创建 CLOB 订单客户端
		client, err := src.NewClobOrderClient(privateKey)
		if err != nil {
			log.Printf("❌ 创建 ClobOrderClient 失败: %v", err)
			return false
		}

		// 派生 API 凭证
		authClient, err := src.NewAuthClient(privateKey)
		if err != nil {
			log.Printf("❌ 创建 AuthClient 失败: %v", err)
			return false
		}

		creds, err := authClient.DeriveAPICredentials(context.Background())
		if err != nil {
			log.Printf("❌ 派生 API 凭证失败: %v", err)
			return false
		}

		// 设置凭证到客户端
		client.SetCredentials(creds)

		// 设置全局客户端
		clobClientMu.Lock()
		clobOrderClient = client
		clobClientMu.Unlock()

		log.Printf("✅ 真实交易模式已启用，钱包地址: %s", client.GetAddress())
	}

	isSimRunning = true
	go runSimulationLoop(configs)
	log.Printf("🚀 策略模拟已启动 - %d 个配置组", len(configs))

	for _, cfg := range configs {
		modeTag := "模拟"
		if cfg.TradingMode == "real" {
			modeTag = "💰真实"
		}
		orderTypeTag := "市价"
		if cfg.OrderType == "limit" {
			orderTypeTag = "挂单"
		}
		log.Printf("📝 配置组[%s]: MinTrendUp=%.4f, MinTrendDown=%.4f, Mode=%s, OrderType=%s",
			cfg.ID, cfg.MinTrendPercentUp, cfg.MinTrendPercentDown, modeTag, orderTypeTag)
	}

	return true
}

// runSimulationLoop 模拟循环
func runSimulationLoop(configs []StrategyConfig) {
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

	// 启动时检查等待决议的交易
	loadPendingFromDatabase(gammaClient, allPendingResolutions, configs)

	log.Println("💓 [启动心跳] 策略已启动，进入主循环...")

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
	configs []StrategyConfig,
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
	SideToTokenID map[string]string // 反向映射，用于真实下单
}

// scanNewMarkets 扫描新市场 (采用两步获取模式)
func scanNewMarkets(
	client *src.Client,
	existingMarkets map[string]map[string]string,
	targets []string,
	windowMin int,
) []MarketInfo {
	var newMarkets []MarketInfo
	now := time.Now()

	// Slug 前缀到 Target 名称的映射
	prefixToTarget := map[string]string{
		"btc-updown-15m": "Bitcoin",
		"eth-updown-15m": "Ethereum",
		"sol-updown-15m": "Solana",
		"xrp-updown-15m": "XRP",
	}

	for _, target := range targets {
		// 构造更精确的搜索查询 (与 FKGoPolyTools 一致)
		var searchQuery string
		switch target {
		case "Bitcoin":
			searchQuery = "btc updown 15m"
		case "Ethereum":
			searchQuery = "eth updown 15m"
		case "Solana":
			searchQuery = "sol updown 15m"
		case "XRP":
			searchQuery = "xrp updown 15m"
		default:
			continue
		}

		// 第一步：搜索获取事件
		resp, err := client.Search(context.Background(), &src.SearchParams{
			Q:            searchQuery,
			EventsStatus: "active",
		})
		if err != nil {
			log.Printf("⚠️ %s 搜索失败: %v", target, err)
			continue
		}

		if len(resp.Events) == 0 {
			log.Printf("⚠️ %s 搜索无结果 (query: %s)", target, searchQuery)
			continue
		}

		// 遍历事件
		foundForTarget := 0
		for _, event := range resp.Events {
			// 必须是 15m 市场
			if !strings.Contains(event.Slug, "15m") {
				continue
			}

			// 必须匹配目标币种前缀
			matched := false
			for prefix, t := range prefixToTarget {
				if strings.HasPrefix(event.Slug, prefix) && t == target {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}

			// 解析时间戳
			parts := strings.Split(event.Slug, "-")
			if len(parts) < 4 {
				continue
			}
			ts, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
			if err != nil {
				continue
			}
			windowStart := time.Unix(ts, 0)
			windowEnd := windowStart.Add(15 * time.Minute)

			// 必须是当前活跃窗口
			if now.Before(windowStart) || now.After(windowEnd) {
				continue
			}

			// 检查是否已存在
			if _, exists := existingMarkets[event.Slug]; exists {
				continue
			}

			// 第二步：获取完整市场详情
			markets, err := client.GetMarkets(context.Background(), &src.GetMarketsParams{Slug: []string{event.Slug}})
			if err != nil || len(markets) == 0 {
				log.Printf("⚠️ 获取市场详情失败: %s, err: %v", event.Slug, err)
				continue
			}

			m := markets[0]

			// 基础验证
			if !m.Active || m.Closed {
				continue
			}

			// 解析 ClobTokenIds
			tokenIDs := []string(m.ClobTokenIDs)
			if len(tokenIDs) != 2 {
				log.Printf("⚠️ 跳过 %s: 期望 2 个 TokenID, 实际得到 %d 个", m.Slug, len(tokenIDs))
				continue
			}

			// 动态 Side 映射：根据 Outcomes 文本判断 UP/DOWN
			tokenIDToSide := make(map[string]string)
			for i, outcome := range m.Outcomes {
				if i >= len(tokenIDs) {
					break
				}
				lowOutcome := strings.ToLower(outcome)
				side := "DOWN" // 默认
				if strings.Contains(lowOutcome, "up") || strings.Contains(lowOutcome, "yes") || strings.Contains(lowOutcome, "over") || strings.Contains(lowOutcome, "above") {
					side = "UP"
				}
				tokenIDToSide[tokenIDs[i]] = side
			}

			// 检查映射冲突
			if len(tokenIDs) == 2 {
				side0 := tokenIDToSide[tokenIDs[0]]
				side1 := tokenIDToSide[tokenIDs[1]]
				if side0 == side1 {
					if side0 == "DOWN" {
						tokenIDToSide[tokenIDs[0]] = "UP"
					} else {
						tokenIDToSide[tokenIDs[1]] = "DOWN"
					}
					log.Printf("⚠️ 检测到 Side 映射冲突，已强制修正")
				}
			}

			if len(tokenIDToSide) != 2 {
				continue
			}

			foundForTarget++
			symbol := extractSymbol(target)

			// 构建反向映射 (Side -> TokenID)
			sideToTokenID := make(map[string]string)
			for tid, side := range tokenIDToSide {
				sideToTokenID[side] = tid
			}

			market := MarketInfo{
				Slug:          m.Slug,
				Symbol:        symbol,
				StartTime:     windowStart,
				EndTime:       windowEnd,
				TokenIDToSide: tokenIDToSide,
				SideToTokenID: sideToTokenID,
			}
			newMarkets = append(newMarkets, market)

			log.Printf("📊 发现新市场: %s [%s] 结束于 %s", m.Slug, symbol, windowEnd.Format("15:04"))
		}

		// 如果搜索没找到，尝试直接构造当前窗口的 slug
		if foundForTarget == 0 {
			currentWindow := now.Truncate(15 * time.Minute)
			timestamp := currentWindow.Unix()

			var prefix string
			switch target {
			case "Bitcoin":
				prefix = "btc-updown-15m"
			case "Ethereum":
				prefix = "eth-updown-15m"
			case "Solana":
				prefix = "sol-updown-15m"
			case "XRP":
				prefix = "xrp-updown-15m"
			}

			if prefix != "" {
				slug := fmt.Sprintf("%s-%d", prefix, timestamp)

				// 检查是否已存在
				if _, exists := existingMarkets[slug]; exists {
					continue
				}

				markets, err := client.GetMarkets(context.Background(), &src.GetMarketsParams{Slug: []string{slug}})
				if err == nil && len(markets) > 0 {
					m := markets[0]
					if m.Active && !m.Closed {
						tokenIDs := []string(m.ClobTokenIDs)
						if len(tokenIDs) == 2 {
							log.Printf("📊 通过直接构造发现市场: %s", m.Slug)

							// 动态 Side 映射：根据 Outcomes 文本判断 UP/DOWN (与 FKGoPolyTools 一致)
							tokenIDToSide := make(map[string]string)
							for i, outcome := range m.Outcomes {
								if i >= len(tokenIDs) {
									break
								}
								lowOutcome := strings.ToLower(outcome)
								side := "DOWN" // 默认
								if strings.Contains(lowOutcome, "up") || strings.Contains(lowOutcome, "yes") || strings.Contains(lowOutcome, "over") || strings.Contains(lowOutcome, "above") {
									side = "UP"
								}
								tokenIDToSide[tokenIDs[i]] = side
							}

							// 检查映射冲突
							if len(tokenIDs) == 2 {
								side0 := tokenIDToSide[tokenIDs[0]]
								side1 := tokenIDToSide[tokenIDs[1]]
								if side0 == side1 {
									if side0 == "DOWN" {
										tokenIDToSide[tokenIDs[0]] = "UP"
									} else {
										tokenIDToSide[tokenIDs[1]] = "DOWN"
									}
									log.Printf("⚠️ 检测到 Side 映射冲突，已强制修正")
								}
							}

							symbol := extractSymbol(target)

							// 构建反向映射 (Side -> TokenID)
							sideToTokenID := make(map[string]string)
							for tid, side := range tokenIDToSide {
								sideToTokenID[side] = tid
							}

							market := MarketInfo{
								Slug:          m.Slug,
								Symbol:        symbol,
								StartTime:     currentWindow,
								EndTime:       currentWindow.Add(15 * time.Minute),
								TokenIDToSide: tokenIDToSide,
								SideToTokenID: sideToTokenID,
							}
							newMarkets = append(newMarkets, market)
						}
					}
				}
			}
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
func checkBinanceArbitrage(state *MarketState, config StrategyConfig) {
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
		// 调试：价格未收到
		log.Printf("🔍 [调试][%s] 价格未收到: side=%s, prices=%v", state.Slug, targetSide, state.LatestPrices)
		return
	}

	// 计算公允价格 (与 FKGoPolyTools 一致)
	// UP 用正趋势，DOWN 用趋势取反 (使之为正)
	// 趋势需要乘以 100 (0.0015 -> 0.15)
	var trendFactor float64
	if targetSide == "UP" {
		trendFactor = (trendPercent * 100) * config.FairPriceTrendFactor
	} else {
		trendFactor = (-trendPercent * 100) * config.FairPriceTrendFactor // DOWN 取反
	}
	fairPrice := 0.5 + trendFactor
	if fairPrice < 0.1 {
		fairPrice = 0.1
	}
	if fairPrice > 0.95 {
		fairPrice = 0.95
	}

	// 检查价格优势
	// priceEdge := fairPrice - price
	// if priceEdge < config.MinPriceEdge {
	// 	// 调试：每个市场每60秒打印一次
	// 	elapsedInt := int(elapsed.Seconds())
	// 	if elapsedInt%60 < 2 { // 在每分钟的前2秒内打印
	// 		log.Printf("🔍 [调试][%s][%s] 价格优势不足: edge=%.4f < %.4f (fair=%.3f, price=%.3f, trend=%.4f)",
	// 			state.Slug, targetSide, priceEdge, config.MinPriceEdge, fairPrice, price, trendPercent)
	// 	}
	// 	return
	// }

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

	// 执行 Leg1 买入
	cost := price * amount
	filledPrice := price
	filledAmount := amount
	realOrderID := ""

	// ========== 真实交易模式 ==========
	if config.TradingMode == "real" {
		clobClientMu.Lock()
		client := clobOrderClient
		clobClientMu.Unlock()

		if client == nil {
			log.Printf("❌ [%s] 真实交易模式但 ClobOrderClient 未初始化", config.ID)
			return
		}

		// 获取 TokenID
		tokenID, ok := state.SideToTokenID[targetSide]
		if !ok || tokenID == "" {
			log.Printf("❌ [%s] 无法获取 %s 方向的 TokenID", config.ID, targetSide)
			return
		}

		// 根据 OrderType 选择下单方式
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var orderResult *src.ClobOrderResult
		var orderErr error

		if config.OrderType == "limit" {
			// 限价单 (GTC) - 做 maker，避免 taker 费用
			log.Printf("💰 [%s] 执行限价单: %s %s @ %.4f x %.2f",
				config.ID, targetSide, state.Slug, price, amount)
			orderResult, orderErr = client.PlaceLimitBuyOrder(ctx, tokenID, price, amount)
		} else {
			// 市价单 (FOK, 30秒过期)
			log.Printf("💰 [%s] 执行市价单: %s %s @ %.4f x %.2f",
				config.ID, targetSide, state.Slug, price, amount)
			orderResult, orderErr = client.PlaceMarketBuyOrder(ctx, tokenID, price, amount, 30)
		}
		cancel()

		if orderErr != nil {
			log.Printf("❌ [%s] 下单失败: %v", config.ID, orderErr)
			return
		}

		if !orderResult.Success {
			log.Printf("❌ [%s] 下单被拒绝: %s", config.ID, orderResult.ErrorMsg)
			return
		}

		// 使用实际成交数据
		realOrderID = orderResult.OrderID
		if orderResult.FilledSize > 0 {
			filledAmount = orderResult.FilledSize
		}
		if orderResult.FilledPrice > 0 {
			filledPrice = orderResult.FilledPrice
		}
		cost = filledPrice * filledAmount

		log.Printf("✅ [%s] 真实订单成交! OrderID: %s, 成交价: %.4f, 成交量: %.2f",
			config.ID, realOrderID, filledPrice, filledAmount)
	} else {
		// 模拟模式日志
		log.Printf("📈 [%s][%s] Leg1 买入 %s: 价格=%.3f, 数量=%.0f, 成本=$%.2f, Binance趋势=%.4f, 公允价=%.3f",
			state.ConfigGroup, state.Slug, targetSide, price, amount, cost, trendPercent, fairPrice)
	}

	// 记录到数据库 (使用实际成交数据)
	trade := &database.SimulationTrade{
		ConfigGroup:  state.ConfigGroup,
		MarketSlug:   state.Slug,
		Stage:        "Leg1",
		Side:         targetSide,
		Price:        filledPrice,
		Amount:       filledAmount,
		Cost:         cost,
		TrendPercent: trendPercent,
		FairPrice:    fairPrice,
	}

	id, err := database.InsertTrade(trade)
	if err != nil {
		log.Printf("❌ 保存交易记录失败: %v", err)
		return
	}

	// 更新状态
	state.Leg1Filled = true
	state.Leg1Side = targetSide
	state.Leg1Price = filledPrice
	state.Leg1Amount = filledAmount
	state.Leg1RecordID = id
	state.Leg1Time = now
}

// checkPendingResolutions 检查等待决议的交易
func checkPendingResolutions(client *src.Client, pending map[string]*MarketState, config StrategyConfig) {
	ctx := context.Background()

	for slug, state := range pending {
		if !state.Leg1Filled {
			delete(pending, slug)
			continue
		}

		// 使用 GetMarketBySlug 查询市场 (与 FKGoPolyTools 一致)
		market, err := client.GetMarketBySlug(ctx, slug, nil)
		if err != nil {
			continue
		}

		if !market.Closed {
			continue // 市场尚未决议
		}

		// 通过 OutcomePrices 找到价格为 "1" 的索引 (获胜方向)
		// Index 0 = UP, Index 1 = DOWN
		prices := market.OutcomePrices
		winIndex := -1
		for i, pStr := range prices {
			p, _ := parsePrice(pStr)
			if p > 0.99 {
				winIndex = i
				break
			}
		}

		if winIndex == -1 {
			log.Printf("⚠️ [%s] %s: 已关闭但未找到胜者 (Prices: %v)", state.ConfigGroup, slug, prices)
			continue
		}

		// 判定胜负 (Index 0=UP, Index 1=DOWN)
		resolution := ""
		if winIndex == 0 {
			resolution = "UP"
		} else if winIndex == 1 {
			resolution = "DOWN"
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

// loadPendingFromDatabase 从数据库加载等待决议的交易
func loadPendingFromDatabase(
	client *src.Client,
	allPendingResolutions map[string]map[string]*MarketState,
	configs []StrategyConfig,
) {
	trades, err := database.GetUnresolvedTrades()
	if err != nil {
		log.Printf("⚠️ 加载未决议交易失败: %v", err)
		return
	}

	if len(trades) == 0 {
		log.Println("📋 启动检查: 没有未决议的交易")
		return
	}

	log.Printf("📋 启动检查: 发现 %d 条未决议交易", len(trades))

	// 按配置组整理
	pendingByConfig := make(map[string][]*database.SimulationTrade)
	for _, trade := range trades {
		pendingByConfig[trade.ConfigGroup] = append(pendingByConfig[trade.ConfigGroup], trade)
	}

	// 检查每条记录的决议状态
	ctx := context.Background()
	for configGroup, configTrades := range pendingByConfig {
		// 找到对应的配置
		var config *StrategyConfig
		for i := range configs {
			if configs[i].ID == configGroup {
				config = &configs[i]
				break
			}
		}
		if config == nil {
			config = &configs[0] // 使用默认配置
		}

		for _, trade := range configTrades {
			// 使用 GetMarketBySlug 查询市场 (与 FKGoPolyTools 一致)
			market, err := client.GetMarketBySlug(ctx, trade.MarketSlug, nil)
			if err != nil {
				log.Printf("⏳ [%s] %s: 等待决议...", trade.ConfigGroup, trade.MarketSlug)

				// 添加到 pending 列表以便后续检查
				if _, ok := allPendingResolutions[trade.ConfigGroup]; !ok {
					allPendingResolutions[trade.ConfigGroup] = make(map[string]*MarketState)
				}
				allPendingResolutions[trade.ConfigGroup][trade.MarketSlug] = &MarketState{
					ConfigGroup:  trade.ConfigGroup,
					Slug:         trade.MarketSlug,
					Leg1Filled:   true,
					Leg1Side:     trade.Side,
					Leg1Price:    trade.Price,
					Leg1Amount:   trade.Amount,
					Leg1RecordID: trade.ID,
				}
				continue
			}

			if !market.Closed {
				log.Printf("⏳ [%s] %s: 市场尚未关闭", trade.ConfigGroup, trade.MarketSlug)

				// 添加到 pending 列表以便后续检查
				if _, ok := allPendingResolutions[trade.ConfigGroup]; !ok {
					allPendingResolutions[trade.ConfigGroup] = make(map[string]*MarketState)
				}
				allPendingResolutions[trade.ConfigGroup][trade.MarketSlug] = &MarketState{
					ConfigGroup:  trade.ConfigGroup,
					Slug:         trade.MarketSlug,
					Leg1Filled:   true,
					Leg1Side:     trade.Side,
					Leg1Price:    trade.Price,
					Leg1Amount:   trade.Amount,
					Leg1RecordID: trade.ID,
				}
				continue
			}

			// 通过 OutcomePrices 找到价格为 "1" 的索引 (获胜方向)
			// Index 0 = UP, Index 1 = DOWN
			prices := market.OutcomePrices
			winIndex := -1
			for i, pStr := range prices {
				p, _ := parsePrice(pStr)
				if p > 0.99 {
					winIndex = i
					break
				}
			}

			if winIndex == -1 {
				log.Printf("⏳ [%s] %s: 已关闭但未找到胜者 (Prices: %v), 稍后重试", trade.ConfigGroup, trade.MarketSlug, prices)

				// 添加到 pending 列表以便后续检查
				if _, ok := allPendingResolutions[trade.ConfigGroup]; !ok {
					allPendingResolutions[trade.ConfigGroup] = make(map[string]*MarketState)
				}
				allPendingResolutions[trade.ConfigGroup][trade.MarketSlug] = &MarketState{
					ConfigGroup:  trade.ConfigGroup,
					Slug:         trade.MarketSlug,
					Leg1Filled:   true,
					Leg1Side:     trade.Side,
					Leg1Price:    trade.Price,
					Leg1Amount:   trade.Amount,
					Leg1RecordID: trade.ID,
				}
				continue
			}

			// 判定胜负 (Index 0=UP, Index 1=DOWN)
			resolution := ""
			if winIndex == 0 {
				resolution = "UP"
			} else if winIndex == 1 {
				resolution = "DOWN"
			}

			if resolution == "" {
				// 添加到 pending 列表以便后续检查
				if _, ok := allPendingResolutions[trade.ConfigGroup]; !ok {
					allPendingResolutions[trade.ConfigGroup] = make(map[string]*MarketState)
				}
				allPendingResolutions[trade.ConfigGroup][trade.MarketSlug] = &MarketState{
					ConfigGroup:  trade.ConfigGroup,
					Slug:         trade.MarketSlug,
					Leg1Filled:   true,
					Leg1Side:     trade.Side,
					Leg1Price:    trade.Price,
					Leg1Amount:   trade.Amount,
					Leg1RecordID: trade.ID,
				}
				continue
			}

			// 计算 PnL
			var pnl float64
			if resolution == trade.Side {
				pnl = trade.Amount - (trade.Price * trade.Amount)
			} else {
				pnl = -(trade.Price * trade.Amount)
			}

			log.Printf("✅ [启动恢复][%s] %s: 决议=%s, 持仓=%s, PnL=$%.2f",
				trade.ConfigGroup, trade.MarketSlug, resolution, trade.Side, pnl)

			// 更新数据库
			if err := database.UpdateTradeResolution(trade.ID, resolution, pnl); err != nil {
				log.Printf("❌ 更新决议失败: %v", err)
			}
		}
	}
}
