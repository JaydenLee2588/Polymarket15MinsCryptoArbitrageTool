# M0 Day 0 — Docker 容器化与真实行情 Smoke Test

**状态：PARTIAL — 容器化通过；Polymarket 全链路通过；Binance 合约 WS 不推送行情（S08 FAIL，S11 部分失败）**

Refs #3 · Refs #2（#2 已 CLOSED / NOT_PLANNED）
报告更新：2026-11-28 (UTC)
执行者：Claude Code，**Mac mini 原生 CLI 会话**（darwin/arm64，直接调用宿主机 Docker Desktop）

---

## 0. 一句话结论

镜像在 Mac mini 上真实构建成功（Ubuntu 24.04.5 / linux/arm64 / Go 1.27.4，官方 SHA256 校验通过，
go.mod/go.sum 未变）；应用在无私钥、无凭证下启动，SQLite、`/health`、Web UI、只读 API、
15 分钟连续运行、优雅停止与同镜像重启全部通过。

真实公开行情中发现两处**接口漂移**，均需修改业务代码才能修复，本阶段只登记不修：

1. **Binance 合约 WS**：应用使用的 `wss://fstream.binance.com/stream?streams=...` 握手 101 但 30 秒 **0 条消息**；
   同主机的 `/market/stream?streams=...` 30 秒 338 条。→ S08 **FAIL**（P-13）。
2. **Polymarket 市场发现**：应用的主路径 `public-search q="btc updown 15m"` 已**不返回任何 15m 事件**；
   应用代码自带的回退路径（按 `<sym>-updown-15m-<UTC 15m 窗口>` 构造 slug 调 `/markets?slug=`）
   对 BTC/ETH/SOL/XRP 均返回有效市场。→ S09 **PASS（经回退路径）**，主路径失效登记为 P-12。

S11（应用内部行情状态更新）经负责人批准后执行了一次模拟启停（§12）：Polymarket 侧应用内部发现 4 个市场、
订阅 8 个资产、回调持续增长 → 通过；Binance 侧应用内部 60 秒无消息触发“连接僵死”重连 → 在应用内复现 P-13。
S11 记为 **PARTIAL**。P-13 / P-12 已分别登记为 Issue #6 / #7，P-17 登记为 Issue #8。

验收合计：**PASS 13**（S09、S12 带限定）· **FAIL 1**（S08）· **PARTIAL 1**（S11）。PoC 整体 **PARTIAL**。

安全事实：经负责人批准执行过一次模拟启停（仅 S11；主策略关闭、仅维护行情状态），未产生模拟成交，
未执行真实交易，未使用真实私钥及真实交易凭证，未访问生产数据库。其余测试均为普通启动测试，未调用启动 / 停止 / 写入型接口（§18.4）。

---

## 1. 任务背景与本轮变更

Issue #3 的核心目标：证明现有 Go 系统能在 Ubuntu Docker 容器中构建、启动、处理真实公开行情。
本阶段不做性能优化、不建 Benchmark、不重构业务架构。状态词：PASS / FAIL / PARTIAL / BLOCKED / NOT_RUN / UNVERIFIED。

本轮（负责人 2026-11-28 决策）：

- 取消旧版 M0 Day 1 原生 macOS 实验；Issue #2 已由负责人侧关闭（NOT_PLANNED，经 GitHub API 核实 `state=closed, state_reason=not_planned`）。
- 清理旧 Day 1 本地产物（§2.1），将 Day 0 成果迁移到以最新 main 为起点的 `poc/m0-day0-docker-smoke`（§2.2）。
- 在宿主机原生执行通道上实际执行构建与 S01–S15。上一版报告中的「执行通道不可用 / BLOCKED」结论已被本轮实测取代。

---

## 2. Git 与工作区

### 2.1 旧 Day 1 产物清理（负责人授权）

删除前核查：`Dockerfile` 不引用 `tmp/gomodcache` 或任何旧实验路径（依赖在容器内 `go mod download` 获取；
`Dockerfile` 中出现的 `/tmp/` 是容器内临时下载路径）；`.dockerignore` 排除 `tmp/`；`scripts/poc/` 与
`.env.lab.example` 无旧路径引用。因此删除不影响 Day 0 构建（随后构建成功即为实证）。

实际删除清单（逐个 `rm -v` / `rmdir -v`，未使用 `git clean` / `git reset --hard`）：

| 路径 | 归属依据 | Git 状态 |
|---|---|---|
| `reports/engineering/baseline/dependency-preparation.md` | Day 1 R2 Gate 3 报告 | 未跟踪 |
| `reports/engineering/baseline/environment-baseline.md` | Day 1 Gate 1 | 未跟踪 |
| `reports/engineering/baseline/gate0-handover.md` | Day 1 Gate 0 | 未跟踪 |
| `reports/engineering/baseline/isolation-evidence.md` | Day 1 Gate 2 | 未跟踪 |
| `reports/engineering/baseline/side-effect-audit.md` | Day 1 Gate 1 | 未跟踪 |
| `reports/engineering/baseline/test-matrix.md` | Day 1 Gate 1 | 未跟踪 |
| `reports/engineering/baseline/dependency-download.log` | Day 1 首轮依赖下载日志 | 被 `*.log` 忽略 |
| `reports/engineering/baseline/scripts/01-preflight.sh` | Day 1 宿主机脚本 | 未跟踪 |
| `reports/engineering/baseline/scripts/02-isolation-check.sh` | 同上 | 未跟踪 |
| `reports/engineering/baseline/scripts/03-offline-validation.sh` | 同上 | 未跟踪 |
| `reports/engineering/baseline/scripts/EXECUTION-GUIDE.md` | 同上 | 未跟踪 |
| `reports/engineering/baseline/scripts/`（空目录） | 同上 | — |
| `tmp/gomodcache/`（约 985 MB，Go 模块缓存，先 `chmod -R u+w`） | Day 1 隔离 GOMODCACHE（Day 1 R2 记录） | 未跟踪 |
| `tmp/home/data/`、`tmp/home/logs/`、`tmp/home/`（均为空目录） | Day 1 隔离 HOME | 未跟踪 |
| `tmp/.DS_Store`、`tmp/` | macOS 噪音 / 空目录 | 未跟踪 |

修改撤销：`docs/engineering/CURRENT_STATUS.md` 的本地 diff 全部是 Day 1 R1/R2 记录，已 `git checkout -- <file>` 单文件撤销，
随后在新分支上按本轮决策重写。

保留：`.gitignore` 的本地 diff 只有一段（`.env.lab` / `.env.lab.*` / `!.env.lab.example`，注释 `Refs #3`），属 Day 0 配置，保留。

`reports/engineering/baseline/day1-environment-baseline.md` 是旧分支上**已提交**文件，未执行删除命令；
切换到以 main 为起点的新分支后自然不在工作树中，历史仍保存在远端分支。

### 2.2 分支迁移

| 项目 | 实测值 |
|---|---|
| `origin/main` | `492ee885d5042ec60539c09ca38c9a7754c81070`（`git fetch` 后核实；PR #1 合并点） |
| PR #4 `docs/m0-day0-poc-status` | 核查时 OPEN、未合并（head `178b1bdabe3512d77d0a5157a0fea6d01304ae5b`）；2026-11-28 按负责人决策**关闭为已被 PR #5 取代**，未合并，远端分支保留 |
| Issue #3 | OPEN |
| Issue #2 | CLOSED / not_planned |
| 新分支 | `poc/m0-day0-docker-smoke`，`git switch -c ... origin/main`；未跟踪 Day 0 文件随工作树带入 |
| 旧本地分支 | `git branch -d baseline/m0-day1-environment-baseline` **成功**（Git 提示：已合并到其上游 `origin/baseline/...`，但未合并到 HEAD），删除前位于 `12e9829`。未用 `-D` |
| 远端分支 | 全部保留（`baseline/m0-day1-environment-baseline`、`docs/engineering-v2-charter`、`docs/m0-day0-poc-status`） |
| 额外 worktree | 无（`git worktree list` 仅主目录） |
| 新分支中 `reports/engineering/baseline/` | 不存在（已核实） |

