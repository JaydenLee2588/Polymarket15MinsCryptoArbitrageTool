#!/usr/bin/env bash
# =============================================================================
# M0 Day 0 — Docker Smoke Test 执行脚本 (Refs #3)
#
# 在 macOS 宿主机终端运行（Docker Desktop 必须已启动）。
# 一次实验 = 一个 RUN_ID = 一个独立证据目录：
#
#   export RUN_ID=$(date -u +%Y%m%dT%H%M%SZ)
#   bash scripts/poc/day0-docker-smoke.sh preflight
#   bash scripts/poc/day0-docker-smoke.sh build       # 失败即停止，后续步骤拒绝执行
#   bash scripts/poc/day0-docker-smoke.sh offline     # S03–S06 + safety
#   bash scripts/poc/day0-docker-smoke.sh online      # 联网启动 + 宿主机侧诊断探测
#   bash scripts/poc/day0-docker-smoke.sh probe 30    # S07–S10 容器网络内探测
#   bash scripts/poc/day0-docker-smoke.sh observe 15  # S12（空闲进程）
#   bash scripts/poc/day0-docker-smoke.sh lifecycle   # S13–S14
#   bash scripts/poc/day0-docker-smoke.sh teardown
#   # 或一次性：bash scripts/poc/day0-docker-smoke.sh all
#
# 覆盖范围：S01–S10、S12–S15。S11（获批模拟启停）不在本脚本内，见报告 §12 / §20.2。
#
# 证据：reports/engineering/poc/evidence/runs/<RUN_ID>/*.txt
#   - 已有证据文件绝不覆盖（存在即拒绝写入）。
#   - 拒绝把证据直接写进 reports/engineering/poc/evidence/（历史证据目录）。
#   - 用 .txt 而非 .log（.gitignore 含 *.log）。
#
# 退出码：
#   0  所有断言通过
#   1  断言失败 / 关键命令失败
#   2  用法错误
#   3  安全拒绝（容器归属不符、证据会被覆盖、未在本轮构建镜像等）
#
# 安全边界（脚本强制，勿删）：
#   - 不加载 .env / .env.example；不传入任何私钥或交易凭证
#   - TRADING_MODE=simulation / ENABLE_MAIN_STRATEGY=false / ENABLE_PATTERN_STRATEGY=false
#   - 端口只绑定 127.0.0.1；不挂载宿主目录 / docker socket；不使用 --privileged
#   - 不调用任何写入型或策略启动接口
#   - 只 stop / rm 带本实验 label 且属于当前 RUN_ID 的容器
#   - 凭证检查只输出 EMPTY / SET / UNSET，不输出值
# =============================================================================
set -uo pipefail

PLATFORM="${PLATFORM:-linux/arm64}"
IMAGE_TAG="${IMAGE_TAG:-polymarket-engineering-lab:day0}"
GO_VERSION="${GO_VERSION:-1.27.4}"
GO_SHA256="${GO_SHA256:-3392582988a273a87ab5043ee1b2c9c17a2c3c00580bf9326c47a2e0aaaeac92}"
HOST_PORT="${HOST_PORT:-8090}"

LEGACY_EVIDENCE_DIR="reports/engineering/poc/evidence"
EVIDENCE_ROOT="${EVIDENCE_ROOT:-$LEGACY_EVIDENCE_DIR/runs}"
RUN_ID="${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)}"
EV="${EVIDENCE_DIR:-$EVIDENCE_ROOT/$RUN_ID}"

LABEL_EXP_KEY="org.polymarket-lab.experiment"
LABEL_EXP_VAL="m0-day0-smoke"
LABEL_RUN_KEY="org.polymarket-lab.run"

CONTAINER="polymarket-lab-day0-${RUN_ID}"
C_OFFLINE="${CONTAINER}-offline"
C_PROBE="${CONTAINER}-probe"

# 不得为空的凭证类变量名（只检查是否为空，绝不打印值）
CRED_VARS="POLYMARKET_PRIVATE_KEY PROXY_WALLET_ADDRESS MAKER_ADDRESS POLY_API_KEY POLY_API_SECRET POLY_PASSPHRASE POLY_EOA_ADDRESS BUILDER_KEY BUILDER_SECRET BUILDER_PASSPHRASE TELEGRAM_BOT_TOKEN"

