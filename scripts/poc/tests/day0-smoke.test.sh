#!/usr/bin/env bash
# =============================================================================
# Tests for scripts/poc/day0-docker-smoke.sh (Refs #3, review R04 / R05)
#
#   bash scripts/poc/tests/day0-smoke.test.sh
#
# 使用 mock docker（临时目录中的假 `docker` 可执行文件），不触碰真实 Docker、
# 不启动容器、不联网、不写入 reports/engineering/poc/evidence/。
# 覆盖：语法检查、退出码传播、构建失败即停止、不复用旧镜像、证据不覆盖、
#       历史证据目录保护、容器归属校验、凭证值不输出。
# =============================================================================
set -uo pipefail

REPO="$(cd "$(dirname "$0")/../../.." && pwd -P)"
SCRIPT="$REPO/scripts/poc/day0-docker-smoke.sh"
LEGACY="$REPO/reports/engineering/poc/evidence"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/day0-smoke-test.XXXXXX")"
trap 'rm -rf "$TMP"' EXIT

PASSN=0; FAILN=0
ok()   { PASSN=$((PASSN+1)); echo "  ok   - $*"; }
bad()  { FAILN=$((FAILN+1)); echo "  FAIL - $*"; }
expect_rc() { # <desc> <expected> <actual>
  if [ "$2" = "$3" ]; then ok "$1 (exit=$3)"; else bad "$1 (expected exit=$2, got $3)"; fi
}
expect_nonzero() { if [ "$2" -ne 0 ]; then ok "$1 (exit=$2)"; else bad "$1 (expected non-zero exit)"; fi; }
contains()     { if grep -qF -- "$3" "$2"; then ok "$1"; else bad "$1 (missing: $3)"; fi; }
not_contains() { if grep -qF -- "$3" "$2"; then bad "$1 (found: $3)"; else ok "$1"; fi; }

# --- mock docker ------------------------------------------------------------
MOCKBIN="$TMP/bin"; STATE="$TMP/state"; mkdir -p "$MOCKBIN" "$STATE/containers"
CALLS="$STATE/calls.log"
cat > "$MOCKBIN/docker" <<'MOCK'
#!/usr/bin/env bash
S="$MOCK_STATE"
echo "docker $*" >> "$S/calls.log"
cmd="$1"; shift
case "$cmd" in
  version|ps|logs|port|stats|cp|start|context) exit 0 ;;
  build)
    iid=""
    while [ $# -gt 0 ]; do [ "$1" = "--iidfile" ] && iid="$2"; shift; done
    if [ "${MOCK_BUILD_RC:-0}" != "0" ]; then echo "ERROR: mock build failure" >&2; exit "$MOCK_BUILD_RC"; fi
    [ -n "$iid" ] && printf '%s' "${MOCK_NEW_ID:-sha256:newimage}" > "$iid"
    exit 0 ;;
  image)  # image inspect <id> --format ...
    case "$*" in *Architecture*) echo arm64 ;; *) echo "Id=$2 (mock)";; esac; exit 0 ;;
  run|create) echo "mockcontainerid"; exit 0 ;;
  rm|stop) exit 0 ;;
  container)  # container inspect [-f FMT] NAME
    shift; fmt=""; [ "$1" = "-f" ] && { fmt="$2"; shift 2; }; name="$1"; shift
    [ "${1:-}" = "--format" ] && fmt="$2"
    d="$S/containers/$name"; [ -d "$d" ] || exit 1
    case "$fmt" in
      *Labels*) key="$(printf '%s' "$fmt" | sed -E 's/.*Labels "([^"]+)".*/\1/')"; cat "$d/$key" 2>/dev/null; echo ;;
      *Privileged*) echo false ;;
      *Mounts*) echo 0 ;;
      *Running*) echo true ;;
      *ExitCode*) echo 0 ;;
      *RestartCount*) echo 0 ;;
      *) echo mock ;;
    esac; exit 0 ;;
  exec)
    name="$1"; shift
    case "$*" in *docker.sock*) exit 0 ;; "id -u") echo 10001; exit 0 ;; esac
    exec env -i PATH="$PATH" TRADING_MODE=simulation ENABLE_MAIN_STRATEGY=false \
      ENABLE_PATTERN_STRATEGY=false SNIPER_MODE=false \
      POLYMARKET_PRIVATE_KEY="${MOCK_SECRET-}" BUILDER_KEY= POLY_API_SECRET= "$@" ;;
  *) echo "mock: unhandled docker $cmd" >&2; exit 99 ;;
esac
MOCK
chmod +x "$MOCKBIN/docker"

mkc() { # mkc <name> <experiment-label> <run-label>
  local d="$STATE/containers/$1"; mkdir -p "$d"
  printf '%s' "$2" > "$d/org.polymarket-lab.experiment"
  printf '%s' "$3" > "$d/org.polymarket-lab.run"
}

