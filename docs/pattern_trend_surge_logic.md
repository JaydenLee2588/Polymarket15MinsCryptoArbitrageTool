# PatternTrendSurge (单边加速) 判定逻辑文档

## 核心定义
**PatternTrendSurge** 旨在捕捉短周期（15分钟）内的强单边行情。该模式不是为了捕捉反转，而是为了捕捉**顺势加速**段。

其核心思想是将 15 分钟周期划分为三个**博弈阶段 (Phases)**，每个阶段的风险点和关注指标权重不同。

---

## 🕒 阶段划分 (Phases)

| 阶段 | 时间范围 (TimeRemaining) | 关键词 | 核心目标 |
| :--- | :--- | :--- | :--- |
| **Phase 1: 趋势确立期** | > 10 分钟 (600s+) | 抢跑/识别 | 确认低成本启动，防止假突破 |
| **Phase 2: 趋势中继期** | 3 分钟 - 10 分钟 (180s - 600s) | 顺势/惯性 | 确认趋势惯性，过滤震荡 |
| **Phase 3: 决议锁定期** | 60 秒 - 3 分钟 (60s - 180s) | 安全垫/收敛 | 忽略技术指标，只看行权价安全距离 |
| **Phase 4: 终极保护期** | < 60 秒 | 禁区 | **禁止开仓** (防止滑点和 Spread 扩大) |

---

## 🛠️ 输入数据要求

### 市场状态
*   `TimeRemaining`: 当前周期剩余秒数。
*   `StrikePrice`: 行权价 (必须准确，若 API 获取失败则降级逻辑)。
*   `CurrentPrice`: 当前最新成交价。
*   `OpenPrice`: 当前 15 分钟 K 线的开盘价。

### 衍生指标
*   **Price Change**: `(CurrentPrice - OpenPrice) / OpenPrice`
*   **Vol Slope**: 最近 5 根 1m K 线的成交量线性回归斜率。
*   **Price Velocity**: 最近 30 秒的价格变动速度 (单位: 价格/秒)。
*   **Volatility (Sigma)**: 最近 15 分钟的波动率 (ATR 或 StdDev)。
*   **Z-Score**: `(CurrentPrice - StrikePrice) / Sigma`。

---

## 🚫 硬性过滤 (Hard Vetoes)
*无论评分多高，触犯以下红线直接拒绝。*

1.  **Phase 4 禁区**: `TimeRemaining < 60s` → **REJECT**.
2.  **趋势反转**:
    *   做多: `CurrentPrice < VWAP`
    *   做空: `CurrentPrice > VWAP`
3.  **动能严重背离 (Phase 1 & 2)**:
    *   做多: 价格新高但 `RSI_Slope < -0.1` (顶背离)。
    *   做空: 价格新低但 `RSI_Slope > 0.1` (底背离)。
4.  **VWAP 过热**: `Abs(VWAPDist) > 2 * ATR(14)` (默认约 1.0%~1.2%) → **REJECT** (视为情绪挤兑)。
5.  **量价背离 (Phase 1 特有)**: 价格上涨但 `VolSlope < 0` (无量上涨) → **REJECT**.
6.  **波动率突刺 (Volatility Spike Guard) (Phase 3 特有)**: 最近 30s 波动率 > 2 * 过去 15m 平均波动率 → **REJECT** (黑天鹅预警)。
7.  **行权价缺失 (Phase 3 特有)**: 若获取不到 StrikePrice → **REJECT (强制观察模式)** (因 Z-Score 无法计算，禁止盲赌)。
8.  **获利期望不足 (Minimum Edge) (Polymarket 特有)**: 若 `(1 - PolyPrice) < 0.05` (预期利润 < 5%) → **REJECT** (风险收益比极差，拒绝接盘)。

---

## 📝 分阶段评分卡 (Total Score: 100)
**触发阈值: Score ≥ 80**

### 🌟 Phase 1: 趋势确立期 (剩余 > 10m)
*风险点：假突破。必须有量能支持。*

