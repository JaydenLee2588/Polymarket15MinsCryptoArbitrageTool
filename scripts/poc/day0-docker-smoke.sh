#!/usr/bin/env bash
# =============================================================================
# M0 Day 0 — Docker Smoke Test 执行脚本 (Refs #3)
#
# 在 macOS 宿主机终端运行（Docker Desktop 必须已启动）：
#   cd <repo>
#   bash scripts/poc/day0-docker-smoke.sh preflight
#   bash scripts/poc/day0-docker-smoke.sh build
#   bash scripts/poc/day0-docker-smoke.sh offline
#   bash scripts/poc/day0-docker-smoke.sh online
#   bash scripts/poc/day0-docker-smoke.sh probe 30
#   bash scripts/poc/day0-docker-smoke.sh observe 20
#   bash scripts/poc/day0-docker-smoke.sh lifecycle
#   bash scripts/poc/day0-docker-smoke.sh teardown
#
# 所有证据写入 reports/engineering/poc/evidence/*.txt
# （用 .txt 而非 .log，因为 .gitignore 含 *.log 规则）
#
# 安全边界（脚本强制，勿删）：
#   - 不加载 .env / .env.example
#   - 不传入任何私钥或交易凭证
#   - TRADING_MODE=simulation / ENABLE_MAIN_STRATEGY=false / ENABLE_PATTERN_STRATEGY=false
#   - 端口只绑定 127.0.0.1
#   - 不挂载宿主目录、不挂载 docker socket、不使用 --privileged
#   - 不调用任何写入型接口（DELETE /api/v1/trades、PUT /api/v1/config、
#     /api/v1/sim-copy-trading 等一律不碰）
# =============================================================================
set -uo pipefail

IMAGE="${IMAGE:-polymarket-engineering-lab:day0}"
CONTAINER="${CONTAINER:-polymarket-lab-day0}"
PLATFORM="${PLATFORM:-linux/arm64}"
GO_VERSION="${GO_VERSION:-1.27.4}"
GO_SHA256="${GO_SHA256:-3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92}"
HOST_PORT="${HOST_PORT:-8090}"
EV="reports/engineering/poc/evidence"
mkdir -p "$EV"

ts()  { date -u +%Y-%m-%dT%H:%M:%SZ; }
hdr() { echo; echo "===== $* ($(ts)) ====="; }
run() { echo "\$ $*"; "$@" 2>&1; echo "[exit=$?]"; }

need_docker() {
  if ! docker version >/dev/null 2>&1; then
    echo "FATAL: Docker daemon 不可达。请先启动 Docker Desktop。" >&2
    exit 1
  fi
}

# --- 本次实验的运行时环境（唯一来源，勿分叉） --------------------------------
# 注意：用普通数组而非 mapfile，以兼容 macOS 自带的 bash 3.2
LAB_ENV=(
  -e TRADING_MODE=simulation
  -e ENABLE_MAIN_STRATEGY=false
  -e ENABLE_PATTERN_STRATEGY=false
  -e SNIPER_MODE=false
  -e CONFIG_FILE=
  -e PATTERN_CONFIG_FILE=
  -e TELEGRAM_BOT_TOKEN=
  -e POLYMARKET_PRIVATE_KEY=
  -e PROXY_WALLET_ADDRESS=
  -e MAKER_ADDRESS=
  -e POLY_API_KEY=
  -e POLY_API_SECRET=
  -e POLY_PASSPHRASE=
  -e POLY_EOA_ADDRESS=
  -e BUILDER_KEY=
  -e BUILDER_SECRET=
  -e BUILDER_PASSPHRASE=
  -e DB_PATH=/app/data/trades.db
  -e BOT_DB_PATH=/app/data/bot.db
)

# =============================================================================
cmd_preflight() {
  local out="$EV/00-preflight.txt"
  {
    hdr "HOST"
    run uname -a
    run sw_vers
    hdr "GIT BASELINE"
    run git rev-parse --show-toplevel
    run git branch --show-current
    run git rev-parse HEAD
    run git status --short
    hdr "DEPENDENCY DECLARATION DIGEST (before build)"
    run shasum -a 256 go.mod go.sum
    hdr "DOCKER"
    run docker version
    run docker context show
    run docker info
  } | tee "$out"
  echo "-> $out"
}

