#!/bin/sh
# SysMan 一键安装脚本
# 用法: curl -fsSL https://your-domain/sysman/install.sh | sh
# 或:   sh install.sh [端口] [用户名] [密码]

set -e

PORT="${1:-8888}"
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
BASE_URL="${SYSMAN_BASE_URL:-https://github.com/your-repo/sysman/releases/latest/download}"
BIN_NAME="sysman-linux-${BINARCH}"
INSTALL_DIR="/usr/local/bin"
CONF_DIR="/etc/sysman"

# 需要 root 权限
if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 root 权限运行: sudo sh install.sh"
  exit 1
fi

echo "==> 下载 sysman (${BINARCH})..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o /tmp/sysman "${BASE_URL}/${BIN_NAME}" || {
    echo "下载失败，请检查网络或 SYSMAN_BASE_URL"; exit 1;
  }
elif command -v wget >/dev/null 2>&1; then
  wget -q -O /tmp/sysman "${BASE_URL}/${BIN_NAME}" || {
    echo "下载失败，请检查网络或 SYSMAN_BASE_URL"; exit 1;
  }
else
  echo "需要 curl 或 wget"; exit 1
fi

chmod +x /tmp/sysman

echo "==> 安装到 ${INSTALL_DIR}..."
mv /tmp/sysman "${INSTALL_DIR}/sysman"

# 配置
mkdir -p "${CONF_DIR}"
if [ -n "$PASS" ]; then
  cat > "${CONF_DIR}/env" <<EOF
SYSMAN_PORT=${PORT}
SYSMAN_USER=${USER}
SYSMAN_PASS=${PASS}
EOF
else
  cat > "${CONF_DIR}/env" <<EOF
SYSMAN_PORT=${PORT}
SYSMAN_USER=${USER}
EOF
fi

# 注册 systemd 服务（若有 systemd）
if [ -d /run/systemd/system ]; then
  cat > /etc/systemd/system/sysman.service <<EOF
[Unit]
Description=SysMan Server Manager
After=network.target

[Service]
Type=simple
EnvironmentFile=${CONF_DIR}/env
ExecStart=${INSTALL_DIR}/sysman
Restart=on-failure
RestartSec=3
NoNewPrivileges=false

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable sysman >/dev/null 2>&1
  systemctl start sysman
  echo "==> 已通过 systemd 启动"
else
  # 无 systemd，用 nohup 启动
  nohup "${INSTALL_DIR}/sysman" >/var/log/sysman.log 2>&1 &
  echo "==> 已通过 nohup 启动（日志: /var/log/sysman.log）"
fi

echo ""
echo "======================================"
echo " SysMan 安装完成！"
echo " 访问地址: http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo '服务器IP'):${PORT}"
echo " 用户名: ${USER}"
if [ -n "$PASS" ]; then
  echo " 密码: ${PASS}"
else
  echo " 密码: 启动时自动生成，查看日志: journalctl -u sysman 或 /var/log/sysman.log"
fi
echo "======================================"
