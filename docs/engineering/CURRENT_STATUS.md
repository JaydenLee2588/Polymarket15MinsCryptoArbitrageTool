# 当前状态

2026-11-28：M0 Day 0 Docker PoC 已在 Mac mini 宿主机实际执行，当前状态 `M0_DAY0_POC_EXECUTED_PENDING_REVIEW`。

- [Issue #3](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/3)：OPEN，**当前唯一实施任务**。Ubuntu 24.04 LTS / linux/arm64 容器化与公开行情冒烟测试，分支 `poc/m0-day0-docker-smoke`（起点 main `492ee885d5042ec60539c09ca38c9a7754c81070`）。报告：`reports/engineering/poc/day0-docker-smoke.md`。
- [Issue #2](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/2)：**CLOSED / NOT_PLANNED**。旧版原生 macOS Day 1 实验已取消，不再恢复；其本地实验报告、脚本与 `tmp/` 缓存已按负责人授权删除。历史提交 `12e98295a3676eea5c292536654645a525b254e5` 仍保留在远端分支 `baseline/m0-day1-environment-baseline`，未删除、未改写。
- **Day 1 将在 Docker PoC 通过独立审查后，以容器环境重新规划并新建 Issue。**
- [PR #4](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/pull/4)（`docs/m0-day0-poc-status`）：截至本次核查仍为 OPEN、未合并，本分支未包含其内容，也未代为合并。两者都修改了本文件，合并时需由负责人选定保留版本。
- PoC 实测摘要：镜像构建成功（Go 1.27.4 官方 SHA256 校验通过，go.mod/go.sum 未变）；无私钥启动、SQLite、`/health`、Web UI、只读 API、15 分钟连续运行、优雅停止与重启均通过。外部行情：Binance REST、Polymarket 市场发现（回退路径）、订单簿 REST/WS 通过；**Binance 合约 WS 的应用端点 0 消息（FAIL）**、**Polymarket `public-search` 已不返回 15m 市场**。S11 经负责人批准执行一次模拟启停（0 笔成交）：Polymarket 应用内部链路通过，Binance 应用内部 WS 无数据（PARTIAL）。
- 已登记待修复（需改业务代码，未开始）：[Issue #6](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/6) Binance 合约 WS 端点；[Issue #7](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/7) Polymarket 15m 市场发现。
- 全程未使用私钥或交易凭证、未执行真实交易（仅一次获批的模拟启停，0 笔模拟成交）、未访问生产数据库、未修改业务代码与依赖声明。尚无正式性能基线。以实际证据为准。
