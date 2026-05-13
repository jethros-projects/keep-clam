#!/usr/bin/env sh
set -eu

APP_NAME="Keep Clam"
BIN_NAME="clam"
REPO_OWNER="${REPO_OWNER:-jethros-projects}"
REPO_NAME="${REPO_NAME:-keep-clam}"
REF="${REF:-main}"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
ARCHIVE_URL="${ARCHIVE_URL:-https://github.com/${REPO_OWNER}/${REPO_NAME}/archive/${REF}.tar.gz}"
EXPECTED_SHA256="${KEEP_CLAM_SHA256:-${SHA256:-}}"

if [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
  BOLD="$(printf '\033[1m')"
  DIM="$(printf '\033[2m')"
  GREEN="$(printf '\033[32m')"
  YELLOW="$(printf '\033[33m')"
  RED="$(printf '\033[31m')"
  CYAN="$(printf '\033[36m')"
  RESET="$(printf '\033[0m')"
else
  BOLD=""
  DIM=""
  GREEN=""
  YELLOW=""
  RED=""
  CYAN=""
  RESET=""
fi

say() { printf '%s\n' "$*"; }
line() { say "${DIM}------------------------------------------------------------------------${RESET}"; }
step() { say "${CYAN}==>${RESET} ${BOLD}$*${RESET}"; }
ok() { say "${GREEN}ok${RESET}  $*"; }
warn() { say "${YELLOW}note${RESET} $*"; }
fail() { say "${RED}error${RESET} $*" >&2; exit 1; }
need_cmd() { command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"; }

download() {
  url="$1"
  dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    fail "missing curl or wget"
  fi
}

verify_sha256() {
  file="$1"
  if [ -z "$EXPECTED_SHA256" ]; then
    warn "SHA256 verification skipped; set KEEP_CLAM_SHA256 for pinned installs"
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$file" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$file" | awk '{print $1}')"
  else
    fail "missing sha256sum or shasum for checksum verification"
  fi
  [ "$actual" = "$EXPECTED_SHA256" ] || fail "SHA256 mismatch for downloaded archive"
  ok "verified SHA256"
}

install_from_source_dir() {
  source_dir="$1"
  [ -f "${source_dir}/${BIN_NAME}" ] || fail "source archive is missing ${BIN_NAME}"
  cp "${source_dir}/${BIN_NAME}" "$tmp_bin"
}

profile_path() {
  if [ -n "${KEEP_CLAM_PROFILE:-}" ]; then
    printf '%s\n' "$KEEP_CLAM_PROFILE"
    return
  fi
  shell_name="$(basename "${SHELL:-}")"
  case "$shell_name" in
    zsh) printf '%s\n' "${HOME}/.zshrc" ;;
    bash)
      if [ "$(uname -s)" = "Darwin" ]; then
        printf '%s\n' "${HOME}/.bash_profile"
      else
        printf '%s\n' "${HOME}/.bashrc"
      fi
      ;;
    fish) printf '%s\n' "${HOME}/.config/fish/config.fish" ;;
    *) printf '%s\n' "${HOME}/.profile" ;;
  esac
}

add_path_to_profile() {
  profile="$1"
  marker="# >>> keep-clam PATH >>>"
  shell_name="$(basename "${SHELL:-}")"
  mkdir -p "$(dirname "$profile")"
  touch "$profile"
  if grep -F "$marker" "$profile" >/dev/null 2>&1; then
    ok "profile already configured: ${profile}"
    return
  fi
  if [ "$shell_name" = "fish" ]; then
    cat >>"$profile" <<EOF

# >>> keep-clam PATH >>>
if not contains "${INSTALL_DIR}" \$PATH
    set -gx PATH "${INSTALL_DIR}" \$PATH
end
# <<< keep-clam PATH <<<
EOF
  else
    cat >>"$profile" <<EOF

# >>> keep-clam PATH >>>
case ":\$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) export PATH="${INSTALL_DIR}:\$PATH" ;;
esac
# <<< keep-clam PATH <<<
EOF
  fi
  ok "updated profile: ${profile}"
}

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

say "${BOLD}${APP_NAME} installer${RESET}"
line
say "install dir: ${INSTALL_DIR}"

mkdir -p "$INSTALL_DIR"
tmp_bin="${tmp_dir}/${BIN_NAME}"

if [ -f "./${BIN_NAME}" ]; then
  step "using local checkout"
  install_from_source_dir "."
else
  need_cmd tar
  archive="${tmp_dir}/keep-clam.tar.gz"
  source_root="${tmp_dir}/source"
  mkdir -p "$source_root"
  step "downloading ${ARCHIVE_URL}"
  download "$ARCHIVE_URL" "$archive"
  verify_sha256 "$archive"
  tar -xzf "$archive" -C "$source_root"
  set -- "$source_root"/*
  install_from_source_dir "$1"
fi

chmod +x "$tmp_bin"
step "validating executable"
"$tmp_bin" --help >/dev/null

install_path="${INSTALL_DIR}/${BIN_NAME}"
mv "$tmp_bin" "$install_path"
chmod +x "$install_path"
ok "installed ${install_path}"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    path_ready=1
    ok "${INSTALL_DIR} is already on PATH"
    ;;
  *)
    path_ready=0
    say ""
    warn "${BIN_NAME} is installed, but ${INSTALL_DIR} is not on PATH in this shell."
    if [ "${KEEP_CLAM_NO_PATH_UPDATE:-0}" = "1" ]; then
      warn "profile update skipped because KEEP_CLAM_NO_PATH_UPDATE=1"
    else
      profile="$(profile_path)"
      add_path_to_profile "$profile"
    fi
    say ""
    say "${BOLD}Use it right now:${RESET}"
    say "  ${YELLOW}export PATH=\"${INSTALL_DIR}:\$PATH\"${RESET}"
    say "  ${GREEN}${BIN_NAME} status${RESET}"
    ;;
esac

say ""
"$install_path" --version >/dev/null
"$install_path" --help >/dev/null
line
ok "done"
if [ "$path_ready" = "1" ]; then
  say "run: ${GREEN}${BIN_NAME} status${RESET}"
  say "try: ${GREEN}${BIN_NAME} enable --force${RESET}"
else
  say "current terminal:"
  say "  ${YELLOW}export PATH=\"${INSTALL_DIR}:\$PATH\"${RESET}"
  say "  ${GREEN}${BIN_NAME} status${RESET}"
  say ""
  say "new terminal:"
  say "  ${GREEN}${BIN_NAME} status${RESET}"
fi
