package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"Polymarket15MinsCryptoArbitrageTool/src"
)

// Const Colors
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[1;31m"
	ColorGreen  = "\033[1;32m"
	ColorYellow = "\033[1;33m"
	ColorCyan   = "\033[1;36m"
)

// clearScreen 清除控制台屏幕
func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// getMACDDescription 获取 MACD 中文描述
func getMACDDescription(macd *src.MACDResult) string {
	if macd == nil {
		return "数据不足"
	}

	desc := ""
	color := ""

	if macd.Histogram > 0 && macd.HistDelta > 0 {
		desc = "看涨 (增强)"
		color = ColorGreen
	} else if macd.Histogram < 0 && macd.HistDelta < 0 {
		desc = "看跌 (增强)"
		color = ColorRed
	} else if macd.Histogram > 0 && macd.HistDelta < 0 {
		desc = "看涨 (减弱)"
		color = ColorGreen
	} else if macd.Histogram < 0 && macd.HistDelta > 0 {
		desc = "看跌 (减弱)"
		color = ColorRed
	} else {
		desc = "震荡"
		color = ColorYellow
	}

	// 添加金叉/死叉状态
	if macd.Histogram > 0 && macd.HistDelta > 0 && macd.Histogram < 5 {
		desc += " [金叉初期]"
	} else if macd.Histogram < 0 && macd.HistDelta < 0 && macd.Histogram > -5 {
		desc += " [死叉初期]"
	}

	return color + desc + ColorReset
}

// getTimeUntilClose 计算距离市场结束的时间
// 假设市场每15分钟结算一次，结算时间为 :00, :15, :30, :45
func getTimeUntilClose() string {
	now := time.Now()
	minute := now.Minute()
	second := now.Second()

	// 计算下一个结算点
	nextSettlement := (minute/15 + 1) * 15
	if nextSettlement >= 60 {
		nextSettlement = 0
	}

	// 计算剩余时间
	var remainingMinutes, remainingSeconds int
	if nextSettlement == 0 {
		remainingMinutes = 59 - minute
		remainingSeconds = 60 - second
		if remainingSeconds == 60 {
			remainingSeconds = 0
			remainingMinutes++
		}
	} else {
		remainingMinutes = nextSettlement - minute - 1
		remainingSeconds = 60 - second
		if remainingSeconds == 60 {
			remainingSeconds = 0
			remainingMinutes++
		}
	}

	return fmt.Sprintf("%02d:%02d", remainingMinutes, remainingSeconds)
}

