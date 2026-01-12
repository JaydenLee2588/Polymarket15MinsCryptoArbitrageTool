package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"
)

// getEnvFloat 从环境变量读取 float64，如果未设置则返回默认值
func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

// getEnvInt 从环境变量读取 int，如果未设置则返回默认值
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// CopyTradingHandler 处理自动跟单请求
func CopyTradingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 解析 type 参数 (只支持 type=2)
	typeStr := r.URL.Query().Get("type")
	if typeStr == "" {
		http.Error(w, `{"error": "Missing 'type' parameter"}`, http.StatusBadRequest)
		return
	}

	tradeType, err := strconv.Atoi(typeStr)
	if err != nil || tradeType != 2 {
		http.Error(w, `{"error": "Only type=2 is supported"}`, http.StatusBadRequest)
		return
	}

	// 构建 Type2 配置
	baseConfig := Type2Config{
		ID:                   "default",
		MinTrendPercentUp:    getEnvFloat("TYPE2_MIN_TREND_PERCENT_UP", 0.0025),
		MinTrendPercentDown:  getEnvFloat("TYPE2_MIN_TREND_PERCENT_DOWN", 0.0020),
		MinPriceEdge:         getEnvFloat("TYPE2_MIN_PRICE_EDGE", 0.12),
		FairPriceTrendFactor: getEnvFloat("TYPE2_FAIR_PRICE_TREND_FACTOR", 0.8),
		TrendWindowSecStart:  getEnvInt("TYPE2_TREND_WINDOW_SEC_START", 180),
		TrendWindowSecEnd:    getEnvInt("TYPE2_TREND_WINDOW_SEC_END", 600),
		Leg0SharePrice0:      getEnvFloat("TYPE2_LEG0_SHARE_PRICE_0", 0.05),
		SharePriceLevel1:     getEnvFloat("TYPE2_SHARE_PRICE_1", 0.20),
		SharePriceLevel2:     getEnvFloat("TYPE2_SHARE_PRICE_2", 0.50),
		SharePriceLevel3:     getEnvFloat("TYPE2_SHARE_PRICE_3", 0.85),
		ShareAmount1:         getEnvFloat("TYPE2_SHARE_AMOUNT_1", 50),
		ShareAmount2:         getEnvFloat("TYPE2_SHARE_AMOUNT_2", 30),
		ShareAmount3:         getEnvFloat("TYPE2_SHARE_AMOUNT_3", 10),
		FeeRate:              getEnvFloat("TYPE2_FEE_RATE", 0.005),
		ScanIntervalSec:      getEnvInt("TYPE2_SCAN_INTERVAL", 5),
	}

	// 获取配置列表
	configs := GetOrLoadType2Configs(baseConfig)
	configCount := len(configs)

	var message string
	started := StartType2MultiConfigSimulation(configs)
	if started {
		if configCount > 1 {
			message = "Started Type2 Multi-Config Simulation (Background)"
		} else {
			message = "Started Type2 Simulation (Background)"
		}
	} else {
		message = "Type2 Simulation is already running"
	}

	// 返回 JSON 响应
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "success",
		"message":      message,
		"type":         typeStr,
		"config_count": configCount,
		"timestamp":    time.Now(),
	})
}
