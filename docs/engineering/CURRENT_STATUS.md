# 当前状态

2026-11-28：M0 Day 0 Docker PoC 已在 Mac mini 宿主机实际执行，当前状态 `M0_DAY0_POC_EXECUTED_PENDING_REVIEW`，**PoC 结论 PARTIAL**。

## 治理与基线

- V2.0 项目章程已由负责人批准（PR #1 已合并）。
- 原始源码审计基线：`f7cfa2e0e8adac782ff2c55efc9e77c40e3e1d84`；PoC 起点 main：`492ee885d5042ec60539c09ca38c9a7754c81070`。
- 角色分工：ChatGPT 负责架构、教学与独立审查；Claude Code 负责实施与测试；项目负责人负责审批及最终验收。
- 授权边界：Day 0 PoC 允许访问 Binance / Polymarket 公开只读行情，无须逐次审批；离线实验与 Benchmark 的原有安全规范继续有效。真实资金交易、真实凭证、生产数据访问及未经审批的业务代码修改仍禁止。

## 任务状态

- [Issue #3](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/3)：OPEN，**当前唯一实施任务**。Ubuntu 24.04 LTS / linux/arm64 容器化与公开行情冒烟测试，分支 `poc/m0-day0-docker-smoke`（起点 main `492ee885d5042ec60539c09ca38c9a7754c81070`），[PR #5](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/pull/5) 待独立审查。报告：`reports/engineering/poc/day0-docker-smoke.md`。
- [Issue #2](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/2)：**CLOSED / NOT_PLANNED，永久取消**。旧版原生 macOS Day 1 实验不再恢复；其本地实验报告、脚本与 `tmp/` 缓存已按负责人授权删除。历史提交 `12e98295a3676eea5c292536654645a525b254e5` 仍保留在远端分支 `baseline/m0-day1-environment-baseline`，未删除、未改写。
- **Day 1 将在完整 Smoke Test 通过后，以容器环境重新规划并新建基线任务。**
- [PR #4](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/pull/4)（`docs/m0-day0-poc-status`）：负责人决策不再合并，已于 2026-11-28 关闭为“已被 PR #5 取代”；远端分支保留。PR #5 经独立审查后由负责人决定是否合并。

## PoC 实测摘要（PARTIAL）

- 验收合计：PASS 13（S09、S12 带限定）· FAIL 1（S08）· PARTIAL 1（S11）。
- 镜像构建成功（Go 1.27.4 官方 SHA256 校验通过，go.mod/go.sum 未变）；无私钥启动、SQLite、`/health`、Web UI、只读 API、15 分钟连续运行（空闲进程）、优雅停止与重启均通过。外部行情：Binance REST、Polymarket 市场发现（回退路径）、订单簿 REST/WS 通过；**Binance 合约 WS 的应用端点 0 消息（FAIL）**、**Polymarket `public-search` 已不返回 15m 市场**。S11 经负责人批准执行一次模拟启停（0 笔成交）：Polymarket 应用内部链路通过，Binance 应用内部 WS 无数据（PARTIAL）。
- 安全事实：经负责人批准执行过一次模拟启停（仅 S11，主交易策略关闭、仅维护行情状态）；未产生模拟成交；未执行真实交易；未使用真实私钥及真实交易凭证；未访问生产数据库；未修改业务代码与依赖声明。尚无正式性能基线。

## 后续顺序（Issue #3 决策）

1. PR #5 完成 R01–R03 整改及独立审查后方可合并；Issue #3 保持 OPEN。
2. [Issue #6](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/6) Binance 合约 WS 端点：已批准，PR #5 合并后**优先实施**。
3. [Issue #7](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/7) Polymarket 15m 市场发现：已批准，#6 验收后实施。#6、#7 各自独立 PR 与回归审查。
4. 两项完成后重跑完整 Smoke Test（含带行情负载的持续观察）。
5. 完整 Smoke Test 通过后，才冻结 ADR-001 与 Benchmark 环境，并另建容器基线任务。
- [Issue #8](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/8)（P-17 Polymarket WS slow consumer）：未来独立性能研究任务，不在本轮修复范围。

以 GitHub 实际记录与 `evidence/` 实测输出为准。