cmd_build() {
  need_docker
  local out="$EV/01-build.txt"
  # Go 版本与官方 SHA256 已在 Dockerfile 中固定；此处显式再传一次，
  # 让构建日志把实际使用的值记录下来（Evidence First）。
  local args=( build --platform "$PLATFORM" --progress=plain
               --build-arg "GO_VERSION=$GO_VERSION"
               --build-arg "GO_SHA256=$GO_SHA256"
               -t "$IMAGE" . )
  {
    hdr "BUILD COMMAND"
    echo "docker ${args[*]}"
    echo "GO_VERSION=$GO_VERSION"
    echo "GO_SHA256=$GO_SHA256"
    echo "期望在下方日志中看到: [go-toolchain] checksum VERIFIED -> 允许解压"
    hdr "BUILD START"
    local t0 t1 rc
    t0=$(date +%s)
    docker "${args[@]}" 2>&1
    rc=$?
    t1=$(date +%s)
    echo "[build exit=$rc elapsed=$((t1-t0))s]"
    if [ "$rc" -ne 0 ]; then
      echo "BUILD FAILED -> 后续步骤不要继续，把本文件内容贴回去做诊断。"
    fi
    hdr "TOOLCHAIN INTEGRITY CHECK (grep from build log above)"
    echo "（人工确认：checksum VERIFIED 出现过，且没有出现 FATAL）"
    hdr "IMAGE IDENTITY"
    run docker image inspect "$IMAGE" --format '{{.Id}}'
    run docker image inspect "$IMAGE" --format 'Arch={{.Architecture}} OS={{.Os}} Variant={{.Variant}}'
    run docker image inspect "$IMAGE" --format 'Size={{.Size}} Created={{.Created}}'
    hdr "BASE IMAGE / ARCH INSIDE IMAGE (确认 arm64 原生，非 AMD64 模拟)"
    run docker run --rm --platform "$PLATFORM" --entrypoint /bin/sh "$IMAGE" -c 'head -3 /etc/os-release; echo "---"; id; echo "---"; uname -m'
    hdr "GO VERSION USED IN BUILDER STAGE"
    run docker build --platform "$PLATFORM" --target builder -t "$IMAGE-builder" .
    run docker run --rm --platform "$PLATFORM" --entrypoint /usr/local/go/bin/go "$IMAGE-builder" version
    hdr "DEPENDENCY DECLARATION DIGEST (after build)"
    run shasum -a 256 go.mod go.sum
    echo "必须与 00-preflight.txt 中 build 前的摘要完全一致。"
  } | tee "$out"
  echo "-> $out"
}

