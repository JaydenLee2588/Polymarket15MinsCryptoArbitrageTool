# Polymarket BTC 15M 自动套利策略实战蓝图 (v4.4 - 最终实战版)

## 1. 核心哲学：剥削概率，拒绝追价
* **极速收割 (Snappy Reversal)**：反转不再寻求“粘性”，只求“触点即离”。
* **双向猎杀**：支持做空 (Short/Sell) 逻辑，利用 Theta 衰减收割高估合约。
* **死鱼掠夺**：Phase 4 不再是禁区，而是寻找“已死亡但未归零”的确定性利润。

---

## 2. 🕒 动态博弈阶段划分 (Phases)

| 阶段 | 时间 (Remaining) | 核心目标 | 交易逻辑 |
| :--- | :--- | :--- | :--- |
| **Phase 1 & 2** | > 3 分钟 | 极值回归 | **触点即离 (Touch-and-Go)**：捕获 Z-Score 极值反弹。 |
| **Phase 3** | 30s - 3 分钟 | 分歧狙击 | **双向套利**：基于 CDF 理论胜率与 Poly 价格差进行掠夺。 |
| **Phase 4** | < 30 秒 | 死鱼收割 | **确定性套利**：只吃价格与胜率完全脱节的“死鱼单”。 |

---

## 3. 🛡️ 核心风控红线 (Hard Rules)

### 3.1 扫盘与执行限制 (Execution Rules)
* **执行原子化**：全量使用 **Limit IOC**。
* **禁止追价**：`Max_Retries` 设为 **1**。一次拿不到立即放弃，严禁在 Gamma 爆发期通过重试去“追涨杀跌”。
* **深度校验 (Depth Guard)**：单笔金额严禁超过卖一深度的 **60%**；穿透扫盘时 VWAP 成本必须在理论胜率覆盖内。
* **滑点熔断**：`(Poly_Ask - Poly_Bid) > 0.05` 时禁止开仓。

### 3.2 趋势过滤器 (Trend Filter)
* **1分钟突破确认**：
    * 做多：`CurrentPrice > High(Last_1min_Candle)`。
    * 做空：`CurrentPrice < Low(Last_1min_Candle)`。
* **事件屏蔽**：成交量 > 均值 5 倍时，停用所有反转逻辑，只允许顺势操作。

---

## 4. 📉 策略模块 A：极值反转 (Reversal - Touch & Go)
*适用时间：Phase 1 & 2*

* **入场判定**：
    1. `Abs(Z-Score) > 3.0`。
    2. 形态确认：价格突破最近 1 分钟高点 (Long) 或低点 (Short)。
* **止盈逻辑 (放弃 Hysteresis)**：
    * **快进快出**：一旦 `Abs(Z-Score) < 1.0` 或单笔盈利 > 5%，**立即市价平仓**，绝不留恋。

---

## 5. 🎯 策略模块 B：分歧套利 (Arbitrage & Shorting)
*适用时间：Phase 3 & 4*

* **判定逻辑**：
    1. **EV 掠夺**：`Theoretical_Prob` 与 `Poly_Price` 偏差 > 0.06。
    2. **支持做空**：若 `Poly_Price > 0.95` 且 `Theoretical_Prob < 50%`，执行 **Sell/Short**。
* **确定性死鱼 (Phase 4)**：最后 30 秒，寻找胜率已降至 < 1% 但价格仍 > 0.02 的合约进行收割。

---

## 6. 🚪 退出与止损策略 (Exit Strategy)

* **时间止损**：入场 120s 价格无有效位移，立即清仓。
* **Gamma 风险清仓**：最后 1 分钟，若价格加速度指向行权价且安全垫 `ProjectedImpact > 0.8 * Margin`，强制平仓。

---

## 7. 🛠️ 技术实现清单 (Go / EC2)

* **核心计算**：环形缓冲区维护 10s 秒级价格，计算 `Realtime_Sigma` 与 `CDF_Probability`。
* **交易器 (Fast-IOC)**：实现 `PlaceLimitIOC(price, amount, side)`，重试次数 = 1。
* **盘口监控**：实时计算 `VWAP_Ask` 和 `VWAP_Bid`，确保扫盘成本在 EV 阈值内。
* **数据对齐**：Binance/Poly 时间戳差 > 2s 自动熔断。

---

# 附录：代码实现规划

## 8. 架构定位

**v4.4 不是新策略，而是对现有 Pattern Strategy 的升级**

| 策略 | 文件 | 状态 |
|-----|-----|-----|
| Main Strategy | `api/strategy.go` | 保持不变 |
| Pattern Strategy | `api/pattern_strategy.go` | **← 升级目标** |

---

## 9. 文件修改清单

### 9.1 新增文件
| 文件 | 职责 |
|-----|-----|
| `src/sniper.go` | v4.4 核心决策引擎：Z-Score 计算、阶段判定、突破检测 |

### 9.2 修改文件
| 文件 | 改动内容 |
|-----|---------|
| `api/pattern_strategy.go` | 集成 v4.4 决策，修改止盈为 Touch-and-Go |
| `src/indicators.go` | 添加 `Get1MinHighLow()` 突破检测函数 |
| `api/pattern_config_loader.go` | 添加 v4.4 模式开关配置 |

