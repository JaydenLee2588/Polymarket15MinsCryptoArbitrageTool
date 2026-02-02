package api

import (
	"database/sql"
	"net/http"
)

var db *sql.DB

// SetDatabase 设置数据库引用
func SetDatabase(database *sql.DB) {
	db = database
}

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(mux *http.ServeMux) {
	// 模拟跟单统计接口 (无需操作码)
	mux.HandleFunc("/api/v1/sim-stats", SimulationStatsHandler)

	// 模拟跟单启动接口 (需要操作码)
	mux.HandleFunc("/api/v1/sim-copy-trading", RequireOperationCode(CopyTradingHandler))

	// 模拟跟单停止接口 (需要操作码)
	mux.HandleFunc("/api/v1/sim-stop-trading", RequireOperationCode(StopCopyTradingHandler))

	// 数据管理接口 - 清空交易记录
	mux.HandleFunc("/api/v1/trades", DeleteTradesHandler)

	// 配置管理接口
	mux.HandleFunc("/api/v1/config", ConfigHandler)              // GET (无需操作码) / PUT (需要操作码)
	mux.HandleFunc("/api/v1/config/reload", ReloadConfigHandler) // POST (需要操作码)
}
