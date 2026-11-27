# Polymarket Quantitative System Engineering — V2.0
批准日期：2026-11-27（America/Guayaquil）。项目负责人已批准章程、路线、角色、文档与 Issue 规范、离线安全原则、归档及 M0 Day 1。

## 目标
通过真实 Go 系统学习架构、静态与动态分析、Benchmark、并发及可靠性。优先级：学习、正确性与安全、可复现性、性能、可维护性、策略表现。交易收益不是当前验收指标；方向性策略不等同于无风险套利。

## 原则
P01 Evidence First；P02 Baseline First；P03 Measure Before Optimize；P04 Single Variable；P05 Reproducibility；P06 功能与性能双重回归；P07 Hot/Cold Path；P08 禁止未经批准的真实交易；P09 独立小 PR；P10 Learning First。

## 角色
负责人：学习、技术决策、最终验收及合并；ChatGPT：教学、架构、实验设计、独立审查与任务定义；Codex：实施、测试、采集证据及提交 PR。每次实验先教学与思考，再实施、分析、总结。

## 管理规范
保留原作者文档。新增 docs/engineering/learning、experiments、adr 与 reports/engineering/baseline、static-analysis、benchmarks、profiling（有实际文件时创建）。Issue 前缀：[LEARN] [AUDIT] [BASELINE] [EXP] [BUG] [SECURITY] [ADR] [DOC]。实验 Issue 必须包含背景、证据、假设、控制变量、范围、测量方法、验收及回滚条件。先查重，再创建；每个 PR 独立审查，负责人批准合并。不直接改 main。
