package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// ConfigResponse 配置响应结构
type ConfigResponse struct {
	Success    bool             `json:"success"`
	Message    string           `json:"message,omitempty"`
	ConfigFile string           `json:"config_file,omitempty"`
	Groups     []ConfigGroupDTO `json:"groups,omitempty"`
	Timestamp  time.Time        `json:"timestamp"`
}

// ConfigGroupDTO 配置组 DTO (用于 API 传输)
type ConfigGroupDTO struct {
	ID                  string  `json:"id"`
	MinTrendPercentUp   float64 `json:"min_trend_percent_up"`
	MinTrendPercentDown float64 `json:"min_trend_percent_down"`
	TrendWindowSecStart int     `json:"trend_window_sec_start"`
	TrendWindowSecEnd   int     `json:"trend_window_sec_end"`
	Leg0SharePrice0     float64 `json:"leg0_share_price_0"`
	Leg1SharePrice1     float64 `json:"leg1_share_price_1"`
	Leg1SharePrice2     float64 `json:"leg1_share_price_2"`
	Leg1SharePrice3     float64 `json:"leg1_share_price_3"`
	Leg1ShareAmount1    float64 `json:"leg1_share_amount_1"`
	Leg1ShareAmount2    float64 `json:"leg1_share_amount_2"`
	Leg1ShareAmount3    float64 `json:"leg1_share_amount_3"`
	MaxSlippage         float64 `json:"max_slippage"`
	PriceTickSize       float64 `json:"price_tick_size"`
	SigmoidSensitivity  float64 `json:"sigmoid_sensitivity"`
	VolatilityBTC       float64 `json:"volatility_btc"`
	VolatilityETH       float64 `json:"volatility_eth"`
	VolatilitySOL       float64 `json:"volatility_sol"`
	VolatilityXRP       float64 `json:"volatility_xrp"`
	Tier0ZScore         float64 `json:"tier_0_z_score"`
	Tier1ZScore         float64 `json:"tier_1_z_score"`
	Tier2ZScore         float64 `json:"tier_2_z_score"`
	Tier3ZScore         float64 `json:"tier_3_z_score"`
	Tier4ZScore         float64 `json:"tier_4_z_score"`
	MinEdgeTier0        float64 `json:"min_edge_tier_0"`
	MinEdgeTier1        float64 `json:"min_edge_tier_1"`
	MinEdgeTier2        float64 `json:"min_edge_tier_2"`
	MinEdgeTier3        float64 `json:"min_edge_tier_3"`
	MinEdgeTier4        float64 `json:"min_edge_tier_4"`
}

// ConfigHandler 处理配置的获取和更新 (GET/PUT)
func ConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		handleGetConfig(w, r)
	case http.MethodPut:
		// PUT 需要操作码验证
		if !checkOpCode(r) {
			sendAuthError(w)
			return
		}
		handlePutConfig(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ConfigResponse{
			Success:   false,
			Message:   "Method not allowed. Use GET or PUT.",
			Timestamp: time.Now(),
		})
	}
}

// ReloadConfigHandler 处理配置重新加载 (POST)
func ReloadConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ConfigResponse{
			Success:   false,
			Message:   "Method not allowed. Use POST.",
			Timestamp: time.Now(),
		})
		return
	}

	// 需要操作码验证
	if !checkOpCode(r) {
		sendAuthError(w)
		return
	}

	configs, err := ReloadConfigs()
	if err != nil {
		log.Printf("❌ 配置重载失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ConfigResponse{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	groups := convertToDTO(configs)
	json.NewEncoder(w).Encode(ConfigResponse{
		Success:    true,
		Message:    "配置已重新加载",
		ConfigFile: GetConfigFilePath(),
		Groups:     groups,
		Timestamp:  time.Now(),
	})
}

// handleGetConfig 获取当前配置
func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	configs := GetCurrentConfigs()
	groups := convertToDTO(configs)

	json.NewEncoder(w).Encode(ConfigResponse{
		Success:    true,
		ConfigFile: GetConfigFilePath(),
		Groups:     groups,
		Timestamp:  time.Now(),
	})
}