# S03/S04/S05/S06 —— 无外部网络启动验证
cmd_offline() {
  need_docker
  local out="$EV/02-offline-start.txt"
  docker rm -f "$CONTAINER-offline" >/dev/null 2>&1
  {
    hdr "START WITH --network none (no external network, no creds)"
    run docker run -d --name "$CONTAINER-offline" --platform "$PLATFORM" \
        --network none "${LAB_ENV[@]}" "$IMAGE"
    sleep 8
    hdr "CONTAINER STATE"
    run docker ps -a --filter "name=$CONTAINER-offline" --format '{{.Names}} {{.Status}} {{.State}}'
    run docker inspect "$CONTAINER-offline" --format 'Running={{.State.Running}} ExitCode={{.State.ExitCode}} OOM={{.State.OOMKilled}} Restarts={{.RestartCount}}'
    hdr "APPLICATION LOG"
    run docker logs "$CONTAINER-offline"
    hdr "S04 SQLITE FILES ON DISK"
    run docker exec "$CONTAINER-offline" ls -la /app/data /app/logs
    hdr "S05 HEALTH ENDPOINT (in-container)"
    run docker exec "$CONTAINER-offline" bash -c 'exec 3<>/dev/tcp/127.0.0.1/8090 || { echo "tcp connect failed"; exit 1; }; printf "GET /health HTTP/1.0\r\nHost: localhost\r\n\r\n" >&3; cat <&3'
    hdr "S04 SQLITE FUNCTIONAL READ via /api/v1/sim-stats (read-only, queries DB)"
    run docker exec "$CONTAINER-offline" bash -c 'exec 3<>/dev/tcp/127.0.0.1/8090; printf "GET /api/v1/sim-stats HTTP/1.0\r\nHost: localhost\r\n\r\n" >&3; cat <&3'
    hdr "S06 WEB UI + READ-ONLY CONFIG API"
    run docker exec "$CONTAINER-offline" bash -c 'exec 3<>/dev/tcp/127.0.0.1/8090; printf "GET / HTTP/1.0\r\nHost: localhost\r\n\r\n" >&3; head -c 400 <&3'
    run docker exec "$CONTAINER-offline" bash -c 'exec 3<>/dev/tcp/127.0.0.1/8090; printf "GET /api/v1/config HTTP/1.0\r\nHost: localhost\r\n\r\n" >&3; head -c 800 <&3'
    hdr "S15 CREDENTIAL / PROCESS SAFETY CHECK"
    echo "期望：以下三项全部为空/未设置"
    run docker exec "$CONTAINER-offline" sh -c 'env | grep -E "^(POLYMARKET_PRIVATE_KEY|BUILDER_|POLY_API)" || echo "(no credential env values)"'
    run docker exec "$CONTAINER-offline" sh -c 'ls -la /app; test -e /app/.env && echo "WARNING: .env present in image" || echo "OK: no .env in image"'
    run docker exec "$CONTAINER-offline" sh -c 'test -e /app/.env.example && echo "WARNING: .env.example present" || echo "OK: no .env.example in image"'
    hdr "PROCESS USER (expect uid=10001 app, not root)"
    run docker exec "$CONTAINER-offline" id
  } | tee "$out"
  echo "-> $out"
}

