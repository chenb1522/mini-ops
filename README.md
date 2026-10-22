# mini-ops — Linux 服务器轻量管理工具

一个极轻量的 Linux 服务器 Web 管理小工具。单文件二进制、无第三方依赖、内存占用极低（约几 MB），一行命令即可安装。

## 功能

- ✅ **机器基本信息**：主机名、发行版、内核、架构、CPU、内存、运行时长、负载、磁盘、网络
- ✅ **实时资源监控**：CPU / 内存使用率（自动刷新）
- ✅ **Nginx 管理**：一键安装、重启、配置编辑（带自动备份与校验回滚）
- ✅ **跨发行版兼容**：自动识别 apt / yum / dnf / apk / pacman / zypper，适配 systemd / sysvinit / openrc
- ✅ **极低资源占用**：Go 静态编译，无运行时依赖，RSS 内存仅几 MB
- ✅ **简单认证**：HTTP Basic Auth，密码可自动生成

## 快速开始

### 方式一：一键安装（推荐）

**国内用户（走加速镜像，任选其一）：**

```bash
# 镜像 1
curl -fsSL https://ghfast.top/https://raw.githubusercontent.com/chenb1522/mini-ops/main/install.sh | sudo sh

# 镜像 2
curl -fsSL https://ghproxy.net/https://raw.githubusercontent.com/chenb1522/mini-ops/main/install.sh | sudo sh

# 镜像 3
curl -fsSL https://gh-proxy.com/https://raw.githubusercontent.com/chenb1522/mini-ops/main/install.sh | sudo sh
```

> 以上镜像均为第三方公共服务，若都不可用，请用下方官方源或改用「方式二」手动下载。

**国外 / 官方源：**

```bash
curl -fsSL https://raw.githubusercontent.com/chenb1522/mini-ops/main/install.sh | sudo sh
```

安装完成后访问 `http://服务器IP:18080`，默认用户名 `admin`（密码在安装结果里直接打印）。

如需指定端口 / 用户名 / 密码：

```bash
sudo sh install.sh 18080 admin mypassword
```

### 方式二：下载二进制手动运行

到 [Releases](https://github.com/chenb1522/mini-ops/releases) 页面下载对应架构的二进制，然后执行：

| 架构 | 文件名 |
|------|--------|
| x86_64 | `mini-ops-linux-amd64` |
| arm64 | `mini-ops-linux-arm64` |
| armv7 | `mini-ops-linux-armv7` |

```bash
# 1. 赋予执行权限
chmod +x mini-ops-linux-amd64

# 2. 直接运行（会打印访问地址和随机密码）
sudo ./mini-ops-linux-amd64

# 3. 指定端口和密码运行
sudo ./mini-ops-linux-amd64 -port 8080 -pass yourpassword
```

> 运行后浏览器打开 `http://服务器IP:端口`，用启动日志里的用户名和密码登录。

### 方式三：手动安装为系统服务

```bash
# 1. 放入系统路径
sudo mv mini-ops-linux-amd64 /usr/local/bin/mini-ops
sudo chmod +x /usr/local/bin/mini-ops

# 2. 后台运行
sudo nohup /usr/local/bin/mini-ops >/var/log/mini-ops.log 2>&1 &
```

## 启动后如何使用

1. 浏览器打开 `http://服务器IP:18080`
2. 输入用户名 `admin`
3. 密码在启动日志中：随机生成会打印 `已生成随机密码，请记录: xxx`；若用 `-pass` 指定则用你设的密码

## 命令行参数

| 参数 | 说明 | 默认 |
|------|------|------|
| `-port` | 监听端口 | `18080` |
| `-user` | 登录用户名 | `admin` |
| `-pass` | 登录密码（留空自动生成） | 随机 |
| `-no-auth` | 禁用认证（仅限可信内网） | 关闭 |

也支持环境变量 `MINIOPS_PORT` / `MINIOPS_USER` / `MINIOPS_PASS` 配置。

示例：

```bash
# 指定端口和密码
./mini-ops-linux-amd64 -port 8080 -pass mypassword

# 内网可信环境，免登录
./mini-ops-linux-amd64 -no-auth

# 环境变量方式
MINIOPS_PORT=8080 MINIOPS_PASS=xxx ./mini-ops-linux-amd64
```

## 本地编译

```bash
cd mini-ops
# 编译当前平台
go build -ldflags="-s -w" -o mini-ops .

# 交叉编译（发布用）
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o mini-ops-linux-amd64 .
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o mini-ops-linux-arm64 .
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -ldflags="-s -w" -o mini-ops-linux-armv7 .
```

## 发布新版本

本项目使用 GitHub Actions 自动构建并发布。**只需打一个 tag 并推送**，GitHub 会自动编译三个架构的二进制并发布到 Releases：

```bash
# 提交你的改动后，打 tag 并推送
git tag v1.0.0
git push origin v1.0.0
```

推送 tag 后，到仓库的 [Actions](https://github.com/chenb1522/mini-ops/actions) 页面可看到自动构建过程，完成后到 [Releases](https://github.com/chenb1522/mini-ops/releases) 即可看到发布好的二进制。

> 也可以手动触发：Actions 页面 → Release workflow → Run workflow。

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/system` | 机器基本信息 |
| GET | `/api/stats` | CPU/内存实时占用 |
| GET | `/api/nginx/status` | nginx 安装状态 |
| POST | `/api/nginx/install` | 安装 nginx |
| POST | `/api/nginx/restart` | 重启 nginx（先自动测试配置） |
| POST | `/api/nginx/test` | 测试 nginx 配置 |
| GET | `/api/nginx/config` | 读取配置 |
| POST | `/api/nginx/config` | 保存配置（自动备份 + 校验回滚） |

## 安全说明

- 默认启用 HTTP Basic Auth，密码可自定义或自动生成
- 配置保存前会先用 `nginx -t` 校验，失败自动回滚并备份
- 建议仅在内网或配合反向代理 + HTTPS 使用

## 目录结构

```
mini-ops/
├── main.go          # 入口 + HTTP 路由 + 认证
├── system.go        # 系统信息采集（/proc 读取）
├── nginx.go         # nginx 管理 + 发行版适配
├── web/             # 前端静态资源（embed 进二进制）
│   ├── index.html
│   ├── style.css
│   └── app.js
├── install.sh       # 一键安装脚本
├── go.mod
└── README.md
```
