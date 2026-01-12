package api

import (
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
	FeeRate         float64 // 费率
	ScanIntervalSec int     // 扫描间隔 (秒)
}

// configGroupYAML YAML 文件中的配置组结构
type configGroupYAML struct {
	ID                   string  `yaml:"id"`
	MinTrendPercentUp    float64 `yaml:"min_trend_percent_up"`
	MinTrendPercentDown  float64 `yaml:"min_trend_percent_down"`
	MinPriceEdge         float64 `yaml:"min_price_edge"`
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
	FeeRate              float64 `yaml:"fee_rate"`
	ScanIntervalSec      int     `yaml:"scan_interval_sec"`
}

type configFileYAML struct {
	Groups []configGroupYAML `yaml:"groups"`
}

var (
	globalConfigs     []StrategyConfig
	globalConfigsOnce sync.Once
)

// GetOrLoadConfigs 获取或加载配置
func GetOrLoadConfigs(baseConfig StrategyConfig) []StrategyConfig {
	globalConfigsOnce.Do(func() {
		// 尝试从 YAML 文件加载
		configFile := os.Getenv("CONFIG_FILE")
		if configFile == "" {
			configFile = "config.yaml"
		}

		if configs, err := LoadConfigsFromYAML(configFile, baseConfig); err == nil && len(configs) > 0 {
			globalConfigs = configs
			log.Printf("📝 从 %s 加载了 %d 个配置组", configFile, len(configs))
		} else {
			// 使用默认配置
			globalConfigs = []StrategyConfig{baseConfig}
			if err != nil {
				log.Printf("⚠️ 无法加载配置文件 %s: %v，使用默认配置", configFile, err)
			}
		}
	})

	return globalConfigs
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
		orderType = "market" // 默认市价单
	}

	for _, g := range yamlConfig.Groups {
		config := StrategyConfig{
			ID:                   g.ID,
			MinTrendPercentUp:    valueOrDefault(g.MinTrendPercentUp, defaultConfig.MinTrendPercentUp),
			MinTrendPercentDown:  valueOrDefault(g.MinTrendPercentDown, defaultConfig.MinTrendPercentDown),
			MinPriceEdge:         valueOrDefault(g.MinPriceEdge, defaultConfig.MinPriceEdge),
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
			FeeRate:              valueOrDefault(g.FeeRate, defaultConfig.FeeRate),
			ScanIntervalSec:      valueOrDefaultInt(g.ScanIntervalSec, defaultConfig.ScanIntervalSec),
			TradingMode:          tradingMode,
			OrderType:            orderType,
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

func valueOrDefaultInt(val, defaultVal int) int {
	if val != 0 {
		return val
	}
	return defaultVal
}