| 检查项 | 判定标准 | 分值 | 备注 |
| :--- | :--- | :--- | :--- |
| **Price vs Open** | `Abs(Change) > 0.08%` 且方向正确 | **20** | 初步脱离成本区 |
| **Heiken Ashi** | 3连同色 + 最新无反向影线 | **20** | 趋势一致性 |
| **MACD** | 同向 (Hist > 0) + 增强 (Delta > 0) | **20** | 动能确认 |
| **Vol Slope** | `Slope > 0` (量增) | **20** | **核心指标** (权重提升) |
| **RSI Slope** | 同向增强 (`Abs > 0.2`) | **20** | 爆发力 |

### 🚀 Phase 2: 趋势中继期 (剩余 3m - 10m)
*风险点：追涨杀跌。必须有均线支撑。*

| 检查项 | 判定标准 | 分值 | 备注 |
| :--- | :--- | :--- | :--- |
| **Price vs Open** | `Abs(Change) > 0.15%` | **30** | 清晰脱离震荡区 |
| **VWAP Dist** | `0.1% < Dist < 2*ATR` | **40** | **核心指标** 健康偏离 (权重提升) |
| **Heiken Ashi** | 3连同色 + 实体未缩小 | **10** | 惯性保持 (权重降低) |
| **MACD** | 同向 (Hist > 0) | **10** | 动能维持 (允许放缓) |
| **Vol Slope** | `Slope > -0.05` | **10** | 量能未骤降 |

### 🔒 Phase 3: 决议锁定期 (剩余 60s - 3m)
*风险点：回归杀。只看行权价安全度。*

| 检查项 | 判定标准 | 分值 | 备注 |
| :--- | :--- | :--- | :--- |
| **Z-Score** | `Abs(Z-Score) > 2.5` | **60** | **核心指标** 2.5倍标准差安全垫 |
| **VWAP Support** | 价格未回测 VWAP | **20** | 趋势未坏 |
| **Heiken Ashi** | 最新一根未变色 | **20** | 无反转信号 |
| **MACD / RSI** | (忽略) | 0 | 指标钝化，不参考 |
| **扣分项** | `PriceVelocity` 指向行权价且过快 | **-100** | **速度风险** (见下文) |

---

## 🧠 Phase 3 速度风险判定 (Price Velocity Logic)

在 Phase 3，单纯的位置安全不够，必须防止**极速回归**。

**判定公式**:
1.  计算 `SafetyMargin = Abs(CurrentPrice - StrikePrice)`
2.  计算 `Velocity = (Price_Now - Price_30s_Ago) / 30`.
    *   **优化**: 对 Velocity 进行 **EMA 平滑** (例如 3s EMA)，过滤单根 K 线插针导致的假信号。
3.  **预测**: `ProjectedImpact = Abs(SmoothedVelocity) * TimeRemaining`.
4.  **规则**:
    *   如果 `Velocity` 方向指向行权价 (即不利方向);
    *   且 `ProjectedImpact > 0.8 * SafetyMargin` (按当前速度能跑完 80% 的安全垫);
    *   则触发 **REJECT** (或扣除 100 分)。

此逻辑确保即使当前还有 1% 的利润空间，如果价格像瀑布一样跌向行权价，也不会去接飞刀。

---

## 💡 实现细节建议

1.  **HA 影线容错**: `ShadowLen <= BodyLen * 0.05`。
2.  **动态阈值**: `ATR(14)` 应基于 1 分钟 K 线计算，反映当前分钟级别的波动率。
3.  **平滑处理**: 简单起见，Phase 切换采用硬切分，但由于评分卡逻辑内在的一致性（Phase 2 的高分往往也能在 Phase 3 得分，只要 Z-Score 足够），信号突变风险较小。为防止临界点反复横跳，建议在策略执行层增加 **5秒冷却时间** (同一分钟内不重复发单)。

---

## 🚪 退出策略 (Exit Strategy)

除了被动的止盈损，引入主动的退出机制以应对"伪单边"。

1.  **停滞退出 (Stagnation Exit)**:
    *   **条件**: 入场后 3 分钟内，价格波动幅度 < 0.1% (未出现预期加速)。
    *   **动作**: 视为单边失败，转为震荡市，主动调低评分或平仓离场，避免时间价值 (Time Decay) 损耗。
