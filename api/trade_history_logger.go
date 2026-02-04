package api

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ==================== 交易历史日志 ====================

// UTC-5 时区 (EST)
var estLocation = time.FixedZone("EST", -5*60*60)

// TradeLogEntry 交易日志条目
type TradeLogEntry struct {
	// 基本信息
	BuyTime    time.Time // 买入时 UTC-5 时间
	ConfigID   string    // 策略名
	MarketSlug string    // 市场名

	// 买入信息
	MarketStartTime time.Time // 市场开始时间 (计算 MinutesIn)
	MinutesIn       float64   // 开局后几分钟
	Side            string    // UP/DOWN
	Amount          float64   // 数量
	BuyPrice        float64   // 买入价格
	TotalCost       float64   // 总价

	// 指标快照
	VWAPDist      float64
	MACDLine      float64
	MACDSignal    float64
	MACDHistogram float64
	MACDHistDelta float64
	RSI           float64
	RSISlope      float64

	// 卖出信息 (后续更新)
	SellTime      time.Time
	SellMinutesIn float64
	SellReason    string // "止盈", "时间止损", "决议赎回"
	SellAmount    float64
	SellPrice     float64
	Revenue       float64 // 到手金额
	PnL           float64 // 总 PnL

	// 决议信息
	Resolution string // "UP" / "DOWN"

	// 状态
	BuyFailed bool   // 买入失败标记
	Written   bool   // 是否已写入完整日志
	FailMsg   string // 失败原因
}

// 全局日志状态
var (
	tradeLogFile     *os.File
	tradeLogFilePath string
	tradeLogMu       sync.Mutex
	tradeLogEntries  map[string]*TradeLogEntry // key: marketSlug + "-" + configID
	tradeLogStarted  bool
)

// InitTradeHistoryLogger 初始化交易历史日志
// 应在服务启动时调用
func InitTradeHistoryLogger() error {
	tradeLogMu.Lock()
	defer tradeLogMu.Unlock()

	if tradeLogStarted {
		return nil // 已初始化
	}

	now := time.Now().In(estLocation)

	// 创建日志目录 logs/YYYYMMDD/
	dateDir := now.Format("20060102")
	logDir := filepath.Join("logs", dateDir)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志文件 trade_history_HHmm.log
	timeStr := now.Format("1504")
	filename := fmt.Sprintf("trade_history_%s.log", timeStr)
	tradeLogFilePath = filepath.Join(logDir, filename)

	f, err := os.OpenFile(tradeLogFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}

	tradeLogFile = f
	tradeLogEntries = make(map[string]*TradeLogEntry)
	tradeLogStarted = true

	log.Printf("📝 [TradeHistory] 日志文件已创建: %s", tradeLogFilePath)
	return nil
}

// CloseTradeHistoryLogger 关闭日志文件
func CloseTradeHistoryLogger() {
	tradeLogMu.Lock()
	defer tradeLogMu.Unlock()

	if tradeLogFile != nil {
		tradeLogFile.Close()
		tradeLogFile = nil
	}
	tradeLogStarted = false
}

// getEntryKey 生成日志条目的唯一键
func getEntryKey(marketSlug, configID string) string {
	return marketSlug + "-" + configID
}

// LogBuyEntry 记录买入事件
func LogBuyEntry(entry *TradeLogEntry) {
	tradeLogMu.Lock()
	defer tradeLogMu.Unlock()

	if !tradeLogStarted || tradeLogFile == nil {
		return
	}

	// 设置买入时间 (确保是 EST 时区)
	if entry.BuyTime.IsZero() {
		entry.BuyTime = time.Now().In(estLocation)
	} else {
		entry.BuyTime = entry.BuyTime.In(estLocation)
	}

	// 计算开局后几分钟
	if !entry.MarketStartTime.IsZero() {
		entry.MinutesIn = time.Since(entry.MarketStartTime).Minutes()
	}

	// 存储条目 (等待后续更新)
	key := getEntryKey(entry.MarketSlug, entry.ConfigID)
	tradeLogEntries[key] = entry

	log.Printf("📝 [TradeHistory] 记录买入: %s | %s | %s | %.4f", entry.ConfigID, entry.MarketSlug, entry.Side, entry.BuyPrice)
}

