#!/usr/bin/env bash
# USCN GPU Agent 1.0.1. Source: github.com/beijingcao/uscn-gpu-agent
# Downloaded packages are pinned by SHA256 before any privileged installation.
set -euo pipefail
uscn_gpu_main() {
  local source=uscn download_only=false
  while test "$#" -gt 0; do
    case "$1" in
      --github) source=github ;;
      --download-only) download_only=true ;;
      -h|--help) echo 'USCN GPU Agent 1.0.1: install.sh [--github] [--download-only]'; return 0 ;;
      *) echo "Unknown option: $1" >&2; return 2 ;;
    esac
    shift
  done
  local archive expected package
  case "$(uname -s):$(uname -m)" in
    Linux:x86_64|Linux:amd64)
      archive=uscn-gpu-agent-linux-amd64.tar.gz
      package=uscn-gpu-agent-linux-amd64
      expected=9654dcda52f6e053c7d227bd20b9573b7c3cac31db5ff22c059c6a387faf648a ;;
    Darwin:arm64|Darwin:x86_64)
      archive=uscn-gpu-agent-macos-universal.zip
      package=uscn-gpu-agent-macos-universal
      expected=37fb545c64262a6b6bdc6caa56b02179fd10cb86b53fd54075757b6ec4c624d3 ;;
    *) echo 'This installer supports Linux x86_64 and macOS Intel/Apple Silicon.' >&2; return 1 ;;
  esac
  local base=https://uscn.si/downloads/gpu-agent/v1.0.1
  if test "$source" = github; then base=https://github.com/beijingcao/uscn-gpu-agent/releases/download/v1.0.1; fi
  command -v curl >/dev/null || { echo 'curl is required.' >&2; return 1; }
  local work
  work=$(mktemp -d "${TMPDIR:-/tmp}/uscn-gpu-install.XXXXXXXX")
  chmod 700 "$work"
  uscn_gpu_work="$work"
  trap 'test -z "${uscn_gpu_work:-}" || rm -rf -- "$uscn_gpu_work"' EXIT
  curl --proto '=https' --proto-redir '=https' --tlsv1.2 --fail --show-error --location --connect-timeout 15 --max-time 180 "$base/$archive" -o "$work/$archive"
  local actual
  if command -v sha256sum >/dev/null; then
    actual=$(sha256sum "$work/$archive"); actual=${actual%% *}
  else
    actual=$(shasum -a 256 "$work/$archive"); actual=${actual%% *}
  fi
  test "$actual" = "$expected" || { echo 'Package SHA256 mismatch; installation stopped.' >&2; return 1; }
  if test "${archive##*.}" = zip; then
    command -v unzip >/dev/null || { echo 'unzip is required.' >&2; return 1; }
    unzip -q "$work/$archive" -d "$work"
  else
    tar -xzf "$work/$archive" -C "$work"
  fi
  test -f "$work/$package/install.sh" && test -f "$work/$package/SHA256SUMS" || { echo 'Invalid package layout.' >&2; return 1; }
  if test "$download_only" = true; then
    echo "Verified USCN GPU Agent 1.0.1: $work/$package"
    trap - EXIT
    return 0
  fi
  echo '请准备GPU上架页的一次性配对码；系统服务安装会请求管理员权限。'
  # curl | bash owns stdin. Pairing must read from the interactive terminal.
  if ! ( : </dev/tty ) 2>/dev/null; then
    echo 'An interactive terminal is required to enter the pairing code. Use --download-only to fetch the package.' >&2
    return 1
  fi
  if test "$(id -u)" = 0; then
    /bin/bash "$work/$package/install.sh" </dev/tty
  else
    sudo /bin/bash "$work/$package/install.sh" </dev/tty
  fi
}
uscn_gpu_main "$@"