# --- 本次实验的运行时环境（唯一来源，勿分叉；bash 3.2 兼容，不用 mapfile） ---
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
# 基础工具
# =============================================================================
ts()   { date -u +%Y-%m-%dT%H:%M:%SZ; }
hdr()  { echo; echo "===== $* ($(ts)) ====="; }
die()  { local rc="$1"; shift; echo "FATAL[$rc]: $*" >&2; exit "$rc"; }

# run: 执行命令，打印输出与真实退出码，并把退出码原样返回（R04）
run() {
  echo "\$ $*"
  "$@" 2>&1
  local rc=$?
  echo "[exit=$rc]"
  return "$rc"
}

# must: 关键步骤，失败即终止当前步骤（R04）
must() {
  run "$@"
  local rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "STEP FAILED: '$*' exited $rc — 停止本步骤。"
    exit "$rc"
  fi
}

# diag: 诊断性命令，允许预期内失败；只记录不传播（R04：诊断失败单独处理）
diag() {
  run "$@"
  local rc=$?
  [ "$rc" -ne 0 ] && echo "[diagnostic only — non-zero exit recorded, not treated as step failure]"
  return 0
}

# check: 断言，失败时记录并累计，不立即退出
FAILS=0
check() {
  local desc="$1"; shift
  if "$@"; then echo "CHECK PASS: $desc"; else echo "CHECK FAIL: $desc"; FAILS=$((FAILS+1)); fi
}
finish_checks() {
  if [ "$FAILS" -ne 0 ]; then echo "RESULT: $FAILS check(s) failed"; exit 1; fi
  echo "RESULT: all checks passed"
}

need_docker() {
  docker version >/dev/null 2>&1 || die 1 "Docker daemon 不可达。请先启动 Docker Desktop。"
}

# --- 证据目录保护（R05） ----------------------------------------------------
init_evidence() {
  case "$RUN_ID" in
    *[!A-Za-z0-9_.-]*|"") die 2 "RUN_ID 只能包含字母、数字、_ . -：'$RUN_ID'" ;;
  esac
  local ev_abs legacy_abs
  mkdir -p "$EV" || die 1 "无法创建证据目录 $EV"
  ev_abs="$(cd "$EV" && pwd -P)"
  legacy_abs="$(cd "$LEGACY_EVIDENCE_DIR" 2>/dev/null && pwd -P || echo "")"
  if [ -n "$legacy_abs" ] && [ "$ev_abs" = "$legacy_abs" ]; then
    die 3 "拒绝写入历史证据目录 ${LEGACY_EVIDENCE_DIR}（请使用 runs/<RUN_ID>/）"
  fi
}

# evidence_path <name>：返回新证据文件路径；文件已存在则拒绝（不覆盖）
evidence_path() {
  local f="$EV/$1"
  if [ -e "$f" ]; then
    die 3 "证据文件已存在，拒绝覆盖：${f}（换一个 RUN_ID 开始新一轮）"
  fi
  printf '%s\n' "$f"
}

# step <evidence-name> <function> [args...]：把一个步骤的输出写入新证据文件并传播退出码
step() {
  local name="$1"; shift
  local out
  out="$(evidence_path "$name")" || exit $?
  { echo "RUN_ID=$RUN_ID"; "$@"; } 2>&1 | tee "$out"
  local rc=${PIPESTATUS[0]}
  echo "-> $out (exit=$rc)"
  return "$rc"
}

# --- 本轮构建产物（R04：不得把旧镜像当新构建） --------------------------------
built_image_id() {
  local f="$EV/image.iid"
  [ -s "$f" ] || die 3 "本轮（RUN_ID=${RUN_ID}）没有成功构建的镜像（缺少 ${f}）。先执行 build；不会回退使用已有标签 ${IMAGE_TAG}。"
  cat "$f"
}

# --- 容器归属（R05） --------------------------------------------------------
container_exists() { docker container inspect "$1" >/dev/null 2>&1; }

label_of() {
  docker container inspect -f "{{index .Config.Labels \"$2\"}}" "$1" 2>/dev/null
}