// LogBuyFailure 记录买入失败
func LogBuyFailure(entry *TradeLogEntry, failMsg string) {
	tradeLogMu.Lock()
	defer tradeLogMu.Unlock()

	if !tradeLogStarted || tradeLogFile == nil {
		return
	}

	// 设置买入时间 (确保是 EST 时区)
	if entry.BuyTime.IsZero() {
		entry.BuyTime = time.Now().In(estLocation)
	} else {
		entry.BuyTime = entry.BuyTime.In(estLocation)
	}

	// 计算开局后几分钟
	if !entry.MarketStartTime.IsZero() {
		entry.MinutesIn = time.Since(entry.MarketStartTime).Minutes()
	}

	entry.BuyFailed = true
	entry.FailMsg = failMsg

	// 写入日志 (买入失败立即写入)
	line := formatBuyFailureLine(entry)
	writeLogLine(line)

	log.Printf("📝 [TradeHistory] 记录买入失败: %s | %s | %s", entry.ConfigID, entry.MarketSlug, failMsg)
}

// UpdateSellEntry 更新卖出信息
func UpdateSellEntry(marketSlug, configID string, sellReason string, sellPrice, sellAmount, revenue, pnl float64) {
	tradeLogMu.Lock()
	defer tradeLogMu.Unlock()

	if !tradeLogStarted {
		return
	}

	key := getEntryKey(marketSlug, configID)
	entry, exists := tradeLogEntries[key]
	if !exists {
		log.Printf("⚠️ [TradeHistory] 找不到买入记录: %s", key)
		return
	}

	entry.SellTime = time.Now().In(estLocation)
	if !entry.MarketStartTime.IsZero() {
		entry.SellMinutesIn = time.Since(entry.MarketStartTime).Minutes()
	}
	entry.SellReason = sellReason
	entry.SellPrice = sellPrice
	entry.SellAmount = sellAmount
	entry.Revenue = revenue
	entry.PnL = pnl

	log.Printf("📝 [TradeHistory] 更新卖出: %s | %s | %s | PnL=$%.2f", configID, marketSlug, sellReason, pnl)

	// 如果已有决议，写入完整日志
	if entry.Resolution != "" && !entry.Written {
		line := formatCompleteLine(entry)
		writeLogLine(line)
		entry.Written = true
		delete(tradeLogEntries, key) // 清除已完成条目
	}
}

// UpdateResolution 更新决议结果
func UpdateResolution(marketSlug, configID string, resolution string) {
	tradeLogMu.Lock()
	defer tradeLogMu.Unlock()

	if !tradeLogStarted {
		return
	}

	key := getEntryKey(marketSlug, configID)
	entry, exists := tradeLogEntries[key]
	if !exists {
		log.Printf("⚠️ [TradeHistory] 找不到买入记录: %s", key)
		return
	}

	entry.Resolution = resolution

	log.Printf("📝 [TradeHistory] 更新决议: %s | %s | %s", configID, marketSlug, resolution)

	// 如果已有卖出信息，写入完整日志
	if entry.SellReason != "" && !entry.Written {
		line := formatCompleteLine(entry)
		writeLogLine(line)
		entry.Written = true
		delete(tradeLogEntries, key) // 清除已完成条目
	}
}

// UpdateResolutionForRedemption 更新决议赎回信息
func UpdateResolutionForRedemption(marketSlug, configID string, resolution string, pnl float64) {
	tradeLogMu.Lock()
	defer tradeLogMu.Unlock()

	if !tradeLogStarted {
		return
	}

	key := getEntryKey(marketSlug, configID)
	entry, exists := tradeLogEntries[key]
	if !exists {
		log.Printf("⚠️ [TradeHistory] 找不到买入记录: %s", key)
		return
	}

	// 决议赎回模式
	entry.Resolution = resolution
	entry.SellTime = time.Now().In(estLocation)
	if !entry.MarketStartTime.IsZero() {
		entry.SellMinutesIn = time.Since(entry.MarketStartTime).Minutes()
	}
	entry.SellReason = "决议赎回"
	entry.PnL = pnl
	entry.SellAmount = entry.Amount // 决议赎回时，数量等于买入数量

	// 如果获胜，收入 = 数量，卖出价格 = 1.0；如果失败，收入 = 0，卖出价格 = 0.0
	if resolution == entry.Side {
		entry.Revenue = entry.Amount // 获胜时收回的是股数 (每股 $1)
		entry.SellPrice = 1.0
	} else {
		entry.Revenue = 0
		entry.SellPrice = 0.0
	}

	// 写入完整日志
	if !entry.Written {
		line := formatCompleteLine(entry)
		writeLogLine(line)
		entry.Written = true
		delete(tradeLogEntries, key)
	}

	log.Printf("📝 [TradeHistory] 记录决议赎回: %s | %s | %s | PnL=$%.2f", configID, marketSlug, resolution, pnl)
}