PR #4 已关闭，不再存在合并冲突问题；`CURRENT_STATUS.md` 以本分支版本为准。

---

## 3. 执行环境（S01 前置条件）

| 项目 | 实测值 |
|---|---|
| 宿主机 | Darwin arm64（Mac mini） |
| Docker context | `desktop-linux` |
| Docker Client / Engine | 29.4.2 / 29.4.2（API 1.54） |
| Docker Desktop | 4.72.0 (225998) |
| Engine OS/Arch | linux/arm64 |
| containerd / runc | v2.2.3 / 1.3.5 |

`evidence/00-execution-channel.txt` 是上一轮（沙箱会话）留下的历史记录，已被本轮取代，仅留档。

---

## 4. Dockerfile 审查（构建前）

本轮**未修改** Dockerfile 与 `.dockerignore`。逐项核对：

| 要求 | 结果 | 依据 |
|---|---|---|
| Ubuntu 24.04 LTS | ✔ | 两阶段 `FROM ubuntu:24.04`；运行镜像内 `PRETTY_NAME="Ubuntu 24.04.5 LTS"` |
| linux/arm64 | ✔ | 镜像 `Arch=arm64 Os=linux`；容器内 `uname -m` = `aarch64` |
| Go 1.27.4 官方 SHA256 | ✔ | 本轮独立核对 `https://go.dev/dl/?mode=json&include=all`：`go1.27.4.linux-arm64.tar.gz` `stable=true`，sha256 `3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92`，与 Dockerfile 一致；构建日志 `actual` 相同且 `sha256sum -c` → `OK` |
| `GOTOOLCHAIN=local` | ✔ | builder 内 `go env GOTOOLCHAIN` = `local` |
| go.mod / go.sum 不变 | ✔ | 构建内 `sha256sum -c` OK；宿主机前后摘要一致（§6）；`git diff origin/main -- go.mod go.sum` 为空 |
| SQLite 与应用同容器 | ✔ | `modernc.org/sqlite` 纯 Go 内嵌（§7） |
| 独立临时 SQLite | ✔ | `/app/data/trades.db`，无 Volume，容器删除即丢弃 |
| 镜像不含原作者私钥 | ✔ | `.dockerignore` 排除 `.env*`；镜像内 `find` 无 `.env*` / `*.key` / `*_real.yaml` / `*.db`，唯一 `.pem` 为系统 CA 包 `/usr/lib/ssl/cert.pem` |
| ENV 运行时注入 | ✔ | 镜像 ENV 仅安全默认值；凭证类变量全部由 `docker run -e` 注入为空 |
| 无私钥启动 | ✔ | §9 |
| 禁止真实交易及未经批准的模拟交易 | ✔ | `TRADING_MODE=simulation`、`ENABLE_MAIN_STRATEGY=false`、`ENABLE_PATTERN_STRATEGY=false`。S01–S10、S12–S15 的普通启动测试中未调用任何写入或策略启动接口；仅 S11 经负责人批准执行一次模拟启停（主策略关闭、仅维护行情状态、0 笔模拟成交，§12） |
| Web UI 仅本地 | ✔ | `-p 127.0.0.1:8090:8090`；宿主机 `lsof` 仅 `127.0.0.1:8090 (LISTEN)` |
| 不挂载生产目录 / Docker Socket | ✔ | `Binds=[] Mounts=[]`；容器内无 `/var/run/docker.sock`；`Privileged=false CapAdd=[]`，进程 `CapEff=0` |

本轮新增的 PoC 文件：`scripts/poc/wsprobe/main.go`（公开行情探测程序，`//go:build ignore`，不参与 `go build .` / `./...`，
非业务代码），以及 `scripts/poc/day0-docker-smoke.sh` 新增 `probe` 子命令（在容器网络内运行探测，探测程序通过
`docker cp` 注入一次性容器，不挂载宿主目录）。

---

## 5. 镜像构建结果（S01 / S02）

### 5.1 第 1 次构建：FAIL（基础设施瞬时故障）

```text
$ docker build --platform linux/arm64 --progress=plain -t polymarket-engineering-lab:day0 .
#2 resolve image config for docker-image://docker.io/docker/dockerfile:1
ERROR: failed to build: failed to solve: failed to resolve source metadata for docker.io/docker/dockerfile:1: lease does not exist: not found
[build exit=1 elapsed=704s]
```

完整日志：`evidence/01-build-attempt1.txt`。卡在拉取 BuildKit 前端镜像 `docker/dockerfile:1`，未进入任何构建步骤。

诊断（实测）：宿主机 `curl https://registry-1.docker.io/v2/` → 401（正常的未认证响应）；
容器内 `wget` 访问 Docker Hub 与 Binance 正常；`docker pull ubuntu:24.04` 成功。
构建挂起期间我另起的一次 `docker pull docker/dockerfile:1` 在 60 秒内未完成并被我终止；
**该 lease 错误很可能与这次并发拉取被中断有关**（根因假设，未验证）。
随后单独重试 `docker pull docker/dockerfile:1` 成功（`Image is up to date`，digest `sha256:572cebbc…9183`）。
未修改 Docker Desktop 设置、未修改 Dockerfile。

### 5.2 第 2 次构建：PASS

```text
$ docker build --platform linux/arm64 --progress=plain -t polymarket-engineering-lab:day0 .
[go-toolchain] actual  : 3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92
/tmp/go1.27.4.linux-arm64.tar.gz: OK
[go-toolchain] checksum VERIFIED -> 允许解压
go version go1.27.4 linux/arm64
go.mod: OK
go.sum: OK
[deps] go.mod / go.sum unchanged
+ go build -buildvcs=false -trimpath -o /out/polymarket-tool .
[build exit=0 elapsed=55s end=2026-11-28T08:17:05Z]
```

完整日志：`evidence/01-build.txt`。构建起点 HEAD `492ee885d5042ec60539c09ca38c9a7754c81070`，分支 `poc/m0-day0-docker-smoke`。
Go 1.27.4 编译 go.mod 声明 `go 1.24.1` 的代码库未出现任何编译错误（关闭 P-11）。

---

## 6. 镜像身份与 Go 版本

证据：`evidence/01b-image-identity.txt`

| 项目 | 实测值 |
|---|---|
| 镜像标签 | `polymarket-engineering-lab:day0` |
| **镜像 ID** | `sha256:54d8f544443779603d71ee6ffb4f5935a69926e03c39113f4e64f11f9d551270`（manifest list；containerd image store 下与 RepoDigest 相同） |
| 平台 manifest | `sha256:b42b0080c169c0f1c6e16ee81824c46564d7ee11874c0130d4eb4f8987111f03` |
| config | `sha256:614471b0796d3a44b9afaabfb14d51e850b240a97f409becf3240afb56874feb` |
| 架构 | `Arch=arm64 Os=linux`，容器内 `uname -m` = `aarch64`（原生，非模拟） |
| 大小 | 60,262,523 B |
| 创建时间 | 2026-11-28T08:17:05Z |
| 运行用户 | `app`（uid/gid 10001） |
| Go（builder 阶段） | `go version go1.27.4 linux/arm64`；`GOTOOLCHAIN=local`、`GOFLAGS=-mod=readonly`、`CGO_ENABLED=1` |
| builder 镜像 | `polymarket-engineering-lab:day0-builder`，仅用于 Go 版本核对与公开行情探测；不是运行镜像 |
| go.mod sha256 | `d69fcade2bf5c6b30a92a047ed9a8ba391b07638d9c76d875e2f1134750f0f50`（构建前后一致） |
| go.sum sha256 | `d8e4cba1de51a8717f59eaed3977db8ad32c2f220e59a41b171125ea806556db`（构建前后一致） |