# assert_owned <name>：容器必须带本实验 label 且属于当前 RUN_ID，否则拒绝（退出 3）
assert_owned() {
  local name="$1" exp run_lbl
  exp="$(label_of "$name" "$LABEL_EXP_KEY")"
  run_lbl="$(label_of "$name" "$LABEL_RUN_KEY")"
  if [ "$exp" != "$LABEL_EXP_VAL" ] || [ "$run_lbl" != "$RUN_ID" ]; then
    echo "REFUSE: 容器 $name 不属于本实验（$LABEL_EXP_KEY='$exp', $LABEL_RUN_KEY='$run_lbl', 期望 '$LABEL_EXP_VAL' / '$RUN_ID'）。未执行任何 stop/rm。"
    exit 3
  fi
}

safe_rm() {
  local name="$1"
  container_exists "$name" || { echo "(container $name absent — nothing to remove)"; return 0; }
  assert_owned "$name"
  must docker rm -f "$name"
}

safe_stop() {
  local name="$1"
  container_exists "$name" || { echo "(container $name absent — nothing to stop)"; return 0; }
  assert_owned "$name"
  must docker stop -t 20 "$name"
}

# 启动实验容器：新名称必须空闲或属于本轮；总是带实验 label
start_container() {
  local name="$1"; shift
  safe_rm "$name"
  must docker run -d --name "$name" --platform "$PLATFORM" \
      --label "$LABEL_EXP_KEY=$LABEL_EXP_VAL" --label "$LABEL_RUN_KEY=$RUN_ID" \
      "$@"
}

# 容器内 HTTP GET（/dev/tcp），状态行含 " 200 " 才返回 0
http_in() {
  local name="$1" path="$2" limit="${3:-2000}"
  docker exec "$name" bash -c '
    exec 3<>/dev/tcp/127.0.0.1/8090 || { echo "tcp connect failed"; exit 1; }
    printf "GET '"$path"' HTTP/1.0\r\nHost: localhost\r\n\r\n" >&3
    IFS= read -r status <&3
    echo "$status"
    head -c '"$limit"' <&3; echo
    case "$status" in *" 200 "*) exit 0;; *) exit 1;; esac'
}

is_running() { [ "$(docker container inspect -f '{{.State.Running}}' "$1" 2>/dev/null)" = "true" ]; }

# =============================================================================
# 步骤
# =============================================================================
do_preflight() {
  hdr "HOST"
  run uname -a
  diag sw_vers
  hdr "GIT BASELINE"
  run git rev-parse --show-toplevel
  run git branch --show-current
  run git rev-parse HEAD
  run git status --short
  hdr "DEPENDENCY DECLARATION DIGEST (before build)"
  must shasum -a 256 go.mod go.sum
  hdr "DOCKER"
  must docker version
  run docker context show
}

do_build() {
  need_docker
  local iid="$EV/image.iid.tmp" t0 t1 rc
  hdr "BUILD (fresh image ID captured with --iidfile; existing tag never reused)"
  echo "GO_VERSION=$GO_VERSION"
  echo "GO_SHA256=$GO_SHA256"
  t0=$(date +%s)
  run docker build --platform "$PLATFORM" --progress=plain \
      --build-arg "GO_VERSION=$GO_VERSION" --build-arg "GO_SHA256=$GO_SHA256" \
      --iidfile "$iid" -t "$IMAGE_TAG" .
  rc=$?
  t1=$(date +%s)
  echo "[build exit=$rc elapsed=$((t1-t0))s]"
  if [ "$rc" -ne 0 ] || [ ! -s "$iid" ]; then
    rm -f "$iid"
    echo "BUILD FAILED — 验收停止。不检查、不使用任何已有镜像（包括标签 $IMAGE_TAG 指向的旧镜像）。"
    [ "$rc" -ne 0 ] && exit "$rc"
    exit 1
  fi
  mv "$iid" "$EV/image.iid"
  local id; id="$(cat "$EV/image.iid")"
  echo "BUILT IMAGE ID (this run): $id"
  hdr "IMAGE IDENTITY (by ID, not by tag)"
  must docker image inspect "$id" --format 'Id={{.Id}} Arch={{.Architecture}} Os={{.Os}} Size={{.Size}} Created={{.Created}}'
  check "image architecture is arm64" [ "$(docker image inspect "$id" --format '{{.Architecture}}')" = "arm64" ]
  must docker run --rm --platform "$PLATFORM" --network none --entrypoint /bin/sh "$id" \
      -c 'grep -E "^(PRETTY_NAME|VERSION_ID)=" /etc/os-release; uname -m; id'
  hdr "GO VERSION (builder stage)"
  local biid="$EV/builder.iid"
  must docker build --platform "$PLATFORM" --progress=quiet --target builder --iidfile "$biid" .
  must docker run --rm --network none --entrypoint /usr/local/go/bin/go "$(cat "$biid")" version
  hdr "DEPENDENCY DECLARATION DIGEST (after build)"
  must shasum -a 256 go.mod go.sum
  finish_checks
}

