#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"
sudo /bin/bash ./install.sh
echo '按回车关闭窗口。'
read -r
