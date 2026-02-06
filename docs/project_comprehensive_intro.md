# Polymarket 15分钟加密货币套利工具 - 全面项目介绍

## 1. 项目概述
本项目是一个针对 Polymarket "Crypto 15min" 市场的自动化高频/套利交易系统。它通过结合 **Binance 实时现货数据** 与 **Polymarket 期权价格**，利用统计学模型（Z-Score）和技术形态分析（Pattern），在极短的时间窗口内（15分钟）捕捉市场定价偏差。

核心目标：**在 Polymarket 二元期权（Yes/No）价格与 Binance 现货价格概率分布不匹配时进行套利。**

---

## 2. 核心架构与数据流

### 2.1 数据来源 (Data Sources)
系统依赖两路核心数据流：
1.  **Binance (基准源)**:
    *   **接口**: WebSocket Stream (`<symbol>usdt@kline_1m`)
    *   **用途**: 获取 BTC, ETH, SOL, XRP 的实时现货价格、K线数据、成交量。
    *   **处理**: 计算实时波动率 (Sigma)、技术指标 (RSI, MACD, Heiken Ashi)、突破检测。

2.  **Polymarket (交易标的)**:
    *   **接口**: GraphQL (Clob API) / REST
    *   **用途**: 获取当前活跃的 15分钟结算市场 (Market Slug)、合约价格 (Price)、盘口价差 (Spread)。
    *   **处理**: 匹配对应的 Binance 标的，计算理论胜率。

### 2.2 数据加工 (Data Processing)
系统内部维护一个实时状态机：
*   **价格对齐**: 将 Binance 价格与 Polymarket 行权价 (Strike Price) 进行比对。
*   **波动率计算**: 使用 Binance 最近 10-60 分钟的 K 线计算实时标准差 ($\sigma$)。
*   **概率模型**: 基于正态分布假设，计算当前价格最终落在行权价之上的**理论概率** (Theoretical Probability)。
    $$ Z = \frac{CurrentPrice - StrikePrice}{\sigma \times \sqrt{TimeLeft}} $$
    $$ P(Yes) = \Phi(Z) $$
*   **指标计算**:
    *   **RSI/MACD**: 用于辅助趋势判断。
    *   **Heiken Ashi**: 用于过滤市场噪音。

---

## 3. 策略体系 (Strategy System)

本项目实现了两套并行的策略引擎，可通过配置切换。

### 3.1 Sniper 策略 (v4.4 Current Core)
**模式**: `SNIPER_MODE=true`
**逻辑**: 基于剩余时间分段的精细化打击策略。

*   **Phase 1 & 2 (剩余时间 > 3分钟)**: **极值回归 (Mean Reversion)**
    *   **原理**: 当价格过度偏离行权价 (Z-Score > 3.0) 且出现反向突破信号时，押注价格回归。
    *   **触发**: Z-Score 绝对值极大 + 1分钟 K 线突破确认。

*   **Phase 3 (剩余时间 30秒 - 3分钟)**: **分歧套利 (Arbitrage)**
    *   **原理**: 捕捉理论胜率与 Polymarket 价格的巨大偏差 (Edge)。
    *   **触发**: `|TheoProb - PolyPrice| > Threshold`。

*   **Phase 4 (剩余时间 < 30秒)**: **死鱼收割 (Dead Fish)**
    *   **原理**: 接近结算时，胜率已定（如 >99%），但盘口仍有非理性价格（如 0.90）。
    *   **触发**: 胜率确定性极高 + 存在微小价差利润。

**退出机制**: **Touch-and-Go**
*   当 Z-Score 回归到正常区间 (|Z| < 1.0) 或 浮盈达到目标 (如 5%) 时立即离场，不等待结算。

### 3.2 Hysteresis 策略 (Legacy)
**模式**: `SNIPER_MODE=false` (且 `ENABLE_PATTERN_STRATEGY=true`)
**逻辑**: 基于状态机的迟滞（Hysteresis）系统，防止在阈值附近反复震荡开仓。
*   **状态**: 维护 `Neutral`, `Signal`, `Entry`, `Cooldown` 状态。
*   **Pattern**: 结合技术形态（如“火箭发射”、“断头铡刀”）作为信号过滤器。

---

## 4. 配置与模式 (Configuration)

配置分为**系统级环境配置**和**策略级参数配置**。

### 4.1 环境配置 (`.env`)
*   `ENABLE_PATTERN_STRATEGY=true`: 启用策略引擎总开关。
*   `SNIPER_MODE=true`: 启用 Sniper v4.4 逻辑 (推荐)。
*   `PATTERN_CONFIG_FILE=strategy_config_3.yaml`: 指定策略参数文件。
*   `TRADING_MODE`: `simulation` (模拟盘) / `real` (实盘)。
*   `ORDER_TYPE`: `FAK` (立即成交否则取消，推荐) / `FOK` / `GTC`。

### 4.2 策略参数 (`strategy_config_3.yaml`)
支持多组策略并行运行 (Config Groups)，每组可以有不同的 Sniper 参数：

```yaml
groups:
  - id: "趋势激进组"
    sniper:
      zscore_entry_threshold: 2.8  # 入场门槛较低
      profit_exit_pct: 3.0         # 快进快出
      spread_max: 0.08             # 容忍较高价差
    symbols: ["BTC", "ETH"]

  - id: "稳健套利组"
    sniper:
      zscore_entry_threshold: 3.5  # 仅在极端行情入场
      edge_threshold: 0.08         # 要求极高安全边际
    symbols: ["SOL", "XRP"]
```

---

## 5. 关键风控机制

1.  **冷却期 (Cooldown)**: 卖出后 30 秒内禁止同一方向开仓，防止连续止损。
2.  **滑点熔断 (Spread Check)**: 当 Polymarket 盘口价差 (Ask-Bid) 过大时，拒绝交易。
3.  **波动率保护**: 当 Binance 数据缺失或波动率计算异常 (<0.001) 时，使用默认高波动率以降低杠杆效应。
4.  **StrikePrice 验证**: 交易前强制检查行权价是否已从数据源正确同步。
5.  **Touch-and-Go**: 优先选择平仓获利，而非持有到期（降低结算风险）。

---

## 6. 技术栈

*   **语言**: Go (Golang)
*   **数据库**: SQLite (轻量级交易记录)
*   **API**: 
    *   Binance Spot Websocket
    *   Polymarket Clob API (Signed Requests)
*   **架构**: 
    *   `src/`: 核心算法 (Sniper, Indicators, Probability)
    *   `api/`: 交易逻辑与调度 (PatternStrategy, ConfigLoader)
    *   `main.go`: 程序入口与服务启动