# S15 safety：凭证只报告状态（R05）；容器隔离属性
do_safety() {
  local name="$1"
  need_docker
  container_exists "$name" || die 1 "container $name not found"
  assert_owned "$name"
  hdr "S15 CREDENTIAL STATUS (values are never printed)"
  # shellcheck disable=SC2016
  local cred_script='bad=0
for v in '"$CRED_VARS"'; do
  if printenv "$v" >/dev/null 2>&1; then
    if [ -n "$(printenv "$v")" ]; then echo "$v=SET (value withheld)"; bad=1; else echo "$v=EMPTY"; fi
  else
    echo "$v=UNSET"
  fi
done
exit $bad'
  check "all credential variables empty or unset" run docker exec "$name" sh -c "$cred_script"
  check "no /app/.env in container" run docker exec "$name" sh -c 'test ! -e /app/.env'
  check "no /app/.env.example in container" run docker exec "$name" sh -c 'test ! -e /app/.env.example'
  hdr "S15 ISOLATION"
  check "not privileged" [ "$(docker container inspect -f '{{.HostConfig.Privileged}}' "$name")" = "false" ]
  check "no mounts" [ "$(docker container inspect -f '{{len .Mounts}}' "$name")" = "0" ]
  check "no docker.sock inside" run docker exec "$name" sh -c 'test ! -S /var/run/docker.sock'
  check "runs as uid 10001" [ "$(docker exec "$name" id -u 2>/dev/null)" = "10001" ]
  run docker exec "$name" sh -c 'env | grep -E "^(TRADING_MODE|ENABLE_MAIN_STRATEGY|ENABLE_PATTERN_STRATEGY|SNIPER_MODE)=" | sort'
  check "TRADING_MODE=simulation" [ "$(docker exec "$name" printenv TRADING_MODE 2>/dev/null)" = "simulation" ]
  check "ENABLE_MAIN_STRATEGY=false" [ "$(docker exec "$name" printenv ENABLE_MAIN_STRATEGY 2>/dev/null)" = "false" ]
  check "ENABLE_PATTERN_STRATEGY=false" [ "$(docker exec "$name" printenv ENABLE_PATTERN_STRATEGY 2>/dev/null)" = "false" ]
  finish_checks
}

do_offline() {
  need_docker
  local id; id="$(built_image_id)" || exit $?
  hdr "START WITH --network none (no external network, no creds) image=$id"
  start_container "$C_OFFLINE" --network none "${LAB_ENV[@]}" "$id"
  sleep 8
  hdr "CONTAINER STATE"
  run docker container inspect "$C_OFFLINE" --format 'Running={{.State.Running}} ExitCode={{.State.ExitCode}} OOM={{.State.OOMKilled}} Restarts={{.RestartCount}}'
  check "S03 container running" is_running "$C_OFFLINE"
  hdr "APPLICATION LOG"
  run docker logs "$C_OFFLINE"
  hdr "S04 SQLITE"
  check "S04 trades.db exists" run docker exec "$C_OFFLINE" test -s /app/data/trades.db
  check "S04 sim-stats (queries DB) returns 200" http_in "$C_OFFLINE" /api/v1/sim-stats
  hdr "S05 HEALTH"
  check "S05 /health returns 200" http_in "$C_OFFLINE" /health
  hdr "S06 WEB UI + READ-ONLY CONFIG API"
  check "S06 / returns 200" http_in "$C_OFFLINE" / 400
  check "S06 GET /api/v1/config returns 200" http_in "$C_OFFLINE" /api/v1/config 800
  finish_checks
}

