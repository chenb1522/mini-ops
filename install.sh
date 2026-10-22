#!/bin/sh
# mini-ops 一键安装脚本
# 用法: curl -fsSL https://raw.githubusercontent.com/chenb1522/mini-ops/main/install.sh | sh
# 或:   sh install.sh [端口] [用户名] [密码]

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

# 需要 root 权限
if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 root 权限运行: sudo sh install.sh"
  exit 1
fi

echo "==> 下载 mini-ops (${BINARCH})..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o /tmp/mini-ops "${BASE_URL}/${BIN_NAME}" || {
    echo "下载失败，请检查网络或 MINIOPS_BASE_URL"; exit 1;
  }
elif command -v wget >/dev/null 2>&1; then
  wget -q -O /tmp/mini-ops "${BASE_URL}/${BIN_NAME}" || {
    echo "下载失败，请检查网络或 MINIOPS_BASE_URL"; exit 1;
  }
else
  echo "需要 curl 或 wget"; exit 1
fi

chmod +x /tmp/mini-ops

echo "==> 安装到 ${INSTALL_DIR}..."
mv /tmp/mini-ops "${INSTALL_DIR}/mini-ops"

# 配置
mkdir -p "${CONF_DIR}"
if [ -n "$PASS" ]; then
  cat > "${CONF_DIR}/env" <<EOF
MINIOPS_PORT=${PORT}
MINIOPS_USER=${USER}
MINIOPS_PASS=${PASS}
EOF
else
  cat > "${CONF_DIR}/env" <<EOF
MINIOPS_PORT=${PORT}
MINIOPS_USER=${USER}
EOF
fi

# 注册 systemd 服务（若有 systemd）
if [ -d /run/systemd/system ]; then
  cat > /etc/systemd/system/mini-ops.service <<EOF
[Unit]
Description=MiniOps Server Manager
After=network.target

[Service]
Type=simple
EnvironmentFile=${CONF_DIR}/env
ExecStart=${INSTALL_DIR}/mini-ops
Restart=on-failure
RestartSec=3
NoNewPrivileges=false

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable mini-ops >/dev/null 2>&1
  systemctl start mini-ops
  echo "==> 已通过 systemd 启动"
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
if [ -n "$PASS" ]; then
  echo " 密码: ${PASS}"
else
  echo " 密码: 启动时自动生成，查看日志: journalctl -u mini-ops 或 /var/log/mini-ops.log"
fi
echo "======================================"
