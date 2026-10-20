#!/usr/bin/env bash
# Invoked explicitly by the server owner. No drivers, reboots or workloads.
set -euo pipefail
umask 077
package_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
action="${1:-}"
if test "$#" -gt 1; then echo 'Usage: install.sh [--install|--uninstall]' >&2; exit 2; fi
if test -z "$action"; then
  printf 'USCN GPU Agent\n  1) 安装 / 升级\n  2) 卸载（保留设备身份与待传报告）\n  0) 退出\n请选择 [0/1/2，默认0]：'
  IFS= read -r choice </dev/tty
  case "$choice" in 1) action=--install ;; 2) action=--uninstall ;; 0|'') exit 0 ;; *) echo '无效选项，未执行任何操作。' >&2; exit 2 ;; esac
fi
case "$action" in --install|--uninstall) ;; *) echo 'Usage: install.sh [--install|--uninstall]' >&2; exit 2 ;; esac
test "$(id -u)" = 0 || { echo '请使用 sudo bash install.sh 管理系统后台服务。' >&2; exit 1; }
if test "$action" = --uninstall; then
  binary=/usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent
  cli=/usr/local/bin/uscn-gpu-agent
  # Refuse symlinked product directories/files; never recursively delete state,
  # service accounts, shared parent directories, or files outside this product.
  for path in /usr/local/libexec/uscn-gpu-agent "$binary"; do
    test ! -L "$path" || { echo "拒绝卸载：路径为符号链接 $path" >&2; exit 1; }
  done
  if test "$(uname -s)" = Darwin; then
    unit=/Library/LaunchDaemons/si.uscn.gpu-agent.plist
    if test -e "$unit" || test -L "$unit"; then
      test -f "$unit" && test ! -L "$unit" || { echo '插件服务文件异常，停止卸载。' >&2; exit 1; }
      test "$(/usr/libexec/PlistBuddy -c 'Print :Label' "$unit")" = si.uscn.gpu-agent &&
      test "$(/usr/libexec/PlistBuddy -c 'Print :UserName' "$unit")" = _uscn_gpu &&
      test "$(/usr/libexec/PlistBuddy -c 'Print :ProgramArguments:0' "$unit")" = "$binary" || { echo '服务文件与USCN GPU插件不一致，停止卸载。' >&2; exit 1; }
    fi
    if /bin/launchctl print system/si.uscn.gpu-agent >/dev/null 2>&1; then
      /bin/launchctl bootout system/si.uscn.gpu-agent
    fi
    /bin/rm -f -- "$unit"
  elif test "$(uname -s)" = Linux; then
    unit=/etc/systemd/system/uscn-gpu-agent.service
    if test -e "$unit" || test -L "$unit"; then
      test -f "$unit" && test ! -L "$unit" || { echo '插件服务文件异常，停止卸载。' >&2; exit 1; }
      grep -Fqx 'ExecStart=/usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent service --config /var/lib/uscn-gpu-agent/device.json' "$unit" &&
      grep -Fqx 'User=uscn-gpu' "$unit" || { echo '服务文件与USCN GPU插件不一致，停止卸载。' >&2; exit 1; }
      systemctl stop uscn-gpu-agent.service
      systemctl disable uscn-gpu-agent.service
      /bin/rm -f -- "$unit"
      systemctl daemon-reload
    elif systemctl is-active --quiet uscn-gpu-agent.service; then
      systemctl stop uscn-gpu-agent.service
    fi
  else
    echo '本安装程序仅支持Linux与macOS。' >&2; exit 1
  fi
  if test -L "$cli"; then
    if test "$(readlink "$cli")" = "$binary"; then /bin/rm -f -- "$cli"; else echo '已有其他CLI链接，已保留。'; fi
  elif test -e "$cli"; then echo '已有其他CLI文件，已保留。'; fi
  test ! -e "$binary" || test -f "$binary" || { echo '插件程序路径异常，停止卸载。' >&2; exit 1; }
  /bin/rm -f -- "$binary"
  /bin/rmdir /usr/local/libexec/uscn-gpu-agent 2>/dev/null || true
  echo 'USCN GPU插件已卸载或未安装。设备身份、待传报告及专用账户已保留，可重新安装恢复。'
  echo '设备将停止上报；平台资源、审核记录和订单义务不因本机卸载而解除。'
  exit 0
fi
platform="${USCN_GPU_PLATFORM:-https://uscn.si}"
cli=/usr/local/bin/uscn-gpu-agent
if test -e "$cli" || test -L "$cli"; then
  test -L "$cli" && test "$(readlink "$cli")" = /usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent || { echo "已有同名CLI，请先由管理员核对：$cli" >&2; exit 1; }
