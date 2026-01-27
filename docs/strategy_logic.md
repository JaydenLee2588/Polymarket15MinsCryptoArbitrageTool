# 策略与下单逻辑文档

本文档详细说明了当前的 Z-Score 公允价格计算模型以及下单的具体逻辑，包括自动赎回机制。

## 1. Z-Score 公允价格模型 (Sigmoid Z-Score Model)

该模型用于根据 Binance 的实时趋势计算 Polymarket 的公允价格。

### 核心公式

$$
Z = \frac{\text{Trend}}{\text{Volatility} \times \sqrt{\text{TimeRatio}}}
$$

$$
\text{FairPrice} = \frac{1}{1 + e^{-K \times Z}}
$$

### 参数说明

1.  **Trend (趋势)**:
    *   来源: Binance 15分钟 K线。
    *   计算: `(CurrentPrice - OpenPrice) / OpenPrice`。
    *   方向: 如果是做空 (DOWN)，Trend 取反。

2.  **Volatility (波动率因子)**:
    *   不同币种有不同的基准波动率 (配置项 `volatility_btc`, `volatility_eth` 等)。
    *   默认参考值:
        *   BTC: 0.0015
        *   ETH: 0.002
        *   SOL: 0.003
        *   XRP: 0.004

3.  **TimeRatio (时间因子)**:
    *   计算: `剩余秒数 / 900` (15分钟窗口)。
    *   **Gamma 效应**: 分母中有 `sqrt(TimeRatio)`。随着时间临近结算，剩余时间变少，`sqrt(TimeRatio)` 变小，会导致 Z-Score 变大。这模拟了期权Gamma在到期日临近时放大的效果。

4.  **Sensitivity (K, 敏感度)**:
    *   配置项: `sigmoid_sensitivity` (通常为 1.5 ~ 2.2)。
    *   作用: 控制 Sigmoid 函数的陡峭程度。K 越大，Z-Score 对价格的影响越剧烈。

### 逻辑流程

1.  计算 Binance 的趋势百分比。
2.  根据剩余时间计算时间因子。
3.  结合波动率计算 **Z-Score** (衡量当前趋势相对于波动率和时间的显著性)。
4.  通过 Sigmoid 函数将 Z-Score 映射到 [0, 1] 区间，得到 **公允价格 (FairPrice)**。
5.  **交易决策**: `FairPrice - MarketPrice > MinEdge` (公允价与市场价的差值必须大于设定的优势阈值)。

---

## 2. 下单逻辑 (Order Logic)

下单逻辑封装在 `src/clob_order.go` 中，根据 `strategy_config.yaml` 中的 `OrderType` 配置决定具体的下单行为。

### 订单类型与应用场景

系统支持三种主要订单类型，**由环境变量 `ORDER_TYPE` 全局控制** (配置文件中的 `order_type` 字段目前未生效)：

1.  **FOK (Fill-Or-Kill) [默认模式]**:
    *   **行为**: 发送市价单 (Market Order)，必须 **立即全部成交**，否则自动撤销。
    *   **用途**: 既想以此价格成交，又不想部分成交导致仓位碎片化时使用。
    *   **滑点控制**: 下单价格 = `当前市场价 + max_slippage`。
    *   **代码实现**: 调用 `PlaceMarketBuyOrder`。

2.  **FAK (Fill-And-Kill)**:
    *   **行为**: 发送市价单，**允许部分成交**，未成交部分立即撤销。
    *   **当前状态**: **您目前的实盘日志显示正在使用 FAK 模式** (可能是 `.env` 或环境变量配置了 `ORDER_TYPE=FAK`)。
    *   **用途**: 市场流动性较差，愿意接受部分成交时使用。
    *   **代码实现**: 调用 `PlaceMarketBuyOrderFAK`。

3.  **GTC (Good-Til-Cancelled)**:
    *   **行为**: 发送限价单 (Limit Order)，挂在 OrderBook 上作为 **Maker**。
    *   **用途**: 希望通过挂单赚取或节省 Spread，避免 Taker 费用。
    *   **风险**: 订单可能长时间不成交，导致错过行情。
    *   **代码实现**: 调用 `PlaceLimitBuyOrder`。

### 关键机制

