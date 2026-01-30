package api

import (
	"fmt"
	"log"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// StrategyConfig 策略配置 (只保留 Leg1 相关配置)
type StrategyConfig struct {
	ID string // 配置组 ID

	// Binance 趋势检测
	MinTrendPercentUp    float64 // 做多最小趋势阈值
	MinTrendPercentDown  float64 // 做空最小趋势阈值
	MinPriceEdge         float64 // 最小价格优势
	FairPriceTrendFactor float64 // 趋势到公允价的转换系数
	TrendWindowSecStart  int     // 开盘后多少秒开始检测
	TrendWindowSecEnd    int     // 开盘后多少秒停止检测

	// 动态 Share 配置
	Leg0SharePrice0  float64 // 最低价格阈值 (低于此不买)
	SharePriceLevel1 float64 // 第一价格阈值
	SharePriceLevel2 float64 // 第二价格阈值
	SharePriceLevel3 float64 // 第三价格阈值 (高于此不买)
	ShareAmount1     float64 // 价格在 (0, A] 区间买入的数量
	ShareAmount2     float64 // 价格在 (A, B] 区间买入的数量
	ShareAmount3     float64 // 价格在 (B, C] 区间买入的数量

	// 交易模式配置
	TradingMode string // "simulation" (默认) 或 "real" (真实下单)
	OrderType   string // "market" (市价FOK) 或 "limit" (限价GTC,做maker)

	// 其他配置
	ScanIntervalSec int // 扫描间隔 (秒)

	// 滑点配置
	MaxSlippage   float64 // 最大滑点 (例如 0.02 表示 2 美分)
	PriceTickSize float64 // 最小价格单位 (通常是 0.01)

	// Sigmoid 公允价模型参数
	SigmoidSensitivity float64 // K 敏感度系数 (建议 1.0 ~ 2.0)
	VolatilityBTC      float64 // BTC 波动率因子 (默认 0.0015)
	VolatilityETH      float64 // ETH 波动率因子 (默认 0.002)
	VolatilitySOL      float64 // SOL 波动率因子 (默认 0.003)
	VolatilityXRP      float64 // XRP 波动率因子 (默认 0.004)

	// [Tier0ZScore, Tier1ZScore] -> MinEdgeTier0
	// [Tier1ZScore, Tier2ZScore] -> MinEdgeTier1
	// [Tier2ZScore, Tier3ZScore] -> MinEdgeTier2
	// [Tier3ZScore, Tier4ZScore] -> MinEdgeTier3
	// [Tier4ZScore, +∞]          -> MinEdgeTier4
	Tier0ZScore  float64 // Tier0 Z-Score 阈值 (默认 0.5)
	Tier1ZScore  float64 // Tier1 Z-Score 阈值 (默认 0.8)
	Tier2ZScore  float64 // Tier2 Z-Score 阈值 (默认 1.0)
	Tier3ZScore  float64 // Tier3 Z-Score 阈值 (默认 1.5)
	Tier4ZScore  float64 // Tier4 Z-Score 阈值 (默认 2.0)
	MinEdgeTier0 float64 // Z [Tier0, Tier1] 的最小优势 (默认 0.08)
	MinEdgeTier1 float64 // Z [Tier1, Tier2] 的最小优势 (默认 0.06)
	MinEdgeTier2 float64 // Z [Tier2, Tier3] 的最小优势 (默认 0.03)
	MinEdgeTier3 float64 // Z [Tier3, Tier4] 的最小优势 (默认 0.01)
	MinEdgeTier4 float64 // Z > Tier4 的最小优势 (默认 0.00)
}

// configGroupYAML YAML 文件中的配置组结构
type configGroupYAML struct {
	ID                   string  `yaml:"id"`
	MinTrendPercentUp    float64 `yaml:"min_trend_percent_up"`
	MinTrendPercentDown  float64 `yaml:"min_trend_percent_down"`
	FairPriceTrendFactor float64 `yaml:"fair_price_trend_factor"`
	TrendWindowSecStart  int     `yaml:"trend_window_sec_start"`
	TrendWindowSecEnd    int     `yaml:"trend_window_sec_end"`
	Leg0SharePrice0      float64 `yaml:"leg0_share_price_0"`
	Leg1SharePrice1      float64 `yaml:"leg1_share_price_1"`
	Leg1SharePrice2      float64 `yaml:"leg1_share_price_2"`
	Leg1SharePrice3      float64 `yaml:"leg1_share_price_3"`
	Leg1ShareAmount1     float64 `yaml:"leg1_share_amount_1"`
	Leg1ShareAmount2     float64 `yaml:"leg1_share_amount_2"`
	Leg1ShareAmount3     float64 `yaml:"leg1_share_amount_3"`
	ScanIntervalSec      int     `yaml:"scan_interval_sec"`
	MaxSlippage          float64 `yaml:"max_slippage"`
	PriceTickSize        float64 `yaml:"price_tick_size"`
	// Sigmoid 模型参数
	SigmoidSensitivity float64 `yaml:"sigmoid_sensitivity"`
	VolatilityBTC      float64 `yaml:"volatility_btc"`
	VolatilityETH      float64 `yaml:"volatility_eth"`
	VolatilitySOL      float64 `yaml:"volatility_sol"`
	VolatilityXRP      float64 `yaml:"volatility_xrp"`
	// 动态优势配置
	Tier0ZScore  float64  `yaml:"tier_0_z_score"`
	Tier1ZScore  float64  `yaml:"tier_1_z_score"`
	Tier2ZScore  float64  `yaml:"tier_2_z_score"`
	Tier3ZScore  float64  `yaml:"tier_3_z_score"`
	Tier4ZScore  float64  `yaml:"tier_4_z_score"`
	MinEdgeTier0 *float64 `yaml:"min_edge_tier_0"`
	MinEdgeTier1 *float64 `yaml:"min_edge_tier_1"`
	MinEdgeTier2 *float64 `yaml:"min_edge_tier_2"`
	MinEdgeTier3 *float64 `yaml:"min_edge_tier_3"`
	MinEdgeTier4 *float64 `yaml:"min_edge_tier_4"`
}

type configFileYAML struct {
	Groups []configGroupYAML `yaml:"groups"`
}

var (
	globalConfigs     []StrategyConfig
	globalConfigsMu   sync.RWMutex   // 读写锁保护配置
	configFilePath    string         // 配置文件路径
	defaultBaseConfig StrategyConfig // 默认基础配置
	configInitialized bool           // 是否已初始化
)

// GetOrLoadConfigs 获取或加载配置
func GetOrLoadConfigs(baseConfig StrategyConfig) []StrategyConfig {
	globalConfigsMu.Lock()
	defer globalConfigsMu.Unlock()

	if !configInitialized {
		// 保存默认配置
		defaultBaseConfig = baseConfig

		// 尝试从 YAML 文件加载
		configFilePath = os.Getenv("CONFIG_FILE")
		if configFilePath == "" {
			configFilePath = "strategy_config.yaml"
		}

		if configs, err := LoadConfigsFromYAML(configFilePath, baseConfig); err == nil && len(configs) > 0 {
			globalConfigs = configs
			log.Printf("📝 从 %s 加载了 %d 个配置组", configFilePath, len(configs))
		} else {
			// 使用默认配置
			globalConfigs = []StrategyConfig{baseConfig}
			if err != nil {
				log.Printf("⚠️ 无法加载配置文件 %s: %v，使用默认配置", configFilePath, err)
			}
		}
		configInitialized = true
	}

	return globalConfigs
}

// GetCurrentConfigs 获取当前配置 (线程安全，只读)
// 如果配置尚未初始化，会先尝试从 YAML 文件加载
func GetCurrentConfigs() []StrategyConfig {
	globalConfigsMu.RLock()
	initialized := configInitialized
	globalConfigsMu.RUnlock()

	// 如果尚未初始化，先加载配置
	if !initialized {
		// 使用空的基础配置触发加载
		GetOrLoadConfigs(StrategyConfig{})
	}

	globalConfigsMu.RLock()
	defer globalConfigsMu.RUnlock()

	// 返回副本防止外部修改
	result := make([]StrategyConfig, len(globalConfigs))
	copy(result, globalConfigs)
	return result
}

// GetConfigFilePath 获取当前配置文件路径
func GetConfigFilePath() string {
	globalConfigsMu.RLock()
	defer globalConfigsMu.RUnlock()
	return configFilePath
}

// ReloadConfigs 强制重新加载配置 (热加载)
func ReloadConfigs() ([]StrategyConfig, error) {
	globalConfigsMu.Lock()
	defer globalConfigsMu.Unlock()

	if configFilePath == "" {
		configFilePath = os.Getenv("CONFIG_FILE")
		if configFilePath == "" {
			configFilePath = "strategy_config.yaml"
		}
	}

	configs, err := LoadConfigsFromYAML(configFilePath, defaultBaseConfig)
	if err != nil {
		return nil, fmt.Errorf("重新加载配置失败: %w", err)
	}

	globalConfigs = configs
	log.Printf("🔄 配置已热加载，共 %d 个配置组", len(configs))
	return configs, nil
}

// SaveConfigsToYAML 将配置保存到 YAML 文件
func SaveConfigsToYAML(configs []StrategyConfig) error {
	globalConfigsMu.Lock()
	defer globalConfigsMu.Unlock()

	if configFilePath == "" {
		configFilePath = "strategy_config.yaml"
	}

	// 转换为 YAML 结构
	yamlGroups := make([]configGroupYAML, len(configs))
	for i, cfg := range configs {
		yamlGroups[i] = configGroupYAML{
			ID:                   cfg.ID,
			MinTrendPercentUp:    cfg.MinTrendPercentUp,
			MinTrendPercentDown:  cfg.MinTrendPercentDown,
			FairPriceTrendFactor: cfg.FairPriceTrendFactor,
			TrendWindowSecStart:  cfg.TrendWindowSecStart,
			TrendWindowSecEnd:    cfg.TrendWindowSecEnd,
			Leg0SharePrice0:      cfg.Leg0SharePrice0,
			Leg1SharePrice1:      cfg.SharePriceLevel1,
			Leg1SharePrice2:      cfg.SharePriceLevel2,
			Leg1SharePrice3:      cfg.SharePriceLevel3,
			Leg1ShareAmount1:     cfg.ShareAmount1,
			Leg1ShareAmount2:     cfg.ShareAmount2,
			Leg1ShareAmount3:     cfg.ShareAmount3,
			ScanIntervalSec:      cfg.ScanIntervalSec,
			MaxSlippage:          cfg.MaxSlippage,
			PriceTickSize:        cfg.PriceTickSize,
			SigmoidSensitivity:   cfg.SigmoidSensitivity,
			VolatilityBTC:        cfg.VolatilityBTC,
			VolatilityETH:        cfg.VolatilityETH,
			VolatilitySOL:        cfg.VolatilitySOL,
			VolatilityXRP:        cfg.VolatilityXRP,
			Tier0ZScore:          cfg.Tier0ZScore,
			Tier1ZScore:          cfg.Tier1ZScore,
			Tier2ZScore:          cfg.Tier2ZScore,
			Tier3ZScore:          cfg.Tier3ZScore,
			Tier4ZScore:          cfg.Tier4ZScore,
			MinEdgeTier0:         &cfg.MinEdgeTier0,
			MinEdgeTier1:         &cfg.MinEdgeTier1,
			MinEdgeTier2:         &cfg.MinEdgeTier2,
			MinEdgeTier3:         &cfg.MinEdgeTier3,
			MinEdgeTier4:         &cfg.MinEdgeTier4,
		}
	}

	yamlConfig := configFileYAML{Groups: yamlGroups}

	data, err := yaml.Marshal(&yamlConfig)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(configFilePath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	// 更新内存中的配置
	globalConfigs = configs
	log.Printf("💾 配置已保存到 %s，共 %d 个配置组", configFilePath, len(configs))
	return nil
}

// LoadConfigsFromYAML 从 YAML 文件加载配置
func LoadConfigsFromYAML(filename string, defaultConfig StrategyConfig) ([]StrategyConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var yamlConfig configFileYAML
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, err
	}

	var configs []StrategyConfig

	// 从环境变量读取交易模式
	tradingMode := os.Getenv("TRADING_MODE")
	if tradingMode == "" {
		tradingMode = "simulation"
	}
	orderType := os.Getenv("ORDER_TYPE")
	if orderType == "" {
		orderType = "GTC" // 默认限价单
	}

	for _, g := range yamlConfig.Groups {
		config := StrategyConfig{
			ID:                   g.ID,
			MinTrendPercentUp:    valueOrDefault(g.MinTrendPercentUp, defaultConfig.MinTrendPercentUp),
			MinTrendPercentDown:  valueOrDefault(g.MinTrendPercentDown, defaultConfig.MinTrendPercentDown),
			FairPriceTrendFactor: valueOrDefault(g.FairPriceTrendFactor, defaultConfig.FairPriceTrendFactor),
			TrendWindowSecStart:  valueOrDefaultInt(g.TrendWindowSecStart, defaultConfig.TrendWindowSecStart),
			TrendWindowSecEnd:    valueOrDefaultInt(g.TrendWindowSecEnd, defaultConfig.TrendWindowSecEnd),
			Leg0SharePrice0:      valueOrDefault(g.Leg0SharePrice0, defaultConfig.Leg0SharePrice0),
			SharePriceLevel1:     valueOrDefault(g.Leg1SharePrice1, defaultConfig.SharePriceLevel1),
			SharePriceLevel2:     valueOrDefault(g.Leg1SharePrice2, defaultConfig.SharePriceLevel2),
			SharePriceLevel3:     valueOrDefault(g.Leg1SharePrice3, defaultConfig.SharePriceLevel3),
			ShareAmount1:         valueOrDefault(g.Leg1ShareAmount1, defaultConfig.ShareAmount1),
			ShareAmount2:         valueOrDefault(g.Leg1ShareAmount2, defaultConfig.ShareAmount2),
			ShareAmount3:         valueOrDefault(g.Leg1ShareAmount3, defaultConfig.ShareAmount3),
			ScanIntervalSec:      valueOrDefaultInt(g.ScanIntervalSec, defaultConfig.ScanIntervalSec),
			MaxSlippage:          valueOrDefault(g.MaxSlippage, 0.02),   // 默认 2 美分
			PriceTickSize:        valueOrDefault(g.PriceTickSize, 0.01), // 默认 1 美分
			// Sigmoid 模型参数
			SigmoidSensitivity: valueOrDefault(g.SigmoidSensitivity, 1.5), // 默认敏感度 1.5
			VolatilityBTC:      valueOrDefault(g.VolatilityBTC, 0.0015),   // BTC 默认波动率
			VolatilityETH:      valueOrDefault(g.VolatilityETH, 0.002),    // ETH 默认波动率
			VolatilitySOL:      valueOrDefault(g.VolatilitySOL, 0.003),    // SOL 默认波动率
			VolatilityXRP:      valueOrDefault(g.VolatilityXRP, 0.004),    // XRP 默认波动率
			// 动态优势配置
			Tier0ZScore:  valueOrDefault(g.Tier0ZScore, 0.5),      // 默认 0.5
			Tier1ZScore:  valueOrDefault(g.Tier1ZScore, 0.8),      // 默认 0.8
			Tier2ZScore:  valueOrDefault(g.Tier2ZScore, 1.0),      // 默认 1.0
			Tier3ZScore:  valueOrDefault(g.Tier3ZScore, 1.5),      // 默认 1.5
			Tier4ZScore:  valueOrDefault(g.Tier4ZScore, 2.0),      // 默认 2.0
			MinEdgeTier0: valueOrDefaultPtr(g.MinEdgeTier0, 0.08), // [Tier0, Tier1]
			MinEdgeTier1: valueOrDefaultPtr(g.MinEdgeTier1, 0.06), // [Tier1, Tier2]
			MinEdgeTier2: valueOrDefaultPtr(g.MinEdgeTier2, 0.03), // [Tier2, Tier3]
			MinEdgeTier3: valueOrDefaultPtr(g.MinEdgeTier3, 0.01), // [Tier3, Tier4]
			MinEdgeTier4: valueOrDefaultPtr(g.MinEdgeTier4, 0.00), // [Tier4, +∞]
			TradingMode:  tradingMode,
			OrderType:    orderType,
		}
		configs = append(configs, config)
	}

	return configs, nil
}

func valueOrDefault(val, defaultVal float64) float64 {
	if val != 0 {
		return val
	}
	return defaultVal
}

func valueOrDefaultPtr(val *float64, defaultVal float64) float64 {
	if val != nil {
		return *val
	}
	return defaultVal
}

func valueOrDefaultInt(val, defaultVal int) int {
	if val != 0 {
		return val
	}
	return defaultVal
}
