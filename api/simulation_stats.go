package api

import (
	"encoding/json"
	"net/http"

	"Polymarket15MinsCryptoArbitrageTool/internal/database"
)

// SimulationStats 模拟交易统计
type SimulationStats struct {
	StrategyType string  `json:"strategy_type"`
	TradeCount   int     `json:"trade_count"`
	TotalPnL     float64 `json:"total_pnl"`
	WinCount     int     `json:"win_count"`
	WinRate      float64 `json:"win_rate"`
}

// ConfigStats 配置组统计
type ConfigStats struct {
	ConfigGroup string  `json:"config_group"`
	TradeCount  int     `json:"trade_count"`
	TotalPnL    float64 `json:"total_pnl"`
	WinCount    int     `json:"win_count"`
	WinRate     float64 `json:"win_rate"`
}

// SimulationStatsResponse API 响应
type SimulationStatsResponse struct {
	Success         bool                       `json:"success"`
	Data            map[string]SimulationStats `json:"data"`
	Summary         SimulationStats            `json:"summary"`
	ConfigBreakdown map[string]ConfigStats     `json:"config_breakdown"`
}

// SimulationStatsHandler 模拟跟单收益统计 API
func SimulationStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 获取类型参数
	strategyType := r.URL.Query().Get("type")
	if strategyType == "2" {
		strategyType = "type2"
	}

	// 获取总体统计
	tradeCount, totalPnL, winCount, err := database.GetTradeStats(strategyType)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	winRate := 0.0
	if tradeCount > 0 {
		winRate = float64(winCount) / float64(tradeCount)
	}

	summary := SimulationStats{
		StrategyType: "type2",
		TradeCount:   tradeCount,
		TotalPnL:     totalPnL,
		WinCount:     winCount,
		WinRate:      winRate,
	}

	// 获取配置组统计
	configStats, err := database.GetTradeStatsByConfig()
	configBreakdown := make(map[string]ConfigStats)
	if err == nil {
		for configGroup, stats := range configStats {
			configWinRate := 0.0
			if stats.TradeCount > 0 {
				configWinRate = float64(stats.WinCount) / float64(stats.TradeCount)
			}
			configBreakdown[configGroup] = ConfigStats{
				ConfigGroup: configGroup,
				TradeCount:  stats.TradeCount,
				TotalPnL:    stats.TotalPnL,
				WinCount:    stats.WinCount,
				WinRate:     configWinRate,
			}
		}
	}

	response := SimulationStatsResponse{
		Success:         true,
		Data:            map[string]SimulationStats{"2": summary},
		Summary:         summary,
		ConfigBreakdown: configBreakdown,
	}

	json.NewEncoder(w).Encode(response)
}