func main() {
	// 使用 SilentLogger 避免日志干扰 TUI 显示
	logger := src.NewSilentLogger()

	// 初始化 Binance 客户端
	client := src.NewBinanceRESTClient(logger)

	fmt.Println("Starting Private Polymarket Arbitrage Dashboard...")
	fmt.Println("Initializing market data...")

	// 1. 获取初始价格，设定一个固定的模拟行权场景
	symbol := "BTCUSDT"
	initialPrice, err := client.FetchLastPrice(symbol)
	if err != nil {
		log.Fatalf("Init failed: %v", err)
	}

	// 设定场景：
	// 假设我们在一个 15 分钟的 Polymarket 市场
	// 行权价固定为初始价格 + 20 (稍微看涨的挑战)
	// 过期时间从 15 分钟开始倒计时
	strikePrice := initialPrice + 20.0
	expiryTime := time.Now().Add(15 * time.Minute)

	fmt.Printf("Scenario Initialized: Strike=%.2f, Expiry=%s\n", strikePrice, expiryTime.Format("15:04:05"))
	time.Sleep(1 * time.Second)

	// TODO: 集成 Polymarket 价格时，需要传入当前市场的 tokenID
	// 可以通过 realtime.NewRESTClient(logger).GetMidPrice(tokenID) 获取
	// 以下为占位符
	polymarketUp := 0.0   // TODO: 从 Polymarket API 获取
	polymarketDown := 0.0 // TODO: 从 Polymarket API 获取

	for {
		// 1. 获取 K 线数据
		klines1m, err := client.FetchKlines(symbol, "1m", 240)
		if err != nil {
			log.Printf("Error fetching data: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// 2. 计算指标
		closes := src.GetCloses(klines1m)
		if len(closes) < 50 {
			log.Println("Waiting for more data...")
			time.Sleep(1 * time.Second)
			continue
		}

		currentPrice := closes[len(closes)-1]

		// 计算剩余时间
		minutesLeft := time.Until(expiryTime).Minutes()
		// 如果时间为负，为了演示效果，重置场景
		if minutesLeft <= 0 {
			expiryTime = time.Now().Add(15 * time.Minute)
			strikePrice = currentPrice + 20.0
			minutesLeft = 15.0
		}

		// RSI
		rsi14 := src.ComputeRSI(closes, 14)
		rsiSlope := src.SlopeLast(src.ComputeRSISeries(closes, 14), 3)

		// MACD
		macd := src.ComputeMACD(closes, 12, 26, 9)

		// VWAP
		vwap := src.ComputeSessionVWAP(klines1m)
		vwapDist := (currentPrice - vwap) / vwap

		// Heiken Ashi
		ha := src.ComputeHeikenAshi(klines1m)
		haColor, haCount := src.CountConsecutive(ha)

		// 3. 形态检测
		prevRSI := 50.0
		if len(closes) > 15 {
			prevRSI = src.ComputeRSI(closes[:len(closes)-1], 14)
		}

		patternInput := src.PatternDetectInput{
			VWAPDist: vwapDist,
			MACD:     macd,
			RSI:      rsi14,
			RSISlope: rsiSlope,
			PrevRSI:  prevRSI,
		}
		pattern := src.DetectPattern(patternInput)

		// 4. 概率计算 (使用动态变化的 input)
		probInput := src.ProbabilityInput{
			Price:         currentPrice,
			PriceToBeat:   strikePrice, // 固定行权价
			MinutesLeft:   minutesLeft, // 倒计时
			VWAP:          vwap,
			VWAPSlope:     0, // 暂未计算
			RSI:           rsi14,
			RSISlope:      rsiSlope,
			MACD:          macd,
			HeikenColor:   haColor,
			HeikenCount:   haCount,
			FailedReclaim: false,
		}
		prob := src.ScoreDirection(probInput)

		// ========== 渲染界面 ==========
		clearScreen()
		fmt.Printf("==================================================\n")
		fmt.Printf("   🚀 POLYMARKET ARBITRAGE DASHBOARD (MAIN) 🚀   \n")
		fmt.Printf("==================================================\n")
		fmt.Printf("Time: %s\n", time.Now().Format("15:04:05"))
		fmt.Printf("Symbol: %s\n", symbol)
		fmt.Printf("Refreshed: %s\n", time.Now().Format("15:04:05"))
		fmt.Println()

		fmt.Printf("📊 Market Data\n")
		fmt.Printf("   Current Price:  \033[1m%.2f\033[0m\n", currentPrice)
		fmt.Printf("   Time Left:      %s\n", getTimeUntilClose())
		fmt.Println()

		fmt.Printf("📉 Technical Indicators (1m)\n")

		// RSI Color
		rsiColor := ColorReset
		if rsiSlope > 0 {
			rsiColor = ColorGreen
		} else if rsiSlope < 0 {
			rsiColor = ColorRed
		}
		fmt.Printf("   RSI (14):       %.2f  (Slope: %s%.4f%s)\n", rsi14, rsiColor, rsiSlope, ColorReset)

		// MACD - 对齐
		fmt.Printf("   MACD趋势:       %s\n", getMACDDescription(macd))

		// VWAP Color (Dist > 0 -> Bearish/Red, Dist < 0 -> Bullish/Green)
		vwapDistColor := ColorReset
		if vwapDist > 0 {
			vwapDistColor = ColorRed
		} else if vwapDist < 0 {
			vwapDistColor = ColorGreen
		}
		fmt.Printf("   VWAP:           %.2f  (Dist: %s%.4f%%%s)\n", vwap, vwapDistColor, vwapDist*100, ColorReset)

		// Heiken Ashi - 翻译 + 颜色
		haText := ""
		haTextColor := ColorReset
		if haColor == "green" {
			haText = "连涨"
			haTextColor = ColorGreen
		} else if haColor == "red" {
			haText = "连跌"
			haTextColor = ColorRed
		} else {
			haText = haColor
		}
		fmt.Printf("   Heiken Ashi:    %s%s x %d%s\n", haTextColor, haText, haCount, ColorReset)
		fmt.Println()

		fmt.Printf("🧩 Pattern Detection\n")

		// Action Color
		actionColor := ColorReset
		if strings.Contains(pattern.ActionSuggestion, "买涨") {
			actionColor = ColorGreen
		} else if strings.Contains(pattern.ActionSuggestion, "买跌") {
			actionColor = ColorRed
		}

		// Recommend Color
		recommendColor := ColorReset
		if strings.Contains(pattern.RecommendEntry, "推荐") {
			recommendColor = ColorYellow
		}

		fmt.Printf("   Result:         %s\n", pattern.PatternName)
		fmt.Printf("   Signal:         %s\n", pattern.SignalType)
		fmt.Printf("   Action:         %s%s%s\n", actionColor, pattern.ActionSuggestion, ColorReset)
		fmt.Printf("   Recommend:      %s%s%s\n", recommendColor, pattern.RecommendEntry, ColorReset)

		fmt.Println()

		// Probability Price (模型计算)
		fmt.Printf("🎲 Probability Price:\n")
		fmt.Printf("   涨：%s%.2f%s\n", ColorGreen, prob.RawUp, ColorReset)
		fmt.Printf("   跌：%s%.2f%s\n", ColorRed, prob.RawDown, ColorReset)
		fmt.Println()

		// Polymarket Price (实时市场价格)
		fmt.Printf("💰 Polymarket Price:\n")
		if polymarketUp > 0 || polymarketDown > 0 {
			fmt.Printf("   涨：%s%.2f%s\n", ColorGreen, polymarketUp, ColorReset)
			fmt.Printf("   跌：%s%.2f%s\n", ColorRed, polymarketDown, ColorReset)
		} else {
			fmt.Printf("   涨：%s--%s  (需要连接 Polymarket)\n", ColorGreen, ColorReset)
			fmt.Printf("   跌：%s--%s  (需要连接 Polymarket)\n", ColorRed, ColorReset)
		}
		fmt.Println()

		fmt.Printf("==================================================\n")
		fmt.Printf("Press Ctrl+C to exit. Refreshing in 1s...\n")

		// 刷新间隔改为 1 秒
		time.Sleep(1 * time.Second)
	}
}