# 在 repo 根目录运行被测脚本，证据写到临时目录
smoke() { # smoke <RUN_ID> <args...>   (extra env passed by caller)
  local rid="$1"; shift
  ( cd "$REPO" && PATH="$MOCKBIN:$PATH" MOCK_STATE="$STATE" RUN_ID="$rid" \
      EVIDENCE_ROOT="$TMP/ev" bash "$SCRIPT" "$@" ) > "$TMP/out.txt" 2>&1
}
reset_calls() { : > "$CALLS"; }

legacy_digest() { (cd "$LEGACY" && find . -maxdepth 1 -type f -name '*.txt' -exec shasum -a 256 {} + | sort); }
LEGACY_BEFORE="$(legacy_digest)"

# =============================================================================
echo "T1 syntax"
bash -n "$SCRIPT"; expect_rc "bash -n smoke script" 0 $?
bash -n "$0";      expect_rc "bash -n test script" 0 $?
if command -v shellcheck >/dev/null 2>&1; then
  shellcheck -S warning "$SCRIPT"; expect_rc "shellcheck -S warning" 0 $?
else
  echo "  skip - shellcheck not installed"
fi

# =============================================================================
echo "T2 exit-code propagation (run / must / diag / step)"
(
  export DAY0_SMOKE_SOURCE_ONLY=1 RUN_ID=t2 EVIDENCE_ROOT="$TMP/ev"
  # shellcheck disable=SC1090
  source "$SCRIPT"
  run false >/dev/null;                 echo "run_false=$?"
  run sh -c 'exit 7' >/dev/null;        echo "run_7=$?"
  run true >/dev/null;                  echo "run_true=$?"
  ( must sh -c 'exit 5' ) >/dev/null;   echo "must_5=$?"
  diag sh -c 'exit 9' >/dev/null;       echo "diag_9=$?"
  { run sh -c 'exit 4'; } 2>&1 | tee /dev/null >/dev/null; echo "pipe_tee_4=${PIPESTATUS[0]}"
  init_evidence
  f() { echo hi; exit 6; }
  step t2-step.txt f >/dev/null;        echo "step_6=$?"
) > "$TMP/t2.txt" 2>&1
for kv in run_false=1 run_7=7 run_true=0 must_5=5 diag_9=0 pipe_tee_4=4 step_6=6; do
  contains "$kv" "$TMP/t2.txt" "$kv"
done

# =============================================================================
echo "T3 build failure stops the run (and never inspects an existing image)"
reset_calls
MOCK_BUILD_RC=17 smoke t3 build; rc=$?
expect_rc "build exits with docker build's code" 17 "$rc"
contains "evidence records BUILD FAILED" "$TMP/ev/t3/01-build.txt" "BUILD FAILED"
contains "evidence records real exit code" "$TMP/ev/t3/01-build.txt" "[exit=17]"
[ ! -e "$TMP/ev/t3/image.iid" ] && ok "no image.iid written" || bad "image.iid exists after failed build"
not_contains "no 'docker image inspect' after failed build" "$CALLS" "docker image inspect"
not_contains "no 'docker run' after failed build" "$CALLS" "docker run"

echo "T4 later steps refuse to use the old tagged image"
reset_calls
smoke t3 offline; rc=$?
expect_rc "offline refuses without a build in this run" 3 "$rc"
contains "refusal explains no build in this run" "$TMP/out.txt" "没有成功构建的镜像"
not_contains "no container started" "$CALLS" "docker run"
for s in online lifecycle probe; do
  reset_calls; smoke t3 "$s"; rc=$?
  expect_rc "$s refuses without a build in this run" 3 "$rc"
  not_contains "$s starts nothing" "$CALLS" "docker run"
done

echo "T5 successful build records this run's image ID"
reset_calls
MOCK_NEW_ID="sha256:fresh123" smoke t5 build; rc=$?
expect_rc "build succeeds" 0 "$rc"
[ "$(cat "$TMP/ev/t5/image.iid" 2>/dev/null)" = "sha256:fresh123" ] && ok "image.iid = fresh ID" || bad "image.iid wrong"
contains "inspects by fresh ID, not tag" "$CALLS" "docker image inspect sha256:fresh123"

# =============================================================================
echo "T6 evidence protection"
smoke t6 preflight; rc=$?; expect_rc "first preflight" 0 "$rc"
sum1="$(shasum -a 256 "$TMP/ev/t6/00-preflight.txt")"
smoke t6 preflight; rc=$?; expect_rc "second preflight with same RUN_ID refuses" 3 "$rc"
contains "refusal message" "$TMP/out.txt" "拒绝覆盖"
[ "$(shasum -a 256 "$TMP/ev/t6/00-preflight.txt")" = "$sum1" ] && ok "existing evidence unchanged" || bad "evidence overwritten"
( cd "$REPO" && PATH="$MOCKBIN:$PATH" MOCK_STATE="$STATE" RUN_ID=t6b \
    EVIDENCE_DIR="reports/engineering/poc/evidence" bash "$SCRIPT" preflight ) > "$TMP/out.txt" 2>&1; rc=$?