1.  **最低金额限制**:
    *   Polymarket API 限制每笔订单名义价值 (Notional Value) 必须 >= **$1.0**。
    *   **自动调整**: 如果 `Price * Size < $1`，系统会自动增加 `Size` (Shares) 直到满足 $1 要求。

2.  **Maker/Taker Amount 转换**:
    *   API 需要区分支付的是 USDC 还是 Shares。
    *   **买入 (Limit/Market Buy)**: 输入 USDC (截断到2位小数)，获得 Shares。
    *   **卖出 (Limit/Market Sell)**: 输入 Shares，获得 USDC。

3.  **签名与防风控**:
    *   支持 EOA 私钥签名和 Gnosis Safe (Proxy) 签名。
    *   集成 `CycleTLS` 伪装 Chrome 指纹，绕过 Cloudflare 403/429 限制。

---

## 3. 自动赎回机制 (Auto Redemption)

策略内置了自动赎回服务 (`src/Redeemer`)，用于在市场决议后自动将获胜的 Shares 兑换回 USDC。

### 工作流程

1.  **监听决议**: 系统定期扫描已持有仓位的市场状态。
2.  **判定胜负**:
    *   当市场状态变为 `Closed` 且 `OutcomePrices` 中某一项价格 > 0.99 时，判定该方向获胜。
    *   检查持仓方向是否与获胜方向一致。
3.  **触发赎回**:
    *   只有 **获胜 (Winner)** 的仓位会被赎回。
    *   **失败 (Loser)** 的仓位价值归零，无需操作。
4.  **执行操作**:
    *   调用 Gamma 合约的 `redeemPositions` 函数。
    *   如果配置了 `BUILDER_KEY`，会尝试使用 Relay 服务加速上链。
    *   **上链确认**: 提交后，系统会启动轮询 (Polling) 机制以确认交易状态。
        *   **日志**: `[赎回] 开始轮询交易状态 (超时: 600s, maxAttempts: 200)`
        *   **含义**: 系统每 3 秒查询一次 Relayer API，最长等待 10 分钟 (600秒)，直到交易被打包确认 (STATE_CONFIRMED) 或失败。

### 异常处理
*   如果赎回因网络拥堵失败，系统会记录日志但目前没有复杂的重试队列（依赖下一次扫描）。
*   需要确保 Polygon 账户 (Maker) 有少量 MATIC (POL) 作为 Gas 费，或者使用支持 Gasless 的 Relay。

---

## 4. 当前策略特性与限制 (Strategy Capabilities)

基于当前 `v1.0` 代码版本与 `strategy_config.yaml` 配置的梳理：

| 功能特性 | 状态 | 详细说明 |
| :--- | :---: | :--- |
| **GTC 挂单 (Limit)** | **⚠️ 未启用** | 虽然代码默认支持 GTC，但**您当前的实盘日志显示使用的是 FAK (OrderType=FAK)**。这意味着您目前是作为 **Taker (吃单)** 进场的，需支付 Taker Fee。 |
| **开盘前5分钟交易** | **❌ 无** | 配置 `trend_window_sec_start: 300`。策略在市场开始后的 **前5分钟 (300秒) 内完全静默**，不进行任何交易监测。 |
| **主动卖出 (平仓)** | **❌ 无** | 策略是 **Buy & Hold (买入持有)** 模式。只买入 (Leg1)，不进行中途卖出和平仓。等待市场自然决议。 |
| **止损 (Stop Loss)** | **❌ 无** | 没有止损逻辑。一旦买入，除非决议获胜，否则若市场反向，资金将归零。 |
| **止盈 (Take Profit)** | **❌ 无** | 没有中途止盈逻辑。例如买入后价格翻倍，策略不会卖出锁定利润，而是持有到最后。 |
| **对冲 (Hedging)** | **❌ 无** | 没有对冲逻辑（如双向开仓或反向保护）。 |
| **追单 (Chasing)** | **❌ 无** | 信号触发时只会尝试下单一次。如果因为滑点或网络原因失败，**不会** 自动提价追单，而是等待下一个信号周期。 |

> **总结**: 当前策略是一个纯粹的 **"择时买入并持有至决议"** 的趋势跟随策略，风险敞口较大 (全额)，依赖高胜率 (High Win Rate) 来盈利。
