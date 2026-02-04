package src

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ==================== 模拟交易器 ====================

// SimPosition 模拟持仓信息
type SimPosition struct {
	Side       string    // "UP" or "DOWN"
	EntryPrice float64   // 入场价格
	EntryTime  time.Time // 入场时间
	RoundSlug  string    // 回合标识
	Pattern    string    // 入场形态
	Edge       float64   // 入场时的 Edge
}

// EntrySignal 入场信号
type EntrySignal struct {
	Side        string  // "UP" or "DOWN"
	MarketPrice float64 // 市场价格
	Edge        float64 // Edge 值
	Pattern     string  // 形态名称
}

// ExitSignal 出场信号
type ExitSignal struct {
	Reason    string  // 出场原因 ("TAKE_PROFIT", "STOP_LOSS", "SETTLEMENT")
	ExitPrice float64 // 出场价格
	Profit    float64 // 盈亏
}

// SimSettlementResult 结算结果
type SimSettlementResult struct {
	Position SimPosition
	Profit   float64
}

// Simulator 交易模拟器
type Simulator struct {
	positions   []SimPosition
	mu          sync.RWMutex
	logFilePath string
	logFile     *os.File
	logger      Logger

	// 已结算回合 (防止重复结算)
	settledRounds sync.Map // map[string]bool

	// 当前回合已入场方向 (防止同方向重复入场)
	enteredDirections sync.Map // map[string]map[string]bool (roundSlug -> side -> true)

	// 当前回合信息
	currentRound struct {
		Slug      string
		StartTime time.Time
	}
}

// NewSimulator 创建模拟器
func NewSimulator(logger Logger) *Simulator {
	if logger == nil {
		logger = NewSilentLogger()
	}

	s := &Simulator{
		positions: make([]SimPosition, 0),
		logger:    logger,
	}

	s.initLogFile()
	return s
}

// initLogFile 初始化日志文件
func (s *Simulator) initLogFile() {
	now := time.Now()
	dateStr := now.Format("20060102")
	timeStr := now.Format("1504")
	filename := fmt.Sprintf("trades_%s_%s.log", dateStr, timeStr)

	logsDir := filepath.Join(".", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		s.logger.Error("创建日志目录失败: %v", err)
		return
	}

	s.logFilePath = filepath.Join(logsDir, filename)

	file, err := os.OpenFile(s.logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		s.logger.Error("创建日志文件失败: %v", err)
		return
	}
	s.logFile = file

	s.logger.Info("[Simulator] 日志文件: %s", s.logFilePath)
}

// writeLog 写入日志
func (s *Simulator) writeLog(message string) {
	timestamp := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("[%s] %s\n", timestamp, message)

	if s.logFile != nil {
		s.logFile.WriteString(line)
	}
	s.logger.Info("[Trade] %s", message)
}

// writeRoundSeparator 写入回合分隔线
func (s *Simulator) writeRoundSeparator() {
	if s.logFile != nil {
		s.logFile.WriteString("\n" + "--------------------------------------------------------------------------------" + "\n\n")
	}
}

// CheckEntry 检查是否应该入场
func (s *Simulator) CheckEntry(pattern *PatternResult, edgeUp, edgeDown, marketUp, marketDown, minutesLeft, windowMinutes float64, roundSlug string) *EntrySignal {
	// 条件1: 必须是"强烈推荐"
	if pattern == nil || !pattern.IsStrongRecommend() {
		return nil
	}

	// 条件2: 必须在前10分钟内
	elapsedMinutes := windowMinutes - minutesLeft
	if elapsedMinutes > 10 {
		return nil
	}

	// 判断方向
	var side string
	if pattern.IsBuyUp() {
		side = "UP"
	} else if pattern.IsBuyDown() {
		side = "DOWN"
	} else {
		return nil
	}

	// 条件3: 检查是否已在本回合同方向入场过
	if roundSlug != "" {
		if dirs, ok := s.enteredDirections.Load(roundSlug); ok {
			if dirMap, ok := dirs.(map[string]bool); ok {
				if dirMap[side] {
					return nil // 已入场过
				}
			}
		}
	}

	// 获取对应方向的价格和edge
	var marketPrice, edge float64
	if side == "UP" {
		marketPrice = marketUp
		edge = edgeUp
	} else {
		marketPrice = marketDown
		edge = edgeDown
	}

	s.logger.Info("[Simulator] 强形态: %s, 方向: %s, Edge: %.3f, 价格: %.3f",
		pattern.PatternName, side, edge, marketPrice)

	// 条件4: 市场价格 < 0.7
	if marketPrice >= 0.7 {
		s.logger.Info("[Simulator] 市场价格过高: %.3f >= 0.7", marketPrice)
		return nil
	}

	return &EntrySignal{
		Side:        side,
		MarketPrice: marketPrice,
		Edge:        edge,
		Pattern:     pattern.PatternName,
	}
}