// ==================== 日志格式化 ====================

// indicatorStrings 指标格式化结果
type indicatorStrings struct {
	VWAP     string
	MACD     string
	RSI      string
	RSISlope string
}

// formatIndicators 格式化指标字符串 (公共函数避免重复代码)
func formatIndicators(entry *TradeLogEntry) indicatorStrings {
	// VWAP 格式
	vwapDir := "+"
	if entry.VWAPDist < 0 {
		vwapDir = ""
	}
	vwapStr := fmt.Sprintf("VWAP %s%.2f%%", vwapDir, entry.VWAPDist*100)

	// MACD 格式
	macdStatus := "中性"
	if entry.MACDHistogram > 0 && entry.MACDHistDelta > 0 {
		macdStatus = "金叉"
	} else if entry.MACDHistogram < 0 && entry.MACDHistDelta < 0 {
		macdStatus = "死叉"
	} else if entry.MACDHistogram > 0 {
		macdStatus = "红柱"
	} else if entry.MACDHistogram < 0 {
		macdStatus = "绿柱"
	}
	macdStr := fmt.Sprintf("MACD %s H=%.2f Δ=%.2f", macdStatus, entry.MACDHistogram, entry.MACDHistDelta)

	// RSI 格式
	rsiStr := fmt.Sprintf("RSI %.1f", entry.RSI)

	// RSI Slope 格式
	rsiSlopeDir := "→"
	if entry.RSISlope > 0.2 {
		rsiSlopeDir = "↑"
	} else if entry.RSISlope < -0.2 {
		rsiSlopeDir = "↓"
	}
	rsiSlopeStr := fmt.Sprintf("RSI%s %.2f", rsiSlopeDir, entry.RSISlope)

	return indicatorStrings{
		VWAP:     vwapStr,
		MACD:     macdStr,
		RSI:      rsiStr,
		RSISlope: rsiSlopeStr,
	}
}

// formatCompleteLine 格式化完整的日志行
func formatCompleteLine(entry *TradeLogEntry) string {
	// 时间格式
	buyTimeStr := entry.BuyTime.Format("15:04:05")

	// 使用公共函数格式化指标
	ind := formatIndicators(entry)

	// 构建日志行
	line := fmt.Sprintf("【%s】【%s】【%s】\"【%.1f分】【买入】【%s】【%.0f】【%.4f】【$%.2f】\"\"【%s】【%s】【%s】【%s】\"\"【%.1f分】【%s】【%.0f】【%.4f】【$%.2f】【$%.2f】\" \"【%s】\"",
		buyTimeStr,
		entry.ConfigID,
		entry.MarketSlug,
		entry.MinutesIn,
		entry.Side,
		entry.Amount,
		entry.BuyPrice,
		entry.TotalCost,
		ind.VWAP,
		ind.MACD,
		ind.RSI,
		ind.RSISlope,
		entry.SellMinutesIn,
		entry.SellReason,
		entry.SellAmount,
		entry.SellPrice,
		entry.Revenue,
		entry.PnL,
		entry.Resolution,
	)

	return line
}

// formatBuyFailureLine 格式化买入失败的日志行
func formatBuyFailureLine(entry *TradeLogEntry) string {
	// 时间格式
	buyTimeStr := entry.BuyTime.Format("15:04:05")

	// 使用公共函数格式化指标
	ind := formatIndicators(entry)

	// 构建日志行 (买入失败格式)
	line := fmt.Sprintf("【%s】【%s】【%s】\"【%.1f分】【买入】【%s】【%.0f】【%.4f】【$%.2f】\"\"【%s】【%s】【%s】【%s】\" \"【待定】 - 买入失败: %s\"",
		buyTimeStr,
		entry.ConfigID,
		entry.MarketSlug,
		entry.MinutesIn,
		entry.Side,
		entry.Amount,
		entry.BuyPrice,
		entry.TotalCost,
		ind.VWAP,
		ind.MACD,
		ind.RSI,
		ind.RSISlope,
		entry.FailMsg,
	)

	return line
}

// writeLogLine 写入日志行
func writeLogLine(line string) {
	if tradeLogFile == nil {
		return
	}

	_, err := tradeLogFile.WriteString(line + "\n")
	if err != nil {
		log.Printf("❌ [TradeHistory] 写入日志失败: %v", err)
	}

	// 立即刷新
	tradeLogFile.Sync()
}