builder 镜像 ID 会随构建上下文变化（本轮先后为 `45dbc3eb…`、`8161c850…`，原因是新增了探测程序文件）；运行镜像未重建。

---

## 7. SQLite 部署与验证

| 项目 | 核查结果 | 证据 |
|---|---|---|
| 驱动 | `modernc.org/sqlite` **v1.41.0** | `go.mod:11`；`internal/database/sqlite.go:12` 空导入 |
| 驱动类型 | **纯 Go**，无 cgo、无外部系统库 | 依赖树中 `modernc.org/libc`、`mathutil`、`memory` 均为纯 Go |
| `sql.Open` 驱动名 | `"sqlite"` | `internal/database/sqlite.go:32` |
| 是否需要独立 SQLite 服务 | **不需要** | 嵌入进程内 |
| 是否需要 `sqlite3` CLI | **不需要**（仅可选调试工具，未装进镜像） | — |
| 连接池 | `MaxOpenConns(1)`、`MaxIdleConns(1)`、`ConnMaxLifetime(1h)` | `sqlite.go:38-40` |
| WAL | `PRAGMA journal_mode=WAL`，失败只告警不致命 | `sqlite.go:43-45` |
| 目录创建 | `os.MkdirAll(filepath.Dir(dbPath), 0755)` | `sqlite.go:27` |
| 建表 | `CREATE TABLE IF NOT EXISTS simulation_trades` + 2 索引 + 列迁移 | `sqlite.go:58-89` |
| 初始化失败后果 | `log.Fatalf` — **进程直接退出** | `main.go:79-81` |

容器内路径（镜像 ENV）：`/app/data/trades.db`、`/app/data/bot.db`，日志 `/app/logs/YYYYMMDD/`。
目录在 Dockerfile 中预建并 `chown app:app`，确保 uid 10001 可写。
本次不引入持久化 Volume，容器删除后实验数据允许丢失。

**「真正初始化成功」的验证方式**（不只看目录是否存在）：脚本调用只读接口
`GET /api/v1/sim-stats`。该 handler 走 `database.GetTradeStats()` 实际查询
`simulation_trades` 表（`api/simulation_stats.go:43`），返回 200 + 合法 JSON
即证明表已建成且可查询。状态：**PASS**（§9）。

---

## 8. ENV 配置字段核查

字段清单来自源码逐一核查（`os.Getenv` / `getEnv` 全量 grep），**不含任何真实值**。

### 8.1 服务与存储

| 变量 | 必填 | 默认值 | 读取位置 | 缺失时行为 |
|---|---|---|---|---|
| `HOST` | 否 | `0.0.0.0` | `main.go:156` | 监听所有网卡（见 §19.3） |
| `PORT` | 否 | `8090` | `main.go:157` | 默认端口 **8090**（已重新核实） |
| `DB_PATH` | 否 | `data/trades.db` | `main.go:78` | 相对 WORKDIR 创建 |
| `BOT_DB_PATH` | 否 | `data/bot.db` | `main.go:104` | 仅在 Telegram Bot 启动时使用 |

### 8.2 交易安全开关

| 变量 | 默认值 | 读取位置 | 行为 |
|---|---|---|---|
| `TRADING_MODE` | 空 → `simulation` | `api/config_loader.go:286`、`api/pattern_config_loader.go:128` | **唯一的交易模式来源**，`"real"` 才进真实下单路径 |
| `ENABLE_MAIN_STRATEGY` | 空 → 启用 | `api/strategy.go:98,320` | 必须显式 `false` |
| `ENABLE_PATTERN_STRATEGY` | **`"true"`** | `main.go:181`、`api/strategy.go:280` | **代码默认开启**，必须显式 `false` |
| `SNIPER_MODE` | 空 → false | `api/pattern_config_loader.go:181` | `=="true"` 才开 |
| `CONFIG_FILE` | 空 → `strategy_config.yaml` | `api/config_loader.go:127,183` | 该文件**仓库中不存在** → 回退到空的默认配置 |
| `PATTERN_CONFIG_FILE` | 空 → `strategy_config_3.yaml` | `main.go:182`、`api/strategy.go:282` | 该文件**仓库中不存在** → 跳过 Pattern 策略 |

**重要架构事实**：YAML 配置文件里**没有** trading mode 字段。
`TradingMode` 只从环境变量取（`config_loader.go:286`、`pattern_config_loader.go:128`），
YAML 只提供阈值等参数。因此 `strategy_config_*_real.yaml` 本身不会让系统进入真实交易，
真正的开关是 `TRADING_MODE`。这是个好设计，但也意味着一个环境变量就能切换真假盘。

### 8.3 凭证类字段（本阶段全部留空）

| 变量 | 读取位置 | 何时才被需要 | 留空时行为 |
|---|---|---|---|
| `TELEGRAM_BOT_TOKEN` | `main.go:102` | 启用 Telegram Bot | 打印「跳过 Telegram Bot」（`main.go:112`） |
| `POLYMARKET_PRIVATE_KEY` | `api/strategy.go:123` | **仅当某配置组 `TradingMode=="real"`** | 打印错误并 `return false`，不启动策略；**不影响应用启动** |
| `PROXY_WALLET_ADDRESS` / `MAKER_ADDRESS` | `strategy.go:133,139`、`pattern_strategy.go:1134,1136` | real 模式代理钱包 | 回退到 EOA 直签 |
| `POLY_API_KEY` / `POLY_API_SECRET` / `POLY_PASSPHRASE` / `POLY_EOA_ADDRESS` | `strategy.go:150-153` | real 模式手动凭证 | 改为用私钥自动派生 API 凭证 |
| `BUILDER_KEY` / `BUILDER_SECRET` / `BUILDER_PASSPHRASE` | `strategy.go:208-210` | real 模式**自动赎回**（链上写入） | 打印「Builder API 凭证未配置 (自动赎回将不可用)」—— **本阶段的期望状态** |
| `POLYGON_RPC_URL` | `strategy.go:202` | real 模式赎回服务 | 默认 `https://polygon-rpc.com` |
| `TX_WAIT_TIMEOUT_SECONDS` | `src/redeem.go:711` | 赎回交易等待 | 用代码默认值 |

### 8.4 `.env.example` 与代码的偏差

- `.env.example` 中存在但**代码从不读取**：`BUILDER_ADDRESS`、`SCAN_INTERVAL`（死配置）。
- 代码读取但 `.env.example` **未记录**：`MAKER_ADDRESS`、`POLY_API_KEY`、`POLY_API_SECRET`、
  `POLY_PASSPHRASE`、`POLY_EOA_ADDRESS`、`POLYGON_RPC_URL`（隐式配置，文档缺口）。

### 8.5 实验配置外部化

提供 `.env.lab.example`（**仅占位符**）。使用方式：`cp .env.lab.example .env.lab`，
`docker run --env-file .env.lab ...`。已向 `.gitignore` 追加 `.env.lab` / `.env.lab.*`
（保留 `!.env.lab.example`），并在 `.dockerignore` 中以 `.env.*` / `*.env` 排除，
确保实验配置既不进 Git 也不进镜像。

**不要从 `.env.example` 复制任何值** —— 原因见 §18.1。

---
## 9. 无私钥启动与 SQLite（S03 / S04 / A01）

证据：`evidence/02-offline-start.txt`（`--network none`，无外网、无凭证）

```text
Running=true ExitCode=0 OOM=false Restarts=0
2026/11/28 08:18:15 ⚠️ 加载 .env 文件失败: open .env: no such file or directory (使用系统环境变量)
2026/11/28 08:18:15 ✅ 数据库已初始化: /app/data/trades.db
2026/11/28 08:18:15 📝 [TradeHistory] 日志文件已创建: logs/20261128/trade_history_0318.log
2026/11/28 08:18:15 ℹ️ 未配置 TELEGRAM_BOT_TOKEN，跳过 Telegram Bot
2026/11/28 08:18:15 ℹ️ Pattern 策略已禁用 (ENABLE_PATTERN_STRATEGY=false)
2026/11/28 08:18:15 🚀 服务器启动在 http://0.0.0.0:8090
```