// ExecuteBuy 执行买入
func (s *Simulator) ExecuteBuy(signal *EntrySignal, roundSlug string) *SimPosition {
	if signal == nil {
		return nil
	}

	pos := SimPosition{
		Side:       signal.Side,
		EntryPrice: signal.MarketPrice,
		EntryTime:  time.Now(),
		RoundSlug:  roundSlug,
		Pattern:    signal.Pattern,
		Edge:       signal.Edge,
	}

	s.mu.Lock()
	s.positions = append(s.positions, pos)
	s.mu.Unlock()

	// 记录已入场方向
	if roundSlug != "" {
		var dirMap map[string]bool
		if dirs, ok := s.enteredDirections.Load(roundSlug); ok {
			dirMap = dirs.(map[string]bool)
		} else {
			dirMap = make(map[string]bool)
		}
		dirMap[signal.Side] = true
		s.enteredDirections.Store(roundSlug, dirMap)
	}

	dirText := "涨"
	if signal.Side == "DOWN" {
		dirText = "跌"
	}

	s.writeLog(fmt.Sprintf("买入 | 方向: %s | 价格: %.3f | Edge: %.3f | 形态: %s",
		dirText, signal.MarketPrice, signal.Edge, signal.Pattern))

	return &pos
}

// CheckExit 检查是否应该出场 (止盈)
func (s *Simulator) CheckExit(pos *SimPosition, currentMarketUp, currentMarketDown float64) *ExitSignal {
	if pos == nil {
		return nil
	}

	var currentPrice float64
	if pos.Side == "UP" {
		currentPrice = currentMarketUp
	} else {
		currentPrice = currentMarketDown
	}

	if currentPrice == 0 {
		return nil
	}

	// 止盈: 价差达到 0.15
	priceDiff := currentPrice - pos.EntryPrice

	if priceDiff >= 0.15 {
		return &ExitSignal{
			Reason:    "TAKE_PROFIT",
			ExitPrice: currentPrice,
			Profit:    priceDiff,
		}
	}

	return nil
}

// ExecuteSell 执行卖出
func (s *Simulator) ExecuteSell(pos *SimPosition, signal *ExitSignal) float64 {
	if pos == nil || signal == nil {
		return 0
	}

	profit := signal.ExitPrice - pos.EntryPrice

	dirText := "涨"
	if pos.Side == "DOWN" {
		dirText = "跌"
	}

	s.writeLog(fmt.Sprintf("卖出 | 方向: %s | 买入价: %.3f | 卖出价: %.3f | 盈亏: %+.3f | 原因: %s",
		dirText, pos.EntryPrice, signal.ExitPrice, profit, signal.Reason))

	// 从持仓中移除
	s.mu.Lock()
	newPositions := make([]SimPosition, 0, len(s.positions))
	for _, p := range s.positions {
		if !(p.Side == pos.Side && p.EntryPrice == pos.EntryPrice && p.RoundSlug == pos.RoundSlug) {
			newPositions = append(newPositions, p)
		}
	}
	s.positions = newPositions
	s.mu.Unlock()

	return profit
}

// HandleSettlement 处理回合结算
func (s *Simulator) HandleSettlement(roundSlug, settlementResult string, marketUp, marketDown float64) []SimSettlementResult {
	// 防止重复结算
	if _, loaded := s.settledRounds.LoadOrStore(roundSlug, true); loaded {
		return nil
	}

	s.mu.Lock()
	settledPositions := make([]SimPosition, 0)
	remainingPositions := make([]SimPosition, 0)

	for _, p := range s.positions {
		if p.RoundSlug == roundSlug {
			settledPositions = append(settledPositions, p)
		} else {
			remainingPositions = append(remainingPositions, p)
		}
	}
	s.positions = remainingPositions
	s.mu.Unlock()

	if len(settledPositions) == 0 {
		s.writeRoundSeparator()
		s.enteredDirections.Delete(roundSlug)
		return nil
	}

	results := make([]SimSettlementResult, 0, len(settledPositions))

	for _, pos := range settledPositions {
		var profit float64
		if pos.Side == settlementResult {
			// 获胜: 获得 1 - 入场价格
			profit = 1 - pos.EntryPrice
		} else {
			// 失败: 损失入场价格
			profit = -pos.EntryPrice
		}

		dirText := "涨"
		if pos.Side == "DOWN" {
			dirText = "跌"
		}
		resultText := "涨"
		if settlementResult == "DOWN" {
			resultText = "跌"
		}

		s.writeLog(fmt.Sprintf("决议 | 持仓方向: %s | 买入价: %.3f | 决议结果: %s | 盈亏: %+.3f",
			dirText, pos.EntryPrice, resultText, profit))

		results = append(results, SimSettlementResult{Position: pos, Profit: profit})
	}

	s.enteredDirections.Delete(roundSlug)
	s.writeRoundSeparator()

	return results
}

// OnNewRound 检测新回合开始
func (s *Simulator) OnNewRound(slug string) {
	if s.currentRound.Slug != "" && s.currentRound.Slug != slug {
		s.writeRoundSeparator()
	}

	s.currentRound.Slug = slug
	s.currentRound.StartTime = time.Now()

	s.writeLog(fmt.Sprintf("新回合开始: %s", slug))
}

// GetPositions 获取当前持仓
func (s *Simulator) GetPositions() []SimPosition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]SimPosition, len(s.positions))
	copy(result, s.positions)
	return result
}

// Close 关闭模拟器
func (s *Simulator) Close() {
	if s.logFile != nil {
		s.logFile.Close()
	}
}
