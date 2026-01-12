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
	// 模拟跟单统计接口
	mux.HandleFunc("/api/v1/sim-stats", SimulationStatsHandler)

	// 模拟跟单启动接口
	mux.HandleFunc("/api/v1/sim-copy-trading", CopyTradingHandler)

	// 数据管理接口 - 清空交易记录
	mux.HandleFunc("/api/v1/trades", DeleteTradesHandler)
}