- 所有凭证变量为空字符串（`POLYMARKET_PRIVATE_KEY=`、`BUILDER_*=`、`POLY_API_*=`）；镜像内无 `.env` / `.env.example`。
- 未生成任何测试钱包（模拟路径不需要，A04）。
- SQLite：`/app/data/trades.db` + `-wal` + `-shm` 已创建；`GET /api/v1/sim-stats`（实际查询 `simulation_trades` 表）返回
  `200 {"success":true,...,"trade_count":0,...}`，证明表已建成且可查询。
- 观察：trade history 日志文件名 `_0318` 与 UTC `08:18` 不一致，像是按美东时间命名（代码内部时区设置，未深究，不影响本阶段）。

## 10. HTTP、Web UI 与只读 API（S05 / S06）

| 请求 | 结果 |
|---|---|
| `GET /health`（容器内） | `200` `{"service":"Polymarket15MinsCryptoArbitrageTool","status":"ok"}` |
| `GET /health`（宿主机 `127.0.0.1:8090`） | `200` |
| `GET /`（Web UI） | `200` `text/html`，1782 B，`<title>Polymarket 15-Min Crypto Arbitrage Tool</title>` |
| `GET /api/v1/config` | `200` `{"success":true,"config_file":"strategy_config.yaml","groups":[{...全 0 默认值...}]}` |
| `GET /api/v1/sim-stats` | `200`，trade_count=0 |

普通启动测试（S01–S10、S12–S15）中未调用任何写入型接口：`DELETE /api/v1/trades`、`PUT /api/v1/config`、
`POST /api/v1/config/reload`、`/api/v1/sim-copy-trading`、`/api/v1/sim-stop-trading`。
唯一例外是 S11：经负责人批准，在独立容器中各调用一次 `POST /api/v1/sim-copy-trading?type=2` 与
`POST /api/v1/sim-stop-trading`（外加一次不带操作码、被 401 拒绝的鉴权检查），见 §12。
`DELETE /api/v1/trades`、`PUT /api/v1/config`、`POST /api/v1/config/reload` 在整个实验期间均未调用。

只读接口清单（经代码确认）：`/health`、`/`、`/index.html`、`/api-test.html`、`GET /api/v1/sim-stats`、`GET /api/v1/config`。

## 11. 公开行情 —— 外部连通性（S07–S10）

应用容器以正常 bridge 网络启动（`-p 127.0.0.1:8090:8090`，`evidence/03-online-market-data.txt`）。
外部探测在**容器网络内**执行（`evidence/03b-container-probe.txt`，`scripts/poc/wsprobe/main.go`），
请求形态与应用代码一致；宿主机侧 curl 探测同时留档于 `03-online-market-data.txt`。
**这里证明的是“外部接口可达且有数据”，不等于“应用内部已接收行情”**（后者见 §12）。

### 11.1 Binance

| ID | 请求 | 结果 |
|---|---|---|
| S07 | `GET api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=15m&limit=2`（`src/binance_rest.go`） | **200**，2 行 |
| S07 | 宿主机 `ticker/price`、`klines` | 200 / 200 |
| S08 | **[APP]** `wss://fstream.binance.com/stream?streams=btcusdt@kline_1m/btcusdt@aggTrade`（`src/binance_ws.go:16` 的 URL 形态） | 握手 **101**，30 s 内 **0 条消息** |
| 诊断 | `wss://fstream.binance.com/ws/btcusdt@aggTrade` | 101，30 s **0 条** |
| 诊断 | `wss://fstream.binance.com/market/stream?streams=btcusdt@kline_1m/btcusdt@aggTrade` | 101，30 s **338 条**，首条 1.53 s（`{"stream":"btcusdt@kline_1m","data":{"e":"kline",...}}`） |
| 诊断 | `wss://stream.binance.com:9443/stream?streams=...`（现货） | 101，30 s **83 条**，首条 2.23 s |

结论：网络可达、服务端可推送；**应用使用的合约 WS 路径不再推送行情**，而 `/market/stream` 路径正常推送。
这与“合约 WS 按 market / public / private 分路径”的端点迁移相符（**根因假设**，未查阅官方公告核实）。
修复需改 `src/binance_ws.go:16`（业务代码），本阶段不改 → **S08 FAIL**，登记 P-13（Issue #6）。

宿主机 curl 的 WS 握手测试返回 `000`（超时 exit=28），这是 curl 的 HTTP Upgrade 测试方式不适用，以容器内 gorilla/websocket 结果为准。

### 11.2 Polymarket

| ID | 请求 | 结果 |
|---|---|---|
| S09 主路径 | `GET gamma-api/public-search?q=btc updown 15m&events_status=active`（`api/strategy.go:1180-1195`） | 200，3 个事件，**全部不是 15m 市场**（MicroStrategy / El Salvador 等长期市场）。eth/sol/xrp 查询同为 200 |
| S09 回退路径 | `GET gamma-api/markets?slug=<sym>-updown-15m-1795853700`（`api/strategy.go:1344-1370` 的请求形态，UTC 15m 对齐） | BTC / ETH / SOL / XRP **均 1 个市场**，`active=True closed=False`，2 个 token，`outcomes=["Up","Down"]`（`evidence/03c-discovery-fallback.txt`） |
| S09 | `GET gamma-api/events?slug=btc-updown-15m-1795853700` | 200，`"Bitcoin Up or Down - November 28, 3:15AM-3:30AM ET"`，`acceptingOrders=true` |
| S10 | `GET clob.polymarket.com/book?token_id=<tok0>` | **200**，23 bids / 76 asks |
| S10 | `wss://ws-subscriptions-clob.polymarket.com/ws/market`，发送 `{"assets_ids":[2 tokens],"type":"market"}` + `"type":"book"`（与 `src/realtime/stream.go:254-268` 相同） | 握手 **101**，30 s **9453 条**，首条 945 ms（`[{"market":"0x0c45…","asset_id":"2769…","bids":[...]` ） |

结论：
- 15m 市场**仍然存在**，slug 规则 `<sym>-updown-15m-<unix>` 与 `outcomes` 文本仍与代码假设一致。
- `public-search` **已不返回 15m 事件**，应用主发现路径失效；回退路径的请求形态有效 → **S09 PASS（经回退路径，外部层）**，主路径失效登记 P-12。
- 订单簿 REST 与 WS 订阅格式有效 → **S10 PASS（外部层）**。
- 新观察：市场描述中结算来源为 **Chainlink BTC/USD TWAP**（“time-weighted average price … generated by Chainlink”）。
  项目策略以 Binance 价格为参考，结算口径变化与策略有效性相关，属后续阶段议题（P-15），不在本 PoC 范围。

## 12. 应用内部行情集成（S11 / A03）

**PARTIAL。** 负责人于 2026-11-28 批准最小模拟启停验证，证据 `evidence/07-s11-app-integration.txt`。

执行方式：同一镜像新容器 `polymarket-lab-day0-s11`（`-p 127.0.0.1:8091:8090`），环境与其它测试完全相同
（`TRADING_MODE=simulation`、`ENABLE_MAIN_STRATEGY=false`、`ENABLE_PATTERN_STRATEGY=false`、凭证全空，容器内 `env` 已核对）。
操作码从源码读入 shell 变量，经 `X-Operation-Code` 头传入，**未写入证据或报告**（已 grep 核实证据文件中不含该值）。

