# 当前状态

2026-11-28：负责人批准 M0 Day 0 V1.1，当前状态 `M0_DAY0_POC_APPROVED`。

- V2.0 章程 PR #1 已合并；原始源码审计基线 `f7cfa2e0e8adac782ff2c55efc9e77c40e3e1d84`；PoC 起点 `492ee885d5042ec60539c09ca38c9a7754c81070`。
- [Issue #3](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/3)：OPEN，优先实施 Ubuntu 24.04 LTS / linux/arm64 容器化及公开行情冒烟测试。Claude Code 实施，ChatGPT 独立审查，负责人最终审批。Go 精确版本待兼容验证；保持业务代码与依赖声明不变。
- [Issue #2](https://github.com/JaydenLee2588/Polymarket15MinsCryptoArbitrageTool/issues/2)：OPEN，原生 macOS 实验暂停，保留历史实验分支 `baseline/m0-day1-environment-baseline`（核查 HEAD `12e98295a3676eea5c292536654645a525b254e5`）及首轮报告。构建测试曾因依赖获取受限而阻塞；缓存路径不一致是待验证假设。第二轮交付未获验收，不继承未经证实结论。
- PoC 特定授权：受控镜像、工具链及依赖准备；无真实凭证的有限应用启动；Binance/Polymarket 公开只读行情访问，无须逐次审批。真实资金交易、生产数据访问及未经审批的业务代码修改仍禁止。其他离线实验遵守原安全规范。
- 当前尚未验证 Docker 镜像、容器运行或公开行情链路；尚无正式性能基线。PoC 通过独立审查后定稿 ADR-001、冻结实验环境，再恢复 #2。以实际证据为准。