# S07-S10 —— 真实公开行情（外部连通性层）
cmd_online() {
  need_docker
  local out="$EV/03-online-market-data.txt"
  docker rm -f "$CONTAINER" >/dev/null 2>&1
  {
    hdr "START WITH NETWORK, PORT BOUND TO 127.0.0.1 ONLY"
    run docker run -d --name "$CONTAINER" --platform "$PLATFORM" \
        -p 127.0.0.1:${HOST_PORT}:8090 "${LAB_ENV[@]}" "$IMAGE"
    sleep 8
    run docker inspect "$CONTAINER" --format 'Running={{.State.Running}} ExitCode={{.State.ExitCode}}'
    run docker port "$CONTAINER"
    hdr "HEALTH FROM HOST (must be 127.0.0.1 only)"
    run curl -sS -o /dev/null -w 'http=%{http_code}\n' --max-time 10 "http://127.0.0.1:${HOST_PORT}/health"
    run curl -sS --max-time 10 "http://127.0.0.1:${HOST_PORT}/health"

    hdr "S07 BINANCE SPOT REST (external connectivity, from inside container)"
    echo "endpoints per src/binance_rest.go:14-16"
    run docker exec "$CONTAINER" sh -c 'command -v curl >/dev/null || echo "(no curl in runtime image; see NOTE below)"'
    echo "NOTE: runtime 镜像刻意不含 curl。外部连通性探测改用 builder 阶段镜像或宿主机执行："
    run curl -sS -o /dev/null -w 'api.binance.com/api/v3/ticker/price?symbol=BTCUSDT -> http=%{http_code}\n' --max-time 15 'https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT'
    run curl -sS -o /dev/null -w 'api.binance.com/api/v3/klines 15m -> http=%{http_code}\n' --max-time 15 'https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=15m&limit=2'
    run bash -c "curl -sS --max-time 15 'https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=15m&limit=1' | head -c 400"

    hdr "S08 BINANCE FUTURES WS ENDPOINT (src/binance_ws.go:16)"
    echo "wss://fstream.binance.com/stream?streams=btcusdt@kline_15m/btcusdt@kline_1m"
    run curl -sS -o /dev/null -w 'ws handshake -> http=%{http_code}\n' --max-time 15 \
        -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' \
        -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
        'https://fstream.binance.com/stream?streams=btcusdt@kline_15m'
    echo "期望 http=101 (Switching Protocols)。其它值请原样记录。"

    hdr "S09 POLYMARKET MARKET DISCOVERY (src/endpoint.go:4, src/search.go:16, api/strategy.go:1180-1189)"
    echo "实际发现方式：GET gamma-api/public-search?q=<btc|eth|sol|xrp> updown 15m&events_status=active"
    for q in 'btc+updown+15m' 'eth+updown+15m' 'sol+updown+15m' 'xrp+updown+15m'; do
      run curl -sS -o /dev/null -w "public-search q=$q -> http=%{http_code}\n" --max-time 20 \
        "https://gamma-api.polymarket.com/public-search?q=${q}&events_status=active"
    done
    echo "--- 关键：15m 市场是否仍然存在、slug 规则是否仍为 <sym>-updown-15m-<unix> ---"
    run bash -c "curl -sS --max-time 20 'https://gamma-api.polymarket.com/public-search?q=btc+updown+15m&events_status=active' | python3 -c 'import sys,json; d=json.load(sys.stdin); ev=d.get(\"events\",[]); print(\"events:\",len(ev)); [print(\"  slug=\",e.get(\"slug\"),\"active=\",e.get(\"active\"),\"closed=\",e.get(\"closed\"),\"end=\",e.get(\"endDate\")) for e in ev[:10]]'"

    hdr "S10 POLYMARKET CLOB ORDERBOOK + WS (src/realtime/rest.go:48,80,106; src/endpoint.go:8)"
    run curl -sS -o /dev/null -w 'clob.polymarket.com/book (no token_id, expect 4xx) -> http=%{http_code}\n' --max-time 15 'https://clob.polymarket.com/book'
    run curl -sS -o /dev/null -w 'ws-subscriptions-clob handshake -> http=%{http_code}\n' --max-time 15 \
        -H 'Connection: Upgrade' -H 'Upgrade: websocket' -H 'Sec-WebSocket-Version: 13' \
        -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
        'https://ws-subscriptions-clob.polymarket.com/ws/market'

    hdr "S11 APPLICATION INTEGRATION — 预期为 BLOCKED"
    echo "代码事实：Polymarket MarketManager 在 api/strategy.go:373 创建，"
    echo "Binance WS 在 api/strategy.go:398 创建，两者都在 runSimulationLoop 内部，"
    echo "只能由 StartSimulation() 触发；StartSimulation 的唯一 HTTP 入口是"
    echo "POST /api/v1/sim-copy-trading（需操作码）。"
    echo "因此：不启动模拟策略 => 应用内部不会有任何行情更新。"
    run docker logs "$CONTAINER" --tail 40
    echo "期望：日志中不出现 MarketManager / Binance WS 相关行。"
  } | tee "$out"
  echo "-> $out"
}

# S07-S10 —— 在容器网络内（而非宿主机）做公开行情探测，并统计 WS 真实消息数。
# 使用 builder 阶段镜像（含 Go + 模块缓存）。探测程序通过 docker cp 注入一次性容器，
# 不挂载任何宿主目录。探测程序只访问公开只读端点，不携带任何凭证。
cmd_probe() {
  need_docker
  local secs="${1:-30}"
  local out="$EV/03b-container-probe.txt"
  local pc="$CONTAINER-probe"
  docker rm -f "$pc" >/dev/null 2>&1
  {
    hdr "CONTAINER-NETWORK PUBLIC MARKET-DATA PROBE (collect ${secs}s per WS)"
    run docker build --platform "$PLATFORM" --progress=quiet --target builder -t "$IMAGE-builder" .
    run docker create --name "$pc" --platform "$PLATFORM" --workdir /src \
        --entrypoint /usr/local/go/bin/go "$IMAGE-builder" \
        run scripts/poc/wsprobe/main.go -seconds "$secs"
    run docker cp scripts/poc/wsprobe/main.go "$pc:/src/scripts/poc/wsprobe/main.go"
    run docker start -a "$pc"
    run docker rm "$pc"
  } | tee "$out"
  echo "-> $out"
}