| 时间 (UTC) | 事件 |
|---|---|
| 09:18:51 | 无操作码 `POST /api/v1/sim-copy-trading?type=2` → **401**（鉴权生效） |
| 09:18:51 | 带操作码同请求 → 200 `Started Simulation (Background)`，`config_count=1` |
| 09:18:51 | 日志：`Main 策略已禁用 … 仅维护市场状态`、`已初始化只读 ClobClient (用于模拟盘订单簿验证)`、`Pattern 策略未启用` |
| 09:18:52–53 | Binance WS 已连接；REST 预加载 BTC/ETH/SOL/XRP 1m K 线各 240 根 |
| 09:18:59–09:19:02 | MarketManager 检测到 8 个资产（4 个市场），REST 快照 8/8 成功，订阅 Polymarket WS（market + book） |
| 09:19:15 | **Polymarket WS 被服务端关闭 `close 1013: slow consumer: send buffer full`**；2 s 后重连，快照 8/8 → 重新订阅成功 |
| 09:19:25 / 09:19:55 / 09:20:25 | 心跳：`活跃市场: 4`、回调数 42,934 → 135,024 → 203,140 |
| 09:19:53 | **`[Binance] 连接僵死，60秒未收到消息，正在触发重连`**；重连“成功”后仍为同一 URL |
| 09:20:51 | `POST /api/v1/sim-stop-trading` → 200；MarketManager 与 Binance WS 均已关闭，“模拟循环已确认退出” |
| 09:20:54 | `sim-stats` trade_count=0（前后一致，未产生任何模拟成交） |
| 结束 | `docker stop` → ExitCode=0 |

结论：

- **Polymarket 应用内部链路：PASS。** 市场发现（经回退路径，主路径失效见 P-12 / #7）→ 订单簿快照 → WS 订阅 → 状态回调持续更新。
- **Binance 应用内部链路：FAIL。** REST 预加载正常，WS 连上但无消息，60 秒僵死检测触发，与 §11.1 的外部探测一致（P-13 / #6）。
  心跳仍报告 `Binance: 已连接`，健康状态具有误导性。
- 新发现 P-17：Polymarket WS 在订阅 8 个资产后约 13 秒即因 slow consumer 被服务端断开，说明应用读取 / 处理速度跟不上推送量；
  本次自动重连成功，但长时间运行下的频率未观察。
- 本次模拟启停期间 0 笔模拟成交（Main 策略关闭，仅维护行情状态），未触及任何真实下单路径。

## 13. 连续运行（S12）

证据：`evidence/04-observe-15min.txt`。实际时长 **15 分 17 秒**（`08:23:53Z` → `08:39:10Z`），每分钟采样 1 次，共 15 次：

| 指标 | 结果 |
|---|---|
| `running` | 15/15 true |
| `restarts` | 0 |
| `/health`（宿主机） | 15/15 HTTP 200 |
| CPU | 0.00% |
| 内存 | 16.77 → 16.92 MiB（上限 7.75 GiB） |
| 日志 | 仅启动 7 行，无错误、无重复告警、无断连 |
| SQLite 文件 | 大小未变 |

说明：这是**空闲进程**的稳定性观察（未启动策略、无行情接入），不能代表带行情负载的稳定性，也不是 Benchmark。

## 14. 停止与重启（S13 / S14）

证据：`evidence/05-lifecycle.txt`

- `docker stop -t 20` → `ExitCode=0 OOMKilled=false`，日志 `🛑 正在关闭服务器...` → `✅ 服务器已优雅退出`（同一秒内完成）。**S13 PASS**
- 删除容器后用同一镜像重建 → `Running=true`；`/health` 200；`/api/v1/sim-stats` 200（新临时库，trade_count=0）。**S14 PASS**
- 测试结束：`polymarket-lab-day0`、`polymarket-lab-day0-offline` 均 `Exited (0)`，容器保留未删除以便复查（`evidence/06-teardown.txt`）。

## 15. 安全核查（S15）

证据：`evidence/02-offline-start.txt`、`evidence/05b-isolation.txt`

| 检查 | 结果 |
|---|---|
| 特权 / 能力 | `Privileged=false CapAdd=[]`；PID 1 `CapPrm=CapEff=0000000000000000` |
| 挂载 | `Binds=[] Mounts=[]`；无 `/var/run/docker.sock` |
| 端口 | `127.0.0.1:8090` only（`docker port` 与宿主机 `lsof`） |
| 用户 | uid 10001 `app` |
| 凭证 | 所有凭证类 ENV 为空；镜像内无 `.env` / `.env.example` / 私钥文件 |
| 交易 | `TRADING_MODE=simulation`；Pattern 策略日志确认禁用；所有容器的 `sim-stats` 始终 trade_count=0。普通启动测试（`02`–`06` 证据）未调用任何策略 / 写入接口；S11（`07` 证据）为负责人批准的一次模拟启停，主策略关闭、0 笔模拟成交、无真实交易 |
| 生产数据 | 未挂载、未访问；仅容器内临时库 |

记录缺陷：`05b-isolation.txt` 中筛选 `ENABLE_*` 变量的 grep 正则写错（`ENABLE_=`），未打印出两个 `ENABLE_*_STRATEGY` 值；
其取值由同文件夹中 `docker run -e ENABLE_MAIN_STRATEGY=false -e ENABLE_PATTERN_STRATEGY=false` 命令行、
镜像 ENV（`01b-image-identity.txt`）及应用日志 `Pattern 策略已禁用 (ENABLE_PATTERN_STRATEGY=false)` 共同证明。

**S15 PASS。**

---

## 16. S01–S15 验收矩阵

| ID | 验证项目 | 状态 | 证据 / 说明 |
|---|---|---|---|
| S01 | Docker 镜像实际构建成功 | **PASS** | 第 2 次构建 exit=0（第 1 次因拉取前端镜像失败，§5.1）；镜像 `sha256:54d8f544…1270` |
| S02 | Go 应用实际编译成功 | **PASS** | go1.27.4 linux/arm64，`go build .` 成功，go.mod/go.sum 未变 |
| S03 | 容器及应用正常启动（无私钥） | **PASS** | `--network none`，凭证全空，Running=true |
| S04 | SQLite 实际初始化成功 | **PASS** | db/wal/shm 存在；`sim-stats` 查表 200 |
| S05 | HTTP 健康检查 | **PASS** | 容器内与宿主机 `127.0.0.1` 均 200 |
| S06 | Web UI 与只读 API | **PASS** | `/` 200 HTML；`/api/v1/config`、`/api/v1/sim-stats` 200 |
| S07 | Binance REST 行情 | **PASS** | klines / ticker 200，有数据 |
| S08 | Binance WebSocket 行情 | **FAIL** | 应用 URL 形态 101 + 0 消息/30 s；`/market/stream` 338 条/30 s（P-13，需改业务代码） |
| S09 | Polymarket 市场发现 | **PASS（回退路径）** | `public-search` 主路径不再返回 15m（P-12）；`/markets?slug=` 回退形态 4/4 币种命中 |
| S10 | Polymarket 订单簿 REST / WS | **PASS** | book 200（23/76 档）；WS 101 + 9453 条/30 s |
| S11 | 应用内部行情接收及状态更新 | **PARTIAL** | 已获批执行（§12）：Polymarket 内部链路 PASS；Binance 内部 WS 60 s 无消息 FAIL（#6） |
| S12 | 连续运行观察 | **PASS（空闲进程）** | 15 分 17 秒，15/15 健康，0 重启 |
| S13 | 容器正常停止 | **PASS** | ExitCode=0，优雅退出日志 |
| S14 | 相同镜像重新启动 | **PASS** | 新容器启动、健康、查库正常 |
| S15 | 资金、凭证及生产数据安全 | **PASS** | §15 |

合计（共 15 项）：**PASS 13**（S01–S07、S09、S10、S12–S15；其中 S09、S12 带限定条件）· **FAIL 1**（S08）· **PARTIAL 1**（S11）· 未执行 0。
PoC 整体状态：**PARTIAL**。

---

## 17. A01–A07 架构观察

