#!/bin/sh
# mini-ops 一键安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/chenb1522/mini-ops/main/install.sh | sudo sh
# 或:   sudo sh install.sh [端口] [用户名] [密码]
#
# 说明：
#   - 不指定密码时，脚本会自动生成随机密码并直接打印在安装结果里
#   - 多次运行是幂等的：已有密码会保留，不会因重复安装而改变

set -e

PORT="${1:-18080}"
USER="${2:-admin}"
PASS="${3:-}"

# 检测架构
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  BINARCH="amd64" ;;
  aarch64|arm64) BINARCH="arm64" ;;
  armv7l)        BINARCH="armv7" ;;
  *) echo "不支持的架构: $ARCH"; exit 1 ;;
esac

# 二进制下载地址（可替换为你的服务器）
BASE_URL="${MINIOPS_BASE_URL:-https://github.com/chenb1522/mini-ops/releases/latest/download}"
BIN_NAME="mini-ops-linux-${BINARCH}"
INSTALL_DIR="/usr/local/bin"
CONF_DIR="/etc/mini-ops"
ENV_FILE="${CONF_DIR}/env"

# 需要 root 权限
if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 root 权限运行: sudo sh install.sh"
  exit 1
fi

# 下载工具选择
if command -v curl >/dev/null 2>&1; then
  DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
  DOWNLOADER="wget"
else
  echo "需要 curl 或 wget"; exit 1
fi

# 下载函数：尝试从某个 URL 下载，成功返回 0
download_from() {
  local url="$1"
  if [ "$DOWNLOADER" = "curl" ]; then
    curl -fsSL --connect-timeout 15 -o /tmp/mini-ops "$url" 2>/dev/null
  else
    wget -q -T 15 -O /tmp/mini-ops "$url" 2>/dev/null
  fi
}

echo "==> 下载 mini-ops (${BINARCH})..."

# 下载源列表：用户自定义源 > 加速镜像（国内） > GitHub 官方源
# 注：加速镜像为第三方公共服务，稳定性不保证；会按顺序逐个尝试，失败自动回退下一个
if [ -n "$MINIOPS_BASE_URL" ]; then
  URLS="${MINIOPS_BASE_URL}/${BIN_NAME}"
else
  URLS="
https://ghfast.top/https://github.com/chenb1522/mini-ops/releases/latest/download/${BIN_NAME}
https://ghproxy.net/https://github.com/chenb1522/mini-ops/releases/latest/download/${BIN_NAME}
https://gh-proxy.com/https://github.com/chenb1522/mini-ops/releases/latest/download/${BIN_NAME}
https://ghproxy.com/https://github.com/chenb1522/mini-ops/releases/latest/download/${BIN_NAME}
https://github.moeyy.xyz/https://github.com/chenb1522/mini-ops/releases/latest/download/${BIN_NAME}
https://github.com/chenb1522/mini-ops/releases/latest/download/${BIN_NAME}
"
fi

DOWNLOAD_OK=0
for URL in $URLS; do
  [ -z "$URL" ] && continue
  echo "    尝试: $URL"
  if download_from "$URL"; then
    DOWNLOAD_OK=1
    break
  fi
done

if [ "$DOWNLOAD_OK" -ne 1 ]; then
  echo "下载失败，请检查网络；或手动下载后放到 ${INSTALL_DIR}/mini-ops"
  exit 1
fi

chmod +x /tmp/mini-ops

echo "==> 安装到 ${INSTALL_DIR}..."
mv /tmp/mini-ops "${INSTALL_DIR}/mini-ops"

# 配置：处理密码保留逻辑
mkdir -p "${CONF_DIR}"

# 如果已有旧配置，读取其中的旧密码（用于重复运行时不丢密码）
OLD_PASS=""
if [ -f "${ENV_FILE}" ]; then
  OLD_PASS=$(grep -E '^MINIOPS_PASS=' "${ENV_FILE}" 2>/dev/null | head -1 | cut -d'=' -f2-)
fi

# 决定最终密码：命令行指定 > 旧配置已有 > 脚本自动生成
if [ -n "$PASS" ]; then
  FINAL_PASS="$PASS"
elif [ -n "$OLD_PASS" ]; then
  FINAL_PASS="$OLD_PASS"
else
  # 生成随机密码（12 位）
  FINAL_PASS=$(head -c 12 /dev/urandom | base64 2>/dev/null | tr -d '/+=' | head -c 12)
  if [ -z "$FINAL_PASS" ]; then
    FINAL_PASS=$(date +%s | md5sum | cut -c1-12)
  fi
fi

cat > "${ENV_FILE}" <<EOF
MINIOPS_PORT=${PORT}
MINIOPS_USER=${USER}
MINIOPS_PASS=${FINAL_PASS}
EOF
echo "==> 使用密码: ${FINAL_PASS}"

# 注册 systemd 服务（若有 systemd）
if [ -d /run/systemd/system ]; then
  cat > /etc/systemd/system/mini-ops.service <<EOF
[Unit]
Description=MiniOps Server Manager
After=network.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
ExecStart=${INSTALL_DIR}/mini-ops
Restart=on-failure
RestartSec=3
NoNewPrivileges=false

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable mini-ops >/dev/null 2>&1 || true
  if systemctl is-active --quiet mini-ops 2>/dev/null; then
    systemctl restart mini-ops
    echo "==> 已重启 systemd 服务"
  else
    systemctl start mini-ops
    echo "==> 已启动 systemd 服务"
  fi
else
  # 无 systemd，用 nohup 启动
  nohup "${INSTALL_DIR}/mini-ops" >/var/log/mini-ops.log 2>&1 &
  echo "==> 已通过 nohup 启动（日志: /var/log/mini-ops.log）"
fi

echo ""
echo "======================================"
echo " mini-ops 安装完成！"
echo " 访问地址: http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo '服务器IP'):${PORT}"
echo " 用户名: ${USER}"
echo " 密码: ${FINAL_PASS}"
echo "======================================"
