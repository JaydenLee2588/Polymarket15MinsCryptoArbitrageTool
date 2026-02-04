package api

import (
	"log"
	"os"
	"strings"
	"sync"

	"Polymarket15MinsCryptoArbitrageTool/src"

	"gopkg.in/yaml.v3"
)

// PatternStrategyConfig Pattern 策略配置
type PatternStrategyConfig struct {
	ID string // 配置组 ID

	// 时间窗口
	TrendWindowSecStart int // 开盘后多少秒开始检测
	TrendWindowSecEnd   int // 开盘后多少秒停止检测

	// 价格档位
	Leg0SharePrice0  float64 // 最低价格阈值 (低于此不买)
	SharePriceLevel1 float64 // 第一价格阈值
	SharePriceLevel2 float64 // 第二价格阈值
	SharePriceLevel3 float64 // 第三价格阈值 (高于此不买)

	// 数量档位
	ShareAmount1 float64 // 价格在 (0, Level1] 区间买入的数量
	ShareAmount2 float64 // 价格在 (Level1, Level2] 区间买入的数量
	ShareAmount3 float64 // 价格在 (Level2, Level3] 区间买入的数量

	// 滑点
	MaxSlippage float64

	// Pattern 配置
	Patterns []src.PatternType // 关注的形态列表

	// 交易配置
	SellThreshold              float64  // 卖出阈值
	EnableStopLoss             bool     // 是否启用时间止损
	StopLossMinutesBeforeClose int      // 距离结算前X分钟止损
	Symbols                    []string // 支持的币种 (BTC, ETH, SOL, XRP)

	// 交易模式 (从环境变量读取)
	TradingMode string // "simulation" 或 "real"
	OrderType   string // "GTC", "FOK", "FAK"
}

// Pattern 名称映射 (中文 → PatternType)
var patternNameMap = map[string]src.PatternType{
	"火箭发射": src.PatternRocketLaunch,
	"空中加油": src.PatternAerialRefuel,
	"断头铡刀": src.PatternGuillotine,
	"水下死叉": src.PatternUnderwaterDead,
	"底背离":  src.PatternBullishDiv,
	"极度超卖": src.PatternOversold,
	"顶背离":  src.PatternBearishDiv,
	"情绪高潮": src.PatternClimaxTop,
	"织布机":  src.PatternChopping,
	"假突破":  src.PatternTrap,
}

// ==================== YAML 结构 ====================

type patternConfigGroupYAML struct {
	ID                         string   `yaml:"id"`
	TrendWindowSecStart        int      `yaml:"trend_window_sec_start"`
	TrendWindowSecEnd          int      `yaml:"trend_window_sec_end"`
	Leg0SharePrice0            float64  `yaml:"leg0_share_price_0"`
	Leg1SharePrice1            float64  `yaml:"leg1_share_price_1"`
	Leg1SharePrice2            float64  `yaml:"leg1_share_price_2"`
	Leg1SharePrice3            float64  `yaml:"leg1_share_price_3"`
	Leg1ShareAmount1           float64  `yaml:"leg1_share_amount_1"`
	Leg1ShareAmount2           float64  `yaml:"leg1_share_amount_2"`
	Leg1ShareAmount3           float64  `yaml:"leg1_share_amount_3"`
	MaxSlippage                float64  `yaml:"max_slippage"`
	Patterns                   []string `yaml:"patterns"`
	SellThreshold              float64  `yaml:"sell_threshold"`
	EnableStopLoss             bool     `yaml:"enable_stop_loss"`               // 时间止损开关
	StopLossMinutesBeforeClose int      `yaml:"stop_loss_minutes_before_close"` // 距离结算前X分钟止损
	Symbols                    []string `yaml:"symbols"`
}

type patternConfigFileYAML struct {
	Groups []patternConfigGroupYAML `yaml:"groups"`
}

// ==================== 全局状态 ====================

var (
	globalPatternConfigs   []PatternStrategyConfig
	globalPatternConfigsMu sync.RWMutex
	patternConfigFilePath  string
)

// ==================== 加载函数 ====================