| ID | 观察项 | 结论 | 依据 |
|---|---|---|---|
| A01 | 应用是否支持无私钥启动 | **支持（实测）** | §9 |
| A02 | 公开行情是否依赖交易认证 | **不依赖认证，但依赖策略启动** | 模拟模式用 dummy key 构造只读客户端（`api/strategy.go:235-245`）；外部行情接口全部匿名可用（§11） |
| A03 | 行情与交易模块是否强耦合 | **存在（实测确认）** | 15 分钟空闲运行无任何行情组件启动；经 `StartSimulation` 拉起后行情组件才出现（§12） |
| A04 | 是否需要独立测试钱包 | **不需要** | 本轮未生成任何钱包 |
| A05 | SQLite 驱动及部署方式 | `modernc.org/sqlite` v1.41.0，纯 Go，进程内嵌（实测可用） | §7、§9 |
| A06 | 原项目 API 是否存在兼容性问题 | **存在（实测）** | P-12 Polymarket 搜索漂移；P-13 Binance 合约 WS 路径漂移；P-15 结算口径变化 |
| A07 | 实验配置是否与镜像正确分离 | **是（实测）** | 镜像内无任何 `.env*`；ENV 仅安全默认值；凭证全部运行时注入 |

---

## 18. 安全边界

### 18.1 凭证泄露（已由负责人决策关闭，记录留档）

> **处置结论（2026-11-28，项目负责人）**：已与原作者确认，`BUILDER_ADDRESS` 对应钱包余额
> 低于 1 美元且原作者已弃用该账号，本议题**不再跟进**，不作为 Issue #3 的阻塞项。
> 以下内容作为审计留档保留，不要求本阶段修复。
>
> **R06 补充**：该决策只表示“不作为 #3 的阻塞项”。凭证已公开且仍可能有效，风险不因钱包余额低而消失（§20.4）。

提示词 §8.1 的假设与实测不符，**实际情况更需要处理**：

| 字段 | `.env.example` 实测状态 | 是否在 `origin/main` |
|---|---|---|
| `POLYMARKET_PRIVATE_KEY` | **空值** | 空值 |
| `TELEGRAM_BOT_TOKEN` | 空值 | 空值 |
| `BUILDER_KEY` | **已填充**（36 字符，UUID 形态） | **是** |
| `BUILDER_SECRET` | **已填充**（44 字符，base64-32B 形态） | **是** |
| `BUILDER_PASSPHRASE` | **已填充**（64 字符） | **是** |
| `BUILDER_ADDRESS` | **已填充**（42 字符，`0x` 地址形态） | **是** |

叠加两个事实后性质才清楚：

1. 本仓库是**公开仓库**（匿名 `git ls-remote` 成功，见 evidence/00）。
2. 这组 `BUILDER_*` 在代码里的用途是构造 `src.Redeemer`
   （`api/strategy.go:212-231`），经 `relayer-v2.polymarket.com` 执行**代理钱包赎回**，
   属链上资金操作路径，不是只读行情凭证。

本次会话对这些值的处理：**未读取其内容、未输出、未复制进任何实验配置、
未写入镜像、未写入日志、未用于任何认证或签名**。只检测了「是否非空」与长度形态。

原建议（现已由上述决策覆盖，仅留档）：吊销 / 轮换凭证、核查钱包资产、清空 `.env.example`、
通知原作者、加 secret scanning。负责人已确认资金风险可忽略，故不执行。

仍然生效的一条工程约束：**本次实验不使用 `.env.example` 中的任何值**，
`.dockerignore` 继续把 `.env.example` 排除在构建上下文之外。

`.dockerignore` 已把 `.env.example` 排除在构建上下文之外，但那只防镜像，不防仓库。

### 18.2 自动交易路径清点（§538：不要只依赖单个环境变量）

共三条路径可能导致交易行为，`ENABLE_PATTERN_STRATEGY=false` **只能堵住第一条**：

| # | 路径 | 触发条件 | 本次的阻断手段 |
|---|---|---|---|
| 1 | 进程启动自动拉起 Pattern 策略（`main.go:184-201`） | `ENABLE_PATTERN_STRATEGY != "false"`（**代码默认 `"true"`**）且 `PATTERN_CONFIG_FILE` 存在 | 显式 `ENABLE_PATTERN_STRATEGY=false`，并且 `PATTERN_CONFIG_FILE` 留空（默认指向仓库中不存在的 `strategy_config_3.yaml`） |
| 2 | HTTP `POST /api/v1/sim-copy-trading` → `StartSimulation` → 可能初始化真实下单客户端 | 操作码正确 **且** 某配置组 `TradingMode=="real"` | `TRADING_MODE=simulation`（`TradingMode` 只取自环境变量），因此即使调用也只进入模拟分支（dummy key 只读客户端，`api/strategy.go:233-245`）。普通启动测试中不调用；S11 经负责人批准调用一次，日志确认 `已初始化只读 ClobClient (用于模拟盘订单簿验证)`、未出现 `真实交易模式已启用` |
| 3 | `StartSimulation` 内部二次拉起 Pattern 策略（`api/strategy.go:279-304`） | `ENABLE_PATTERN_STRATEGY == "true"` 且配置文件存在 | 同 #1 |

真实下单与链上写入的实际闸门有三道，必须同时成立才会发生：
`TradingMode=="real"` **且** `POLYMARKET_PRIVATE_KEY` 非空 **且**（赎回路径还需）`BUILDER_*` 非空。
本次实验三者全部不满足。

另外注意 `api/strategy.go:226-230`：real 模式下 `redeemer.Start()` 会启动**定时自动赎回**
（决议后 N 秒自动执行链上赎回）。这是一条不需要人工触发的链上写入路径。

### 18.3 其它安全观察（与上线目标相关，非本次任务范围）

1. **操作码硬编码在源码里**（`api/auth.go:10`，明文常量，公开仓库可见），
   且是全部写入型接口的唯一鉴权手段。建议改为环境变量注入并轮换。
2. **`DELETE /api/v1/trades` 没有操作码保护**（`api/routes.go:27` 直接注册
   `DeleteTradesHandler`，未包 `RequireOperationCode`）。handler 只校验方法为 DELETE
   （`api/data_management.go:13`），所以浏览器 GET 不会误删；但任何能访问该端口的客户端
   都能一次请求清空全部交易记录。
3. **`HOST` 默认 `0.0.0.0`**（`main.go:156`）。与第 1、2 点叠加，等于在局域网上暴露了
   一个用公开已知操作码保护的控制面。本次实验用 `-p 127.0.0.1:8090:8090` 限制暴露面，
   但这是实验层的缓解，不是代码层的修复。
4. **反 Cloudflare 指纹伪装**：`src/clob_order.go:194-195,514-516` 用 CycleTLS 伪装
   Chrome 120 的 JA3 指纹，`src/utls_dialer.go` 用 uTLS 伪装 `HelloChrome_Auto`，
   注释明确写「绕过 Cloudflare TLS 指纹检测」。这不是行情读取所必需（它在下单路径上），
   但属于规避服务商机器人检测的手段，对「最终上线」是实质性的 ToS / 合规风险，
   建议作为独立议题评估。本次未通过代理绕过任何区域访问限制（§588）。


### 18.4 本轮边界遵守情况

按阶段区分：

| 阶段 | 启动 / 停止 / 写入型 API | 交易 |
|---|---|---|
| S01–S10、S12–S15 普通启动测试（证据 `01`–`06`） | 未调用任何策略启动、停止或写入型接口；仅访问 `/health`、`/`、`GET /api/v1/config`、`GET /api/v1/sim-stats` | 无 |
| S11 获批验证（证据 `07`） | 经负责人批准，调用 `POST /api/v1/sim-copy-trading?type=2` 与 `POST /api/v1/sim-stop-trading` 各一次（另有一次无操作码请求被 401 拒绝） | 主策略关闭，仅维护行情状态；**0 笔模拟成交**；无真实交易 |

整个实验期间统一成立的事实：