cmd_observe() {
  need_docker
  local mins="${1:-20}"
  local out="$EV/04-observe-${mins}min.txt"
  {
    hdr "CONTINUOUS OBSERVATION ${mins} MINUTES (real elapsed time, recorded)"
    echo "start: $(ts)"
    local i
    for ((i=1; i<=mins; i++)); do
      sleep 60
      printf '[%s] t+%dm ' "$(ts)" "$i"
      docker inspect "$CONTAINER" --format 'running={{.State.Running}} exit={{.State.ExitCode}} restarts={{.RestartCount}}' 2>&1 | tr -d '\n'
      printf ' | '
      docker stats --no-stream --format '{{.CPUPerc}} {{.MemUsage}}' "$CONTAINER" 2>&1 | tr -d '\n'
      printf ' | health='
      curl -sS -o /dev/null -w '%{http_code}' --max-time 8 "http://127.0.0.1:${HOST_PORT}/health" 2>&1
      echo
    done
    echo "end: $(ts)"
    hdr "LOG AFTER OBSERVATION (error/repeat check)"
    run docker logs "$CONTAINER" --tail 120
    hdr "SQLITE FILES AFTER OBSERVATION"
    run docker exec "$CONTAINER" ls -la /app/data /app/logs
  } | tee "$out"
  echo "-> $out"
}

cmd_lifecycle() {
  need_docker
  local out="$EV/05-lifecycle.txt"
  {
    hdr "S13 GRACEFUL STOP"
    run docker stop -t 20 "$CONTAINER"
    run docker inspect "$CONTAINER" --format 'ExitCode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}} Error={{.State.Error}} FinishedAt={{.State.FinishedAt}}'
    echo "期望 ExitCode=0（main.go:204-227 处理 SIGTERM 并优雅退出）"
    run docker logs "$CONTAINER" --tail 15
    hdr "S14 RESTART FROM THE SAME IMAGE (fresh container, fresh temp DB)"
    run docker rm -f "$CONTAINER"
    run docker run -d --name "$CONTAINER" --platform "$PLATFORM" \
        -p 127.0.0.1:${HOST_PORT}:8090 "${LAB_ENV[@]}" "$IMAGE"
    sleep 8
    run docker inspect "$CONTAINER" --format 'Running={{.State.Running}} ExitCode={{.State.ExitCode}}'
    run curl -sS -o /dev/null -w 'health after restart -> http=%{http_code}\n' --max-time 10 "http://127.0.0.1:${HOST_PORT}/health"
    run docker exec "$CONTAINER" bash -c 'exec 3<>/dev/tcp/127.0.0.1/8090; printf "GET /api/v1/sim-stats HTTP/1.0\r\nHost: localhost\r\n\r\n" >&3; cat <&3'
    run docker logs "$CONTAINER" --tail 25
  } | tee "$out"
  echo "-> $out"
}

cmd_teardown() {
  need_docker
  local out="$EV/06-teardown.txt"
  {
    hdr "STOP EXPERIMENT CONTAINERS (images and previous experiment dirs untouched)"
    run docker stop -t 20 "$CONTAINER"
    run docker stop -t 20 "$CONTAINER-offline"
    run docker ps -a --filter "name=$CONTAINER" --format '{{.Names}} {{.Status}}'
    echo "NOTE: 容器刻意保留（未 rm），以便复查状态与日志。"
    echo "      确认不再需要后手动删除： docker rm $CONTAINER $CONTAINER-offline"
  } | tee "$out"
  echo "-> $out"
}

case "${1:-}" in
  preflight) cmd_preflight ;;
  build)     cmd_build ;;
  offline)   cmd_offline ;;
  online)    cmd_online ;;
  probe)     cmd_probe "${2:-30}" ;;
  observe)   cmd_observe "${2:-20}" ;;
  lifecycle) cmd_lifecycle ;;
  teardown)  cmd_teardown ;;
  *) sed -n '2,30p' "$0"; exit 2 ;;
esac