---

## 10. 核心数据结构

```go
// src/sniper.go

// SniperInput 狙击策略输入
type SniperInput struct {
    CurrentPrice   float64   // Binance 当前价格
    StrikePrice    float64   // 行权价
    TimeLeftSec    float64   // 剩余秒数
    Sigma          float64   // 实时波动率
    PolyPriceUp    float64   // Polymarket UP 价格
    PolyPriceDown  float64   // Polymarket DOWN 价格
    Last1MinHigh   float64   // 最近1分钟高点
    Last1MinLow    float64   // 最近1分钟低点
    PolySpread     float64   // Poly 盘口价差
}

// SniperDecision 决策结果
type SniperDecision struct {
    Action      string  // "BUY_UP", "BUY_DOWN", "SELL_UP", "SELL_DOWN", "HOLD", "EXIT"
    Reason      string  // 决策原因
    Phase       int     // 当前阶段 (1-4)
    ZScore      float64 // 当前 Z-Score
    TheoProb    float64 // 理论胜率
    Edge        float64 // Edge = TheoProb - PolyPrice
    Confidence  float64 // 信号置信度
}
```

---

## 11. 核心函数设计

### 11.1 阶段判定
```go
// GetPhase 根据剩余时间判定阶段
func GetPhase(timeLeftSec float64) int {
    switch {
    case timeLeftSec > 180:  return 1  // > 3min: 极值回归
    case timeLeftSec > 30:   return 3  // 30s-3min: 分歧狙击
    default:                 return 4  // < 30s: 死鱼收割
    }
}
```

### 11.2 Z-Score 计算
```go
// CalculateZScore 计算 Z-Score
func CalculateZScore(currentPrice, strikePrice, sigma float64) float64 {
    if sigma < 0.001 { sigma = 0.001 }
    return (currentPrice - strikePrice) / sigma
}
```

### 11.3 突破检测
```go
// CheckBreakout 检测1分钟突破
func CheckBreakout(currentPrice, high1m, low1m float64) (isLongBreakout, isShortBreakout bool) {
    isLongBreakout = currentPrice > high1m
    isShortBreakout = currentPrice < low1m
    return
}
```

### 11.4 主决策函数
```go
// EvaluateSniper 主决策入口
func EvaluateSniper(input SniperInput) SniperDecision {
    phase := GetPhase(input.TimeLeftSec)
    zScore := CalculateZScore(input.CurrentPrice, input.StrikePrice, input.Sigma)
    theoProb := NormalCDF(zScore)
    
    // 滑点熔断
    if input.PolySpread > 0.05 {
        return SniperDecision{Action: "HOLD", Reason: "spread_too_wide"}
    }
    
    switch phase {
    case 1, 2:
        return evaluateReversal(input, zScore, theoProb)  // 极值反转
    case 3:
        return evaluateArbitrage(input, zScore, theoProb) // 分歧套利
    case 4:
        return evaluateDeadFish(input, zScore, theoProb)  // 死鱼收割
    }
    return SniperDecision{Action: "HOLD"}
}
```

---

## 12. 集成到 Pattern Strategy

在 `checkPatternBuySignal()` 中替换原有检测逻辑：

```go
// 原逻辑 (Hysteresis)
// result, newState := src.DetectPatternWithHysteresis(...)

// 新逻辑 (Sniper v4.4)
sniperInput := src.SniperInput{
    CurrentPrice:  currentPrice,
    StrikePrice:   state.StrikePrice,
    TimeLeftSec:   timeLeftSec,
    Sigma:         realtimeSigma,
    PolyPriceUp:   state.LatestPrices["UP"],
    PolyPriceDown: state.LatestPrices["DOWN"],
    Last1MinHigh:  high1m,
    Last1MinLow:   low1m,
    PolySpread:    math.Abs(state.LatestPrices["UP"] - (1 - state.LatestPrices["DOWN"])),
}
decision := src.EvaluateSniper(sniperInput)
```

---

## 13. 止盈逻辑修改 (Touch-and-Go)

在 `checkPatternSellCondition()` 中：

```go
// 原逻辑: Hysteresis 65分退出
// if score < 65 { sell }

// 新逻辑: Touch-and-Go
zScore := src.CalculateZScore(currentPrice, strikePrice, sigma)
profitPct := (currentPrice - pos.BuyPrice) / pos.BuyPrice * 100

if math.Abs(zScore) < 1.0 || profitPct > 5.0 {
    // 立即止盈
    executePatternSellWithReason(..., "touch_and_go_exit")
}
```

---

## 14. 配置开关

在 `pattern_strategy_config.yaml` 中添加：

```yaml
pattern_strategy:
  mode: "sniper"  # "hysteresis" | "sniper"
  sniper:
    zscore_entry_threshold: 3.0
    zscore_exit_threshold: 1.0
    profit_exit_pct: 5.0
    spread_max: 0.05
    dead_fish_min_edge: 0.02
```