- 经项目负责人批准，执行过且仅执行过**一次**模拟启停（S11）。
- 主交易策略关闭（`ENABLE_MAIN_STRATEGY=false`），Pattern 策略关闭，仅维护市场行情状态。
- 未产生任何模拟成交（`sim-stats` 始终 trade_count=0）。
- 未执行真实交易：无真实订单提交或撤销、无资金转移、无赎回、无链上授权。
- 未使用真实私钥及真实交易凭证（所有凭证类环境变量为空）。S11 使用的是源码中硬编码的应用操作码
  （`api/auth.go:10`，公开仓库可见，P-05），仅作为 HTTP 头传入，未写入任何证据或报告。
- 未访问生产数据库（仅容器内临时 SQLite）。
- `DELETE /api/v1/trades`、`PUT /api/v1/config`、`POST /api/v1/config/reload` 从未调用。

同样未执行：业务代码修改、依赖升级、修改 Docker Desktop 设置、特权容器、挂载宿主目录或 Docker socket、
通过代理绕过区域限制、`git clean` / `git reset --hard`、强制删除分支、删除远端分支、改写历史、强制推送、
自动合并 PR、关闭 Issue。

---

## 19. 问题登记

| ID | 类别 | 已观察事实 | 根因假设（未验证） | 最小修复建议 | 需审批 | 状态 |
|---|---|---|---|---|---|---|
| P-01 / P-02 / P-03 | ENV / NETWORK / DELIVERY | 上一轮沙箱会话无 Docker daemon、出口受限、无法 push | — | 改用宿主机原生 CLI | — | **CLOSED**（本轮在原生通道实测通过） |
| P-04 | SAFETY | `.env.example` 含原作者 `BUILDER_*` | — | — | 已决策 | **CLOSED**（负责人 2026-11-28 决策，§18.1） |
| P-05 | AUTH | 操作码明文硬编码 `api/auth.go:10` | 早期单机自用 | 环境变量注入 + 轮换 | 是 | OPEN（不在 #3 范围） |
| P-06 | SAFETY | `DELETE /api/v1/trades` 无操作码保护（`api/routes.go:27`） | 注册时遗漏中间件 | 用 `RequireOperationCode` 包裹 | 是 | OPEN（不在 #3 范围） |
| P-07 | API | Polymarket 发现逻辑的 4 条假设 | — | — | — | **已由实测替代**：slug 结构 / 前缀 / `15m` 子串 / outcomes 仍成立；`public-search` 不成立 → P-12 |
| P-08 | BUILD | CGO 是否必需 | go-ethereum 有纯 Go 回退 | `CGO_ENABLED=0` 对照构建 | 否 | UNVERIFIED（本轮 `CGO_ENABLED=1` 构建成功） |
| P-09 | CONFIG | Go 版本与 SHA256 | — | — | — | **CLOSED**（本轮再次独立核对 go.dev 一致） |
| P-10 | CONFIG | `.env.example` 与代码字段双向偏差（§8.4） | 文档未随代码更新 | 同步字段清单 | 否 | OPEN |
| P-11 | BUILD | Go 1.27.4 vs go.mod `go 1.24.1` | — | — | — | **CLOSED**（编译成功） |
| **P-12** | API | `public-search q="<sym> updown 15m"&events_status=active` 返回 200 但不含任何 `*-updown-15m-*` 事件；`/markets?slug=<构造 slug>` 正常 | Polymarket 搜索索引不再收录短周期循环市场 | 发现逻辑改为直接按 slug 规则查询（即把现有回退升为主路径） | 是（业务代码） | OPEN → **Issue #7** |
| **P-13** | API | `wss://fstream.binance.com/stream?streams=...` 101 但 30 s 0 消息；`/market/stream?streams=...` 338 条 | Binance USDⓈ-M 合约 WS 按 market/public/private 分路径，旧路径不再推送行情 | `src/binance_ws.go:16` 改为 `/market/stream?streams=`，并补一个“连上但无数据”的告警 | 是（业务代码） | OPEN → **Issue #6**（S11 已在应用内复现） |
| **P-14** | BUILD-ENV | 第 1 次构建卡在 `resolve image config for docker/dockerfile:1` 704 s 后 `lease does not exist` | 与构建期间并发且被中断的 `docker pull` 有关，或 Docker Hub 瞬时故障 | 无需改动；若复现，可预先 `docker pull docker/dockerfile:1` 或去掉 `# syntax=` 行（Dockerfile 未用到新语法特性） | 否 | OBSERVED，重试即通过 |
| **P-15** | DOMAIN | 15m 市场描述的结算来源为 Chainlink BTC/USD TWAP | — | 后续阶段评估策略参考价与结算口径是否一致 | — | OPEN（不在 #3 范围） |
| **P-17** | PERF | S11 中 Polymarket 市场 WS 订阅 8 个资产约 13 s 后被服务端 `close 1013: slow consumer: send buffer full` 断开，自动重连成功 | **假设（未验证）**：读取循环与回调处理串行，处理速度跟不上推送量 | 读循环与处理解耦（带缓冲通道）；先做长时间观察量化断连频率 | 是（业务代码） | OPEN → **Issue #8**（后续独立性能研究，不在 #3 范围） |
| **P-16** | EVIDENCE | `05b-isolation.txt` 的 `ENABLE_*` grep 正则错误 | 本人脚本失误 | 已在 §15 用其它证据补足 | 否 | NOTED |

---

## 20. 下一步

### 20.1 状态与待决策

1. ~~S11 授权~~：已批准并执行（§12）。
2. ~~P-12 / P-13 立项~~：已提交并获批 Issue #6 / #7。顺序（Issue #3 决策）：PR #5 合并后先实施 #6，验收后实施 #7，各自独立 PR 与回归审查；两项完成后重跑完整 Smoke Test（含带行情负载的持续观察）。
3. ~~PR #4 与 PR #5 的合并顺序~~：PR #4 已关闭为被 #5 取代；PR #5 待独立审查后由负责人决定合并。
4. ~~P-17 立项~~：已登记 Issue #8，作为后续独立性能研究，不在本轮修复范围。

### 20.2 复现命令与自动化覆盖范围

**重要：** `evidence/` 根目录下的 `00`–`07` 文件由**旧版脚本**（提交 `8ee6e41` 时的版本）及手工命令生成，属历史实测证据，
**保持原样不改**。下方是按审查 R04 / R05 修复后的**新版脚本**，尚未用它重跑完整实验（本轮不要求重跑）；
新版脚本只用 mock Docker 测试和一次真实 `preflight` 做过验证（§20.3）。

```bash
export RUN_ID=$(date -u +%Y%m%dT%H%M%SZ)          # 一次实验一个 RUN_ID
bash scripts/poc/day0-docker-smoke.sh preflight   # 主机 / Git / 依赖摘要 / Docker
bash scripts/poc/day0-docker-smoke.sh build       # S01–S02；失败即停止
bash scripts/poc/day0-docker-smoke.sh offline     # S03–S06（--network none）
bash scripts/poc/day0-docker-smoke.sh safety offline   # S15（凭证只报告 EMPTY/SET/UNSET）
bash scripts/poc/day0-docker-smoke.sh online      # 联网启动 + 宿主机侧诊断探测
bash scripts/poc/day0-docker-smoke.sh probe 30    # S07–S10 容器网络内探测
bash scripts/poc/day0-docker-smoke.sh observe 15  # S12（空闲进程）
bash scripts/poc/day0-docker-smoke.sh lifecycle   # S13–S14
bash scripts/poc/day0-docker-smoke.sh safety      # S15（联网容器）
bash scripts/poc/day0-docker-smoke.sh teardown    # 停止本轮容器（保留）
bash scripts/poc/day0-docker-smoke.sh cleanup     # 删除本轮容器（校验归属）
# 或一次性执行上述全部步骤：bash scripts/poc/day0-docker-smoke.sh all
```