// LoadPatternConfigs 从 YAML 文件加载 Pattern 策略配置
func LoadPatternConfigs(filename string) ([]PatternStrategyConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var yamlConfig patternConfigFileYAML
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, err
	}

	// 从环境变量读取交易模式
	tradingMode := os.Getenv("TRADING_MODE")
	if tradingMode == "" {
		tradingMode = "simulation"
	}
	orderType := os.Getenv("ORDER_TYPE")
	if orderType == "" {
		orderType = "GTC"
	}

	var configs []PatternStrategyConfig

	for _, g := range yamlConfig.Groups {
		// 解析 patterns
		var patterns []src.PatternType
		for _, pName := range g.Patterns {
			if pt, ok := patternNameMap[pName]; ok {
				patterns = append(patterns, pt)
			} else if pName != "" {
				log.Printf("⚠️ [%s] 未知的 Pattern 名称: %s", g.ID, pName)
			}
		}

		// 警告: patterns 为空
		if len(patterns) == 0 && len(g.Patterns) > 0 {
			log.Printf("⚠️ [%s] 所有配置的 patterns 都无效，策略将不会触发", g.ID)
		}

		// 解析 symbols (默认全部)
		symbols := g.Symbols
		if len(symbols) == 0 {
			symbols = []string{"BTC", "ETH", "SOL", "XRP"}
		}

		config := PatternStrategyConfig{
			ID:                         g.ID,
			TrendWindowSecStart:        g.TrendWindowSecStart,
			TrendWindowSecEnd:          g.TrendWindowSecEnd,
			Leg0SharePrice0:            g.Leg0SharePrice0,
			SharePriceLevel1:           g.Leg1SharePrice1,
			SharePriceLevel2:           g.Leg1SharePrice2,
			SharePriceLevel3:           g.Leg1SharePrice3,
			ShareAmount1:               g.Leg1ShareAmount1,
			ShareAmount2:               g.Leg1ShareAmount2,
			ShareAmount3:               g.Leg1ShareAmount3,
			MaxSlippage:                valueOrDefault(g.MaxSlippage, 0.03),
			Patterns:                   patterns,
			SellThreshold:              valueOrDefault(g.SellThreshold, 0.15),
			EnableStopLoss:             g.EnableStopLoss,
			StopLossMinutesBeforeClose: valueOrDefaultInt(g.StopLossMinutesBeforeClose, 3),
			Symbols:                    symbols,
			TradingMode:                tradingMode,
			OrderType:                  orderType,
		}
		configs = append(configs, config)
	}

	// 保存到全局
	globalPatternConfigsMu.Lock()
	globalPatternConfigs = configs
	patternConfigFilePath = filename
	globalPatternConfigsMu.Unlock()

	return configs, nil
}

// GetPatternConfigs 获取当前 Pattern 策略配置 (线程安全)
func GetPatternConfigs() []PatternStrategyConfig {
	globalPatternConfigsMu.RLock()
	defer globalPatternConfigsMu.RUnlock()

	result := make([]PatternStrategyConfig, len(globalPatternConfigs))
	copy(result, globalPatternConfigs)
	return result
}

// ReloadPatternConfigs 重新加载 Pattern 配置
func ReloadPatternConfigs() ([]PatternStrategyConfig, error) {
	globalPatternConfigsMu.RLock()
	filename := patternConfigFilePath
	globalPatternConfigsMu.RUnlock()

	if filename == "" {
		filename = "strategy_config_3.yaml"
	}

	return LoadPatternConfigs(filename)
}

// ContainsPattern 检查配置是否包含指定 Pattern
func (c *PatternStrategyConfig) ContainsPattern(pt src.PatternType) bool {
	for _, p := range c.Patterns {
		if p == pt {
			return true
		}
	}
	return false
}

// ContainsSymbol 检查配置是否支持指定币种 (精确匹配)
func (c *PatternStrategyConfig) ContainsSymbol(symbol string) bool {
	for _, s := range c.Symbols {
		if s == symbol {
			return true
		}
	}
	return false
}

// ContainsSymbolPrefix 检查配置是否支持指定币种 (前缀匹配)
// 支持 "BTC" 匹配 "BTC" 或 "BTCUSDT"
func (c *PatternStrategyConfig) ContainsSymbolPrefix(symbol string) bool {
	for _, s := range c.Symbols {
		if s == symbol || strings.HasPrefix(symbol, s) || strings.HasPrefix(s, symbol) {
			return true
		}
	}
	return false
}

// GetShareAmount 根据价格获取买入数量
func (c *PatternStrategyConfig) GetShareAmount(price float64) float64 {
	if price < c.Leg0SharePrice0 {
		return 0 // 价格太低，不买
	}
	if price > c.SharePriceLevel3 {
		return 0 // 价格太高，不买
	}
	if price <= c.SharePriceLevel1 {
		return c.ShareAmount1
	}
	if price <= c.SharePriceLevel2 {
		return c.ShareAmount2
	}
	return c.ShareAmount3
}
