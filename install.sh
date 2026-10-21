#!/usr/bin/env bash
# USCN GPU Agent 1.0.3. Source: github.com/beijingcao/uscn-gpu-agent
# Downloaded packages are pinned by SHA256 before any privileged installation.
set -euo pipefail
uscn_gpu_main() {
  local source=uscn download_only=false action= choice=
  while test "$#" -gt 0; do
    case "$1" in
      --github) source=github ;;
      --download-only) download_only=true ;;
      --install|--uninstall)
        test -z "$action" || { echo 'Select only one action.' >&2; return 2; }
        action="${1#--}" ;;
      -h|--help) echo 'USCN GPU Agent 1.0.3: install.sh [--install | --uninstall | --download-only] [--github]'; return 0 ;;
      *) echo "Unknown option: $1" >&2; return 2 ;;
    esac
    shift
  done
  if test "$download_only" = true && test -n "$action"; then
    echo '--download-only cannot be combined with an install/uninstall action.' >&2; return 2
  fi
  if test "$download_only" = false && test -z "$action"; then
    echo 'USCN GPU Agent 1.0.3' >&2
    echo '  1) 安装 / 升级' >&2
    echo '  2) 卸载' >&2
    echo '  0) 退出' >&2
    if ! ( : </dev/tty ) 2>/dev/null; then
      echo '请在交互终端选择操作，或显式使用 --install / --uninstall / --download-only。' >&2; return 1
    fi
    printf '请选择 [0/1/2，默认0]：' >&2
    IFS= read -r choice </dev/tty || return 1
    case "$choice" in
      1) action=install ;;
      2) action=uninstall ;;
      0|'') echo '已退出。'; return 0 ;;
      *) echo '无效选项，未执行任何操作。' >&2; return 2 ;;
    esac
  fi
  local archive expected package
  case "$(uname -s):$(uname -m)" in
    Linux:x86_64|Linux:amd64)
      archive=uscn-gpu-agent-linux-amd64.tar.gz
      package=uscn-gpu-agent-linux-amd64
      expected=af84b5c1954c88dd5be922dc5da76b6024b4a1f50b4ce04a9382051310bd171d ;;
    Linux:aarch64|Linux:arm64)
      archive=uscn-gpu-agent-linux-arm64.tar.gz
      package=uscn-gpu-agent-linux-arm64
      expected=6c499094e5c459ba47bce48083ab5821be2d4fa1f1fc6f59cf2f3cc09e0cd1fe ;;
    Darwin:arm64|Darwin:x86_64)
      archive=uscn-gpu-agent-macos-universal.zip
      package=uscn-gpu-agent-macos-universal
      expected=50cc6ed13de50454ac3e9c3a40267ccd8dfce89e811a545cb67d59872b421595 ;;
    *) echo 'This installer supports Linux x86_64/ARM64 (DGX Spark) and macOS Intel/Apple Silicon.' >&2; return 1 ;;
  esac
  local base=https://uscn.si/downloads/gpu-agent/v1.0.3
  if test "$source" = github; then base=https://github.com/beijingcao/uscn-gpu-agent/releases/download/v1.0.3; fi
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
    echo "Verified USCN GPU Agent 1.0.3: $work/$package"
    trap - EXIT
    return 0
  fi
  if test "$action" = uninstall; then
    echo '卸载将停止上报并移除本机程序；保留设备身份与待传报告，不解除平台资源或订单。'
    if test "$(id -u)" = 0; then
      /bin/bash "$work/$package/install.sh" --uninstall
    else
      sudo /bin/bash "$work/$package/install.sh" --uninstall
    fi
  else
    echo '请准备GPU上架页的一次性配对码；系统服务安装会请求管理员权限。'
    # curl | bash owns stdin. Pairing must read from the interactive terminal.
    if ! ( : </dev/tty ) 2>/dev/null; then
      echo '安装需要交互终端输入配对码。仅下载请使用 --download-only。' >&2; return 1
    fi
    if test "$(id -u)" = 0; then
      /bin/bash "$work/$package/install.sh" --install </dev/tty
    else
      sudo /bin/bash "$work/$package/install.sh" --install </dev/tty
    fi
  fi
}
uscn_gpu_main "$@"