| 范围 | 状态 |
|---|---|
| S01–S10、S12–S15 | 脚本自动化 |
| **S11**（获批模拟启停） | **未自动化**：本次手工执行（证据 `07`）；需要操作码与负责人逐次批准，刻意不放进脚本 |
| S12 | 仅空闲进程（不启动策略）；**不覆盖带行情负载的持续观察** |
| S08 / S10 WS | `probe` 统计的是 WebSocket **文本帧数**，不校验是否为有效市场消息 |
| S09 | `probe` 的回退探测走 `/events?slug=`，应用实际走 `/markets?slug=`；后者由 `evidence/03c` 单独覆盖（手工命令） |

新版脚本的行为（R04 / R05）：

| 规则 | 实现 |
|---|---|
| 退出码传播 | `run` 返回被执行命令的真实退出码；`must` 失败即终止当前步骤；`step` 经 `PIPESTATUS` 把 `tee` 前的退出码传出；诊断性预期失败用 `diag` 单独记录、不传播 |
| 构建失败即停止 | `build` 失败立即以 docker build 的退出码退出，不检查任何镜像 |
| 不误用旧镜像 | 用 `--iidfile` 记录**本轮**构建的镜像 ID；`offline` / `online` / `lifecycle` / `probe` 只使用该 ID，缺失即以退出码 3 拒绝，不回退到已有标签 |
| 证据隔离 | 默认写入 `evidence/runs/<RUN_ID>/`；目标文件已存在则拒绝（退出码 3）；拒绝写入 `evidence/` 根目录；`RUN_ID` 只允许 `[A-Za-z0-9_.-]` |
| 容器归属 | 所有实验容器带 label `org.polymarket-lab.experiment=m0-day0-smoke` 与 `org.polymarket-lab.run=<RUN_ID>`；`stop` / `rm` / `exec` 前必须两者都匹配，否则拒绝（退出码 3） |
| 凭证检查 | 只输出 `NAME=EMPTY` / `NAME=SET (value withheld)` / `NAME=UNSET`，任一非空即断言失败 |
| 断言 | `/health`、`sim-stats`、Web UI、config 以 HTTP 200 判定；运行状态、端口仅 127.0.0.1、非特权、无挂载、无 socket、uid、交易开关均为断言，任一失败步骤非零退出 |

退出码：0 全部通过 · 1 断言 / 关键命令失败 · 2 用法错误 · 3 安全拒绝。

### 20.3 脚本测试（R04 / R05）

`bash scripts/poc/tests/day0-smoke.test.sh`：使用临时目录里的 mock `docker`，不触碰真实 Docker、不联网、
不写入 `evidence/`。2026-11-28 在本机（macOS 自带 bash 3.2.57）运行结果：**57 pass / 0 fail，退出码 0**。

| 组 | 覆盖 |
|---|---|
| T1 | `bash -n` 两个脚本 |
| T2 | `run` / `must` / `diag` / `tee` 管道 / `step` 的退出码（1、7、0、5、诊断 0、4、6） |
| T3 | mock 构建返回 17 → 脚本退出 17，证据记录 `BUILD FAILED` 与 `[exit=17]`，无 `image.iid`，之后无 `docker image inspect` / `docker run` |
| T4 | 同一 RUN_ID 构建失败后，`offline` / `online` / `lifecycle` / `probe` 均退出 3 且不启动任何容器 |
| T5 | 构建成功 → `image.iid` 为本轮新 ID，按 ID 而非标签检查 |
| T6 | 同一 RUN_ID 重复写同一证据 → 退出 3 且原文件摘要不变；指向 `evidence/` 根目录 → 3；含 `/` 的 RUN_ID → 2；无子命令 → 2 |
| T7 | 外来 label / 其它 RUN_ID / 同名外来容器 → 退出 3，无 `stop` / `rm` / `run`；本轮容器 → 正常 `stop` |
| T8 | 凭证为假秘密值时 safety 退出 1，输出 `SET (value withheld)`，证据与标准输出均不含该值；凭证为空时退出 0；外来容器 → 3 且不 `exec` |
| T9 | `evidence/*.txt` 历史证据摘要前后一致 |

补充验证：
- `shellcheck -S warning`（0.9.0，一次性 `ubuntu:24.04` 容器内运行，只读挂载 `scripts/poc`）：0 告警，退出码 0。
- 真实 Docker：`preflight` 退出 0；随后同一 RUN_ID 执行 `offline`，本机仍有旧标签 `polymarket-engineering-lab:day0`，
  脚本以退出码 3 拒绝，未创建任何容器。（证据写到临时目录，未提交。）
- 修复过程中发现并修正的真实缺陷：bash 3.2 会把紧跟在 `$f` 后的全角字符当作变量名的一部分，导致 `unbound variable`
  并以 1 而不是 3 退出；已统一改为 `${var}`。

### 20.4 R06 记录（非阻塞，留待最终复测 / ADR-001）

| 项 | 记录 |
|---|---|
| WS 计数口径 | `probe` 与报告中的 S08 / S10 消息数是**帧数**，未区分心跳、订阅确认与有效行情；最终复测需按事件类型断言有效消息 |
| S09 探测路径 | `wsprobe` 回退用 `/events?slug=`，应用用 `/markets?slug=`，两者不完全相同；应用路径由 `03c` 证据覆盖。最终复测应统一为应用路径 |
| S11 / S12 | S11 为手工执行；S12 仅空闲进程。Issue #6 / #7 完成后的完整 Smoke Test 需包含带行情负载的持续观察 |
| 镜像可复现性 | `ubuntu:24.04` 与 BuildKit 前端 `docker/dockerfile:1` 均为可变标签，未固定 digest；留 ADR-001 决定是否按 digest 固定 |
| 凭证风险 | §18.1 的 `.env.example` 凭证泄露仍是真实风险：负责人决策的是“不作为 #3 阻塞项”，低余额不等于风险消失（凭证仍可用于赎回 / relayer 路径，且已公开） |
| P-17 | slow consumer 的根因（读取与处理串行）仍是**假设**，未经实测验证，交由 Issue #8 研究 |

### 20.5 历史证据清单（`reports/engineering/poc/evidence/`，旧版脚本与手工命令生成，不改动）

| 文件 | 内容 |
|---|---|
| `00-execution-channel.txt` | 上一轮沙箱会话记录（历史，已被取代） |
| `01-build-attempt1.txt` | 第 1 次构建（FAIL） |
| `01-build.txt` | 第 2 次构建（PASS）完整日志 |
| `01b-image-identity.txt` | 镜像 ID / 架构 / ENV / 内容检查 / Go 版本 / 依赖摘要 |
| `02-offline-start.txt` | S03–S06、凭证检查 |
| `03-online-market-data.txt` | 联网启动 + 宿主机侧探测 |
| `03b-container-probe.txt` | 容器网络内 S07–S10 探测 |
| `03c-discovery-fallback.txt` | Polymarket 回退路径 4 币种核查 |
| `04-observe-15min.txt` | S12 |
| `05-lifecycle.txt` | S13 / S14 |
| `05b-isolation.txt` | S15 隔离证据 |
| `06-teardown.txt` | 收尾 |
| `07-s11-app-integration.txt` | S11 获批模拟启停（操作码已脱敏） |

证据仅含公开行情数据与本地命令输出，不含凭证。

---

## 21. 诚实声明

- 所有 PASS 均对应 `evidence/` 中的真实命令输出；镜像 ID、时间、消息数均为实测值。
- 第 1 次构建失败如实记录，未删除日志；其根因只是假设。
- S09 的 PASS 明确限定为“经回退路径、外部层”；S12 的 PASS 明确限定为“空闲进程”。
- 外部接口可达不等于应用功能可用：S11 单独在应用内部验证，结果按 Polymarket / Binance 分别记录。
- 未修改业务代码、go.mod、go.sum；未为了让矩阵变绿而绕过任何限制。
- 2026-11-28 按审查意见 R01–R03 只修正了报告措辞与统计，`evidence/` 下的原始实验输出未作任何修改。
- 按 R04 / R05 修复的是复现脚本；修复后的脚本未用于重跑实验，历史证据仍来自旧版脚本（§20.2）。