fi
binary="$package_dir/uscn-gpu-agent"
test -f "$binary" && test ! -L "$binary"
if test "$(uname -s)" = Darwin; then
  (cd "$package_dir" && /usr/bin/shasum -a 256 -c SHA256SUMS)
  account='_uscn_gpu'
  if ! /usr/bin/id "$account" >/dev/null 2>&1; then
    uid_candidate=400
    while /usr/bin/dscl . -search /Users UniqueID "$uid_candidate" | /usr/bin/grep -q .; do
      uid_candidate=$((uid_candidate+1))
      test "$uid_candidate" -lt 500 || { echo '无法分配插件专用系统用户。' >&2; exit 1; }
    done
    /usr/bin/dscl . -create "/Users/$account"
    /usr/bin/dscl . -create "/Users/$account" UniqueID "$uid_candidate"
    /usr/bin/dscl . -create "/Users/$account" PrimaryGroupID 20
    /usr/bin/dscl . -create "/Users/$account" UserShell /usr/bin/false
    /usr/bin/dscl . -create "/Users/$account" NFSHomeDirectory /var/db/uscn-gpu-agent
    /usr/bin/dscl . -create "/Users/$account" RealName 'USCN GPU Agent'
    /usr/bin/dscl . -create "/Users/$account" IsHidden 1
  fi
  state=/var/db/uscn-gpu-agent
  /bin/mkdir -p "$state"
  # The service runs as _uscn_gpu. umask 077 protects state, but must not
  # prevent that account from traversing the root-owned program directories.
  /usr/bin/install -d -m 755 /usr/local/libexec /usr/local/libexec/uscn-gpu-agent /usr/local/bin
  /usr/sbin/chown "$account":staff "$state"
  /bin/chmod 700 "$state"
  /usr/bin/install -m 755 "$binary" /usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent
  if test ! -f "$state/device.json"; then
    echo '请粘贴USCN GPU上架页面生成的一次性配对码：'
    IFS= read -r -s pairing_code
    echo
    /usr/bin/printf '%s\n' "$pairing_code" | /usr/bin/sudo -u "$account" /usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent enroll --config "$state/device.json" --platform "$platform"
    unset pairing_code
  fi
  /bin/cat > /Library/LaunchDaemons/si.uscn.gpu-agent.plist <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>si.uscn.gpu-agent</string>
<key>UserName</key><string>_uscn_gpu</string>
<key>ProgramArguments</key><array><string>/usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent</string><string>service</string><string>--config</string><string>/var/db/uscn-gpu-agent/device.json</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
<key>ThrottleInterval</key><integer>30</integer>
<key>ProcessType</key><string>Background</string>
<key>EnvironmentVariables</key><dict><key>PATH</key><string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string></dict>
</dict></plist>
PLIST
  /usr/sbin/chown root:wheel /Library/LaunchDaemons/si.uscn.gpu-agent.plist
  /bin/chmod 644 /Library/LaunchDaemons/si.uscn.gpu-agent.plist
  /bin/launchctl bootout system/si.uscn.gpu-agent 2>/dev/null || true
  /bin/launchctl bootstrap system /Library/LaunchDaemons/si.uscn.gpu-agent.plist
elif test "$(uname -s)" = Linux; then
  (cd "$package_dir" && sha256sum -c SHA256SUMS)
  account=uscn-gpu
  id "$account" >/dev/null 2>&1 || useradd --system --home-dir /var/lib/uscn-gpu-agent --shell /usr/sbin/nologin "$account"
  state=/var/lib/uscn-gpu-agent
  install -d -m 700 -o "$account" -g "$account" "$state"
  install -d -m 755 /usr/local/libexec/uscn-gpu-agent
  install -m 755 "$binary" /usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent
  if test ! -f "$state/device.json"; then
    echo '请粘贴USCN GPU上架页面生成的一次性配对码：'
    IFS= read -r -s pairing_code
    echo
    printf '%s\n' "$pairing_code" | runuser -u "$account" -- /usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent enroll --config "$state/device.json" --platform "$platform"
    unset pairing_code
  fi
  cat > /etc/systemd/system/uscn-gpu-agent.service <<'UNIT'
[Unit]
Description=USCN GPU hardware inventory and reporting
Wants=network-online.target
After=network-online.target
[Service]
Type=simple
User=uscn-gpu
Group=uscn-gpu
ExecStart=/usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent service --config /var/lib/uscn-gpu-agent/device.json
Restart=on-failure
RestartSec=30
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/uscn-gpu-agent
UMask=0077
[Install]
WantedBy=multi-user.target
UNIT
  systemctl daemon-reload
  systemctl enable uscn-gpu-agent.service
  systemctl restart uscn-gpu-agent.service
else
  echo '本安装程序仅支持Linux与macOS。' >&2
  exit 1
fi
test -L "$cli" || ln -s /usr/local/libexec/uscn-gpu-agent/uscn-gpu-agent "$cli"
echo 'USCN GPU插件已安装。可运行 uscn-gpu-agent version；请回到上架页查看设备采集结果。'
