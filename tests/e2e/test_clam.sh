#!/usr/bin/env sh
set -eu

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
TMP="${TMPDIR:-/tmp}/keep-clam-test.$$"
FAKE_BIN="${TMP}/fake-bin"
HOME_DIR="${TMP}/home"
LOADED_FILE="${TMP}/launchd.loaded"
PMSET_LOG="${TMP}/pmset.log"

cleanup() {
  rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  haystack="$1"
  needle="$2"
  printf '%s' "$haystack" | grep -F "$needle" >/dev/null || {
    printf 'output did not contain: %s\n--- output ---\n%s\n' "$needle" "$haystack" >&2
    exit 1
  }
}

mkdir -p "$FAKE_BIN" "$HOME_DIR"

cat >"${FAKE_BIN}/launchctl" <<'EOF'
#!/usr/bin/env sh
set -eu
case "${1:-}" in
  print)
    [ -f "$CLAM_TEST_LOADED_FILE" ]
    ;;
  bootout)
    rm -f "$CLAM_TEST_LOADED_FILE"
    ;;
  bootstrap)
    touch "$CLAM_TEST_LOADED_FILE"
    ;;
  *)
    echo "fake launchctl: unsupported $*" >&2
    exit 2
    ;;
esac
EOF

cat >"${FAKE_BIN}/pmset" <<'EOF'
#!/usr/bin/env sh
set -eu
case "$*" in
  "-g batt")
    echo "Now drawing from 'Battery Power'"
    ;;
  "-g custom")
    printf 'Battery Power:\n disablesleep 0\n'
    ;;
  "-a disablesleep "*)
    echo "$*" >> "$CLAM_TEST_PMSET_LOG"
    ;;
  "noidle")
    while :; do sleep 60; done
    ;;
  *)
    echo "fake pmset: unsupported $*" >&2
    exit 2
    ;;
esac
EOF

cat >"${FAKE_BIN}/sudo" <<'EOF'
#!/usr/bin/env sh
set -eu
if [ "${1:-}" = "-n" ]; then
  shift
fi
exec "$@"
EOF

cat >"${FAKE_BIN}/ioreg" <<'EOF'
#!/usr/bin/env sh
printf '    | |   "AppleClamshellState" = No\n'
EOF

cat >"${FAKE_BIN}/caffeinate" <<'EOF'
#!/usr/bin/env sh
while :; do sleep 60; done
EOF

chmod +x "${FAKE_BIN}/launchctl" "${FAKE_BIN}/pmset" "${FAKE_BIN}/sudo" "${FAKE_BIN}/ioreg" "${FAKE_BIN}/caffeinate"

export NO_COLOR=1
export CLAM_OS=Darwin
export CLAM_HOME="$HOME_DIR"
export CLAM_LAUNCHCTL="${FAKE_BIN}/launchctl"
export CLAM_PMSET="${FAKE_BIN}/pmset"
export CLAM_SUDO="${FAKE_BIN}/sudo"
export CLAM_IOREG="${FAKE_BIN}/ioreg"
export CLAM_CAFFEINATE="${FAKE_BIN}/caffeinate"
export CLAM_TEST_LOADED_FILE="$LOADED_FILE"
export CLAM_TEST_PMSET_LOG="$PMSET_LOG"

help_output="$("${ROOT}/clam" help)"
assert_contains "$help_output" "clam update"
assert_contains "$help_output" "clam enable [--timeout 4h] [--force]"

status_output="$("${ROOT}/clam" status)"
assert_contains "$status_output" "Clam mode: inactive"
assert_contains "$status_output" "Battery Power"

"${ROOT}/clam" enable --timeout 1m >/dev/null
status_output="$("${ROOT}/clam" status)"
assert_contains "$status_output" "Clam mode: active"
assert_contains "$status_output" "Launch agent:      loaded"
grep -F 'state_timeout_seconds=60' "${HOME_DIR}/Library/Application Support/keep-clam/state.env" >/dev/null ||
  fail "timeout was not written to state"

"${ROOT}/clam" disable >/dev/null
status_output="$("${ROOT}/clam" status)"
assert_contains "$status_output" "Clam mode: inactive"
assert_contains "$status_output" "Launch agent:      not loaded"

"${ROOT}/clam" enable --force >/dev/null
status_output="$("${ROOT}/clam" status)"
assert_contains "$status_output" "Force mode:        on"
"${ROOT}/clam" disable >/dev/null
grep -F -- '-a disablesleep 1' "$PMSET_LOG" >/dev/null || fail "force enable did not call pmset disablesleep 1"
grep -F -- '-a disablesleep 0' "$PMSET_LOG" >/dev/null || fail "disable did not restore pmset disablesleep 0"

json_output="$("${ROOT}/clam" status --json)"
assert_contains "$json_output" '"active": false'
assert_contains "$json_output" '"power_source": "Battery Power"'

INSTALL_DIR="${TMP}/install" KEEP_CLAM_NO_PATH_UPDATE=1 "${ROOT}/install.sh" >/dev/null
[ -x "${TMP}/install/clam" ] || fail "installer did not install clam"
installed_help="$("${TMP}/install/clam" help)"
assert_contains "$installed_help" "Keep Clam and Carry On"

printf 'ok\n'
