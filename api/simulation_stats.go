package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"Polymarket15MinsCryptoArbitrageTool/internal/database"
)

// SimulationStats 模拟交易统计
type SimulationStats struct {
	TradeCount int     `json:"trade_count"`
	TotalPnL   float64 `json:"total_pnl"`
	WinCount   int     `json:"win_count"`
	WinRate    float64 `json:"win_rate"`
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
	Uptime          string                     `json:"uptime,omitempty"` // 已启动时间
}

// SimulationStatsHandler 模拟跟单收益统计 API
func SimulationStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 获取总体统计
	tradeCount, totalPnL, winCount, err := database.GetTradeStats()
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
		TradeCount: tradeCount,
		TotalPnL:   totalPnL,
		WinCount:   winCount,
		WinRate:    winRate,
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

	// 获取最早交易时间并计算已启动时间
	var uptimeStr string
	if earliest, err := database.GetEarliestTradeTime(); err == nil && !earliest.IsZero() {
		duration := time.Since(earliest)
		hours := int(duration.Hours())
		minutes := int(duration.Minutes()) % 60
		if hours > 0 {
			uptimeStr = fmt.Sprintf("%d小时%d分钟", hours, minutes)
		} else {
			uptimeStr = fmt.Sprintf("%d分钟", minutes)
		}
	}

	response := SimulationStatsResponse{
		Success:         true,
		Data:            map[string]SimulationStats{"2": summary},
		Summary:         summary,
		ConfigBreakdown: configBreakdown,
		Uptime:          uptimeStr,
	}

	json.NewEncoder(w).Encode(response)
}