do_online() {
  need_docker
  local id; id="$(built_image_id)" || exit $?
  hdr "START WITH NETWORK, PORT BOUND TO 127.0.0.1 ONLY image=$id"
  start_container "$CONTAINER" -p "127.0.0.1:${HOST_PORT}:8090" "${LAB_ENV[@]}" "$id"
  sleep 8
  run docker port "$CONTAINER"
  check "container running" is_running "$CONTAINER"
  check "port bound to 127.0.0.1 only" [ "$(docker port "$CONTAINER" 8090/tcp 2>/dev/null)" = "127.0.0.1:${HOST_PORT}" ]
  check "host /health 200" run curl -fsS --max-time 10 "http://127.0.0.1:${HOST_PORT}/health"

  hdr "HOST-SIDE DIAGNOSTIC PROBES (not acceptance; container-network probe is 'probe')"
  diag curl -sS -o /dev/null -w 'api.binance.com klines -> http=%{http_code}\n' --max-time 15 'https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=15m&limit=2'
  diag curl -sS -o /dev/null -w 'gamma public-search -> http=%{http_code}\n' --max-time 20 'https://gamma-api.polymarket.com/public-search?q=btc+updown+15m&events_status=active'
  diag curl -sS -o /dev/null -w 'clob /book without token_id (expect 4xx) -> http=%{http_code}\n' --max-time 15 'https://clob.polymarket.com/book'
  hdr "APPLICATION LOG (no strategy started => no market components expected)"
  run docker logs "$CONTAINER" --tail 40
  finish_checks
}

do_probe() {
  need_docker
  local secs="${1:-30}"
  built_image_id >/dev/null || exit $?
  hdr "CONTAINER-NETWORK PUBLIC MARKET-DATA PROBE (collect ${secs}s per WS)"
  echo "NOTE: probe 统计的是 WebSocket 文本帧数量，不等于有效市场消息数（R06）。"
  local biid="$EV/probe-builder.iid"
  must docker build --platform "$PLATFORM" --progress=quiet --target builder --iidfile "$biid" .
  safe_rm "$C_PROBE"
  must docker create --name "$C_PROBE" --platform "$PLATFORM" \
      --label "$LABEL_EXP_KEY=$LABEL_EXP_VAL" --label "$LABEL_RUN_KEY=$RUN_ID" \
      --workdir /src --entrypoint /usr/local/go/bin/go "$(cat "$biid")" \
      run scripts/poc/wsprobe/main.go -seconds "$secs"
  must docker cp scripts/poc/wsprobe/main.go "$C_PROBE:/src/scripts/poc/wsprobe/main.go"
  run docker start -a "$C_PROBE"
  local rc=$?
  safe_rm "$C_PROBE"
  [ "$rc" -ne 0 ] && echo "PROBE RESULT: FAIL (exit=$rc) — 见上方 RESULT 行"
  exit "$rc"
}

do_observe() {
  need_docker
  local mins="${1:-15}" i code
  case "$mins" in ''|*[!0-9]*) die 2 "observe 分钟数必须是正整数：$mins" ;; esac
  container_exists "$CONTAINER" || die 1 "container $CONTAINER not found (run 'online' first)"
  assert_owned "$CONTAINER"
  hdr "CONTINUOUS OBSERVATION ${mins} MINUTES (idle process; real elapsed time)"
  echo "NOTE: 未启动策略，进程无行情负载；这不是带行情负载的稳定性测试（R06）。"
  echo "start: $(ts)"
  for ((i=1; i<=mins; i++)); do
    sleep 60
    code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 8 "http://127.0.0.1:${HOST_PORT}/health" 2>/dev/null)"
    printf '[%s] t+%dm %s | %s | health=%s\n' "$(ts)" "$i" \
      "$(docker container inspect "$CONTAINER" --format 'running={{.State.Running}} exit={{.State.ExitCode}} restarts={{.RestartCount}}' 2>&1)" \
      "$(docker stats --no-stream --format '{{.CPUPerc}} {{.MemUsage}}' "$CONTAINER" 2>&1)" "$code"
    [ "$code" = "200" ] || { echo "CHECK FAIL: health at t+${i}m = $code"; FAILS=$((FAILS+1)); }
    is_running "$CONTAINER" || { echo "CHECK FAIL: not running at t+${i}m"; FAILS=$((FAILS+1)); }
  done
  echo "end: $(ts)"
  hdr "LOG AFTER OBSERVATION"
  run docker logs "$CONTAINER" --tail 120
  check "restart count is 0" [ "$(docker container inspect -f '{{.RestartCount}}' "$CONTAINER")" = "0" ]
  finish_checks
}

