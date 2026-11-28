# syntax=docker/dockerfile:1
# =============================================================================
# M0 Day 0 PoC — Ubuntu 24.04 / ARM64 smoke-test image
# Refs: Issue #3
#
# 目的：证明现有 Go 应用可以在 Ubuntu 容器中构建、启动并处理真实公开行情。
# 本镜像不包含任何真实凭证。所有配置通过容器运行时注入。
#
# 构建（在 macOS 宿主机终端执行）：
#   docker build --platform linux/arm64 -t polymarket-engineering-lab:day0 .
#
# Go 版本与官方 SHA256 已固定在下方 ARG 中，无需再传 --build-arg。
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: builder
# -----------------------------------------------------------------------------
FROM ubuntu:24.04 AS builder

# Go 工具链版本 —— 由项目负责人核实并固定 (2026-11-28)。
# 来源: https://go.dev/dl/?mode=json
# 下载: https://go.dev/dl/go1.27.4.linux-arm64.tar.gz
# 满足 go.mod 的 `go 1.24.1` 下限 (go.mod:3)。
ARG GO_VERSION=1.27.4
# 官方 SHA256 (go1.27.4.linux-arm64.tar.gz)。
# 强制校验：校验不通过或留空，构建立即失败，绝不解压安装。
ARG GO_SHA256=3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92
# 该校验和只对应 linux-arm64。构建其它架构必须同时覆盖 GO_SHA256。
ARG GO_EXPECTED_ARCH=arm64
ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive

# 最小系统依赖：
#   ca-certificates  -> 访问 go 模块代理 / Binance / Polymarket 的 TLS 根证书
#   curl             -> 下载 Go 工具链
#   build-essential  -> cgo。go-ethereum 依赖链中包含 cgo 包
#                       (c-kzg-4844, blst)，首轮 PoC 不赌纯 Go 回退路径。
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      build-essential \
 && rm -rf /var/lib/apt/lists/*

# 安装官方 Go 工具链（不使用 apt 的 golang 包：Ubuntu 24.04 仓库版本低于 go.mod 要求）
# 顺序严格为：下载 -> 校验 -> 通过后才解压。校验失败即构建失败。
RUN set -eux; \
    if [ -z "${GO_SHA256}" ]; then \
      echo "[go-toolchain] FATAL: GO_SHA256 为空，拒绝在未校验的情况下安装工具链。" >&2; exit 1; \
    fi; \
    arch="${TARGETARCH:-arm64}"; \
    if [ "${arch}" != "${GO_EXPECTED_ARCH}" ]; then \
      echo "[go-toolchain] FATAL: 目标架构 ${arch} 与内置校验和的架构 ${GO_EXPECTED_ARCH} 不一致。" >&2; \
      echo "[go-toolchain] 请用 --build-arg GO_SHA256=<对应架构的官方校验和> --build-arg GO_EXPECTED_ARCH=${arch} 覆盖。" >&2; \
      exit 1; \
    fi; \
    tarball="go${GO_VERSION}.linux-${arch}.tar.gz"; \
    url="https://dl.google.com/go/${tarball}"; \
    echo "[go-toolchain] version : go${GO_VERSION}"; \
    echo "[go-toolchain] source  : ${url}"; \
    echo "[go-toolchain] expected: ${GO_SHA256}"; \
    curl -fsSL -o "/tmp/${tarball}" "${url}"; \
    actual="$(sha256sum "/tmp/${tarball}" | awk '{print $1}')"; \
    echo "[go-toolchain] actual  : ${actual}"; \
    echo "${GO_SHA256}  /tmp/${tarball}" | sha256sum -c -; \
    echo "[go-toolchain] checksum VERIFIED -> 允许解压"; \
    tar -C /usr/local -xzf "/tmp/${tarball}"; \
    rm -f "/tmp/${tarball}"; \
    /usr/local/go/bin/go version

ENV PATH=/usr/local/go/bin:$PATH \
    GOTOOLCHAIN=local \
    GOFLAGS=-mod=readonly \
    CGO_ENABLED=1

WORKDIR /src

# 依赖层：先只拷贝 go.mod / go.sum，利用层缓存
COPY go.mod go.sum ./
# 记录依赖声明的摘要，用于构建前后比对（本次任务不得修改依赖声明）
RUN set -eux; \
    sha256sum go.mod go.sum | tee /src/.gomod-sha256-before.txt; \
    go mod download

# 源码层
COPY . .

# 校验依赖声明未被构建过程改动
RUN set -eux; \
    sha256sum -c /src/.gomod-sha256-before.txt; \
    echo "[deps] go.mod / go.sum unchanged"

# 编译根应用（仓库内还有 src/cmd 等其它 main 包，此处只构建根包）
RUN set -eux; \
    go env GOVERSION GOARCH GOOS CGO_ENABLED; \
    go build -buildvcs=false -trimpath -o /out/polymarket-tool . ; \
    ls -l /out/polymarket-tool; \
    file /out/polymarket-tool 2>/dev/null || true

# -----------------------------------------------------------------------------
# Stage 2: runtime
# -----------------------------------------------------------------------------
FROM ubuntu:24.04 AS runtime

ENV DEBIAN_FRONTEND=noninteractive

# 运行期只需要 TLS 根证书；sqlite3 CLI 不是运行必需（驱动为纯 Go 的 modernc.org/sqlite）
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd --gid 10001 app \
 && useradd --uid 10001 --gid 10001 --create-home --shell /usr/sbin/nologin app

WORKDIR /app

COPY --from=builder /out/polymarket-tool /app/polymarket-tool

# 运行所需静态资源（main.go 以相对路径 front/index.html、front/api-test.html 提供）
COPY --chown=app:app front/ /app/front/

# 仅携带模拟盘配置模板。*_real.yaml 刻意不入镜像（纵深防御）。
# 注意：这些文件默认不生效 —— 只有显式设置 CONFIG_FILE / PATTERN_CONFIG_FILE 才会被加载。
COPY --chown=app:app strategy_config_sim.yaml   /app/strategy_config_sim.yaml
COPY --chown=app:app strategy_config_3_sim.yaml /app/strategy_config_3_sim.yaml

# 数据与日志目录（internal/database 会 MkdirAll，但预建并移交属主以确保非 root 可写）
RUN mkdir -p /app/data /app/logs && chown -R app:app /app

USER app

# main.go: HOST 默认 0.0.0.0 / PORT 默认 8090
EXPOSE 8090

# 实验安全默认值。运行时可覆盖，但默认即为"无凭证、不自动交易"。
ENV DB_PATH=/app/data/trades.db \
    BOT_DB_PATH=/app/data/bot.db \
    HOST=0.0.0.0 \
    PORT=8090 \
    TRADING_MODE=simulation \
    ENABLE_MAIN_STRATEGY=false \
    ENABLE_PATTERN_STRATEGY=false \
    SNIPER_MODE=false

ENTRYPOINT ["/app/polymarket-tool"]