expect_rc "writing into legacy evidence dir refuses" 3 "$rc"
( cd "$REPO" && PATH="$MOCKBIN:$PATH" MOCK_STATE="$STATE" RUN_ID='bad/../id' \
    EVIDENCE_ROOT="$TMP/ev" bash "$SCRIPT" preflight ) > "$TMP/out.txt" 2>&1; rc=$?
expect_rc "path-like RUN_ID rejected" 2 "$rc"
( cd "$REPO" && PATH="$MOCKBIN:$PATH" MOCK_STATE="$STATE" RUN_ID=t6c bash "$SCRIPT" 2>/dev/null >/dev/null ); rc=$?
expect_rc "no subcommand -> usage error" 2 "$rc"
smoke t6d preflight >/dev/null; [ -f "$TMP/ev/t6d/00-preflight.txt" ] && ok "default layout runs/<RUN_ID>/" || bad "unexpected evidence layout"

# =============================================================================
echo "T7 container ownership"
mkc polymarket-lab-day0-t7 "someone-else" "t7"
reset_calls; smoke t7 teardown; rc=$?
expect_rc "teardown refuses foreign-labelled container" 3 "$rc"
contains "refusal message" "$TMP/out.txt" "不属于本实验"
not_contains "no docker stop issued" "$CALLS" "docker stop"
not_contains "no docker rm issued" "$CALLS" "docker rm"

mkc polymarket-lab-day0-t7b "m0-day0-smoke" "OTHER-RUN"
reset_calls; smoke t7b cleanup; rc=$?
expect_rc "cleanup refuses container from another run" 3 "$rc"
not_contains "no docker rm issued" "$CALLS" "docker rm"

mkc polymarket-lab-day0-t7c-offline "unrelated" ""
mkdir -p "$TMP/ev/t7c"; printf 'sha256:x' > "$TMP/ev/t7c/image.iid"
reset_calls; smoke t7c offline; rc=$?
expect_rc "start refuses when name is held by a foreign container" 3 "$rc"
not_contains "foreign container not removed" "$CALLS" "docker rm"
not_contains "nothing started" "$CALLS" "docker run"

mkc polymarket-lab-day0-t7d "m0-day0-smoke" "t7d"
reset_calls; smoke t7d teardown; rc=$?
expect_rc "teardown of own container succeeds" 0 "$rc"
contains "docker stop issued for own container" "$CALLS" "docker stop -t 20 polymarket-lab-day0-t7d"

# =============================================================================
echo "T8 credential values are never printed"
SECRET="0xSECRET_SHOULD_NOT_LEAK_4242"
mkc polymarket-lab-day0-t8 "m0-day0-smoke" "t8"
MOCK_SECRET="$SECRET" smoke t8 safety; rc=$?
expect_rc "safety fails when a credential is set" 1 "$rc"
contains "reports SET status" "$TMP/ev/t8/02b-safety-online.txt" "POLYMARKET_PRIVATE_KEY=SET (value withheld)"
not_contains "secret absent from evidence" "$TMP/ev/t8/02b-safety-online.txt" "$SECRET"
not_contains "secret absent from stdout" "$TMP/out.txt" "$SECRET"

mkc polymarket-lab-day0-t8b "m0-day0-smoke" "t8b"
MOCK_SECRET="" smoke t8b safety; rc=$?
expect_rc "safety passes when credentials empty" 0 "$rc"
contains "reports EMPTY status" "$TMP/ev/t8b/02b-safety-online.txt" "POLYMARKET_PRIVATE_KEY=EMPTY"
contains "reports UNSET status" "$TMP/ev/t8b/02b-safety-online.txt" "TELEGRAM_BOT_TOKEN=UNSET"

mkc polymarket-lab-day0-t8c "intruder" "t8c"
reset_calls; smoke t8c safety; rc=$?
expect_rc "safety refuses foreign container" 3 "$rc"
not_contains "no exec into foreign container" "$CALLS" "docker exec"

# =============================================================================
echo "T9 historical evidence untouched"
[ "$(legacy_digest)" = "$LEGACY_BEFORE" ] && ok "reports/engineering/poc/evidence/*.txt unchanged" || bad "legacy evidence changed"
[ ! -d "$LEGACY/runs" ] && ok "no runs/ dir created in repo by tests" || echo "  note - $LEGACY/runs exists (pre-existing real runs?)"

echo
echo "SUMMARY: pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