do_lifecycle() {
  need_docker
  local id; id="$(built_image_id)" || exit $?
  container_exists "$CONTAINER" || die 1 "container $CONTAINER not found (run 'online' first)"
  hdr "S13 GRACEFUL STOP"
  safe_stop "$CONTAINER"
  run docker container inspect "$CONTAINER" --format 'ExitCode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}} FinishedAt={{.State.FinishedAt}}'
  check "S13 exit code 0" [ "$(docker container inspect -f '{{.State.ExitCode}}' "$CONTAINER")" = "0" ]
  run docker logs "$CONTAINER" --tail 5
  hdr "S14 RESTART FROM THE SAME IMAGE ID ($id)"
  start_container "$CONTAINER" -p "127.0.0.1:${HOST_PORT}:8090" "${LAB_ENV[@]}" "$id"
  sleep 8
  check "S14 running after restart" is_running "$CONTAINER"
  check "S14 host /health 200" run curl -fsS --max-time 10 "http://127.0.0.1:${HOST_PORT}/health"
  check "S14 sim-stats 200" http_in "$CONTAINER" /api/v1/sim-stats
  finish_checks
}

do_teardown() {
  need_docker
  hdr "STOP THIS RUN'S EXPERIMENT CONTAINERS (kept, not removed)"
  safe_stop "$CONTAINER"
  safe_stop "$C_OFFLINE"
  run docker ps -a --filter "label=$LABEL_RUN_KEY=$RUN_ID" --format '{{.Names}} {{.Status}}'
  echo "NOTE: 容器保留以便复查。删除：RUN_ID=$RUN_ID bash $0 cleanup"
}

do_cleanup() {
  need_docker
  hdr "REMOVE THIS RUN'S EXPERIMENT CONTAINERS (ownership checked)"
  safe_rm "$CONTAINER"
  safe_rm "$C_OFFLINE"
  safe_rm "$C_PROBE"
}

# all：基础设施失败立即中止；行情探测失败记录后继续，最终退出码非零
do_all() {
  local overall=0 rc
  step 00-preflight.txt do_preflight || exit $?
  step 01-build.txt do_build || exit $?
  step 02-offline.txt do_offline || exit $?
  step 02b-safety-offline.txt do_safety "$C_OFFLINE" || exit $?
  step 03-online.txt do_online || exit $?
  step 03b-probe.txt do_probe "${PROBE_SECONDS:-30}"; rc=$?; [ "$rc" -ne 0 ] && overall=$rc
  step 04-observe.txt do_observe "${OBSERVE_MINUTES:-15}"; rc=$?; [ "$rc" -ne 0 ] && overall=$rc
  step 05-lifecycle.txt do_lifecycle || exit $?
  step 05b-safety-online.txt do_safety "$CONTAINER"; rc=$?; [ "$rc" -ne 0 ] && overall=$rc
  step 06-teardown.txt do_teardown || exit $?
  echo "ALL DONE RUN_ID=$RUN_ID overall_exit=$overall evidence=$EV"
  exit "$overall"
}

usage() { sed -n '2,40p' "$0"; }

main() {
  local cmd="${1:-}"
  [ -n "$cmd" ] || { usage; exit 2; }
  shift
  init_evidence
  echo "RUN_ID=$RUN_ID evidence=$EV" >&2
  case "$cmd" in
    preflight) step 00-preflight.txt do_preflight ;;
    build)     step 01-build.txt do_build ;;
    offline)   step 02-offline.txt do_offline ;;
    safety)    step "02b-safety-${1:-online}.txt" do_safety "$( [ "${1:-online}" = offline ] && echo "$C_OFFLINE" || echo "$CONTAINER")" ;;
    online)    step 03-online.txt do_online ;;
    probe)     step 03b-probe.txt do_probe "${1:-30}" ;;
    observe)   step 04-observe.txt do_observe "${1:-15}" ;;
    lifecycle) step 05-lifecycle.txt do_lifecycle ;;
    teardown)  step 06-teardown.txt do_teardown ;;
    cleanup)   step 07-cleanup.txt do_cleanup ;;
    all)       do_all ;;
    *) usage; exit 2 ;;
  esac
}

# 测试可 source 本文件而不执行 main
if [ "${DAY0_SMOKE_SOURCE_ONLY:-0}" != "1" ]; then
  main "$@"
fi