// handlePutConfig 更新配置
func handlePutConfig(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Groups []ConfigGroupDTO `json:"groups"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ConfigResponse{
			Success:   false,
			Message:   "无效的 JSON 格式: " + err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	if len(request.Groups) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ConfigResponse{
			Success:   false,
			Message:   "配置组不能为空",
			Timestamp: time.Now(),
		})
		return
	}

	// 转换为内部配置格式
	configs := convertFromDTO(request.Groups)

	// 保存到文件
	if err := SaveConfigsToYAML(configs); err != nil {
		log.Printf("❌ 保存配置失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ConfigResponse{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
		})
		return
	}

	json.NewEncoder(w).Encode(ConfigResponse{
		Success:    true,
		Message:    "配置已保存",
		ConfigFile: GetConfigFilePath(),
		Timestamp:  time.Now(),
	})
}

// checkOpCode 检查操作码
func checkOpCode(r *http.Request) bool {
	code := r.Header.Get("X-Operation-Code")
	return ValidateOperationCode(code)
}

// sendAuthError 发送认证错误响应
func sendAuthError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(ConfigResponse{
		Success:   false,
		Message:   "无效的操作码，请输入正确的操作码",
		Timestamp: time.Now(),
	})
}

// convertToDTO 将内部配置转换为 DTO
func convertToDTO(configs []StrategyConfig) []ConfigGroupDTO {
	groups := make([]ConfigGroupDTO, len(configs))
	for i, cfg := range configs {
		groups[i] = ConfigGroupDTO{
			ID:                  cfg.ID,
			MinTrendPercentUp:   cfg.MinTrendPercentUp,
			MinTrendPercentDown: cfg.MinTrendPercentDown,
			TrendWindowSecStart: cfg.TrendWindowSecStart,
			TrendWindowSecEnd:   cfg.TrendWindowSecEnd,
			Leg0SharePrice0:     cfg.Leg0SharePrice0,
			Leg1SharePrice1:     cfg.SharePriceLevel1,
			Leg1SharePrice2:     cfg.SharePriceLevel2,
			Leg1SharePrice3:     cfg.SharePriceLevel3,
			Leg1ShareAmount1:    cfg.ShareAmount1,
			Leg1ShareAmount2:    cfg.ShareAmount2,
			Leg1ShareAmount3:    cfg.ShareAmount3,
			MaxSlippage:         cfg.MaxSlippage,
			PriceTickSize:       cfg.PriceTickSize,
			SigmoidSensitivity:  cfg.SigmoidSensitivity,
			VolatilityBTC:       cfg.VolatilityBTC,
			VolatilityETH:       cfg.VolatilityETH,
			VolatilitySOL:       cfg.VolatilitySOL,
			VolatilityXRP:       cfg.VolatilityXRP,
			Tier0ZScore:         cfg.Tier0ZScore,
			Tier1ZScore:         cfg.Tier1ZScore,
			Tier2ZScore:         cfg.Tier2ZScore,
			Tier3ZScore:         cfg.Tier3ZScore,
			Tier4ZScore:         cfg.Tier4ZScore,
			MinEdgeTier0:        cfg.MinEdgeTier0,
			MinEdgeTier1:        cfg.MinEdgeTier1,
			MinEdgeTier2:        cfg.MinEdgeTier2,
			MinEdgeTier3:        cfg.MinEdgeTier3,
			MinEdgeTier4:        cfg.MinEdgeTier4,
		}
	}
	return groups
}

// convertFromDTO 将 DTO 转换为内部配置
func convertFromDTO(groups []ConfigGroupDTO) []StrategyConfig {
	configs := make([]StrategyConfig, len(groups))
	for i, g := range groups {
		configs[i] = StrategyConfig{
			ID:                  g.ID,
			MinTrendPercentUp:   g.MinTrendPercentUp,
			MinTrendPercentDown: g.MinTrendPercentDown,
			TrendWindowSecStart: g.TrendWindowSecStart,
			TrendWindowSecEnd:   g.TrendWindowSecEnd,
			Leg0SharePrice0:     g.Leg0SharePrice0,
			SharePriceLevel1:    g.Leg1SharePrice1,
			SharePriceLevel2:    g.Leg1SharePrice2,
			SharePriceLevel3:    g.Leg1SharePrice3,
			ShareAmount1:        g.Leg1ShareAmount1,
			ShareAmount2:        g.Leg1ShareAmount2,
			ShareAmount3:        g.Leg1ShareAmount3,
			MaxSlippage:         g.MaxSlippage,
			PriceTickSize:       g.PriceTickSize,
			SigmoidSensitivity:  g.SigmoidSensitivity,
			VolatilityBTC:       g.VolatilityBTC,
			VolatilityETH:       g.VolatilityETH,
			VolatilitySOL:       g.VolatilitySOL,
			VolatilityXRP:       g.VolatilityXRP,
			Tier0ZScore:         g.Tier0ZScore,
			Tier1ZScore:         g.Tier1ZScore,
			Tier2ZScore:         g.Tier2ZScore,
			Tier3ZScore:         g.Tier3ZScore,
			Tier4ZScore:         g.Tier4ZScore,
			MinEdgeTier0:        g.MinEdgeTier0,
			MinEdgeTier1:        g.MinEdgeTier1,
			MinEdgeTier2:        g.MinEdgeTier2,
			MinEdgeTier3:        g.MinEdgeTier3,
			MinEdgeTier4:        g.MinEdgeTier4,
		}
	}
	return configs
}
