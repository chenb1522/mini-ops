package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// pkgManager 包管理器信息
type pkgManager struct {
	Install []string // 安装命令
	Update  []string // 更新源命令
	Query   []string // 查询是否已安装
}

// detectPkgManager 检测当前发行版使用的包管理器
func detectPkgManager() pkgManager {
	if pathExists("/etc/alpine-release") {
		return pkgManager{
			Install: []string{"apk", "add", "nginx"},
			Update:  []string{"apk", "update"},
			Query:   []string{"apk", "info", "-e", "nginx"},
		}
	}
	if which("dnf") {
		return pkgManager{
			Install: []string{"dnf", "install", "-y", "nginx"},
			Update:  []string{"dnf", "makecache"},
			Query:   []string{"rpm", "-q", "nginx"},
		}
	}
	if which("yum") {
		return pkgManager{
			Install: []string{"yum", "install", "-y", "nginx"},
			Update:  []string{"yum", "makecache"},
			Query:   []string{"rpm", "-q", "nginx"},
		}
	}
	if which("apt-get") || which("apt") {
		return pkgManager{
			Install: []string{"apt-get", "install", "-y", "nginx"},
			Update:  []string{"apt-get", "update"},
			Query:   []string{"dpkg", "-s", "nginx"},
		}
	}
	if which("pacman") {
		return pkgManager{
			Install: []string{"pacman", "-S", "--noconfirm", "nginx"},
			Update:  []string{"pacman", "-Sy"},
			Query:   []string{"pacman", "-Q", "nginx"},
		}
	}
	if which("zypper") {
		return pkgManager{
			Install: []string{"zypper", "--non-interactive", "install", "nginx"},
			Update:  []string{"zypper", "refresh"},
			Query:   []string{"rpm", "-q", "nginx"},
		}
	}
	return pkgManager{}
}

// serviceManager 服务管理器
func restartNginx() (string, error) {
	var cmd *exec.Cmd
	switch {
	case pathExists("/run/systemd/system"):
		cmd = exec.Command("systemctl", "restart", "nginx")
	case which("service"):
		cmd = exec.Command("service", "nginx", "restart")
	case pathExists("/etc/init.d/nginx"):
		cmd = exec.Command("/etc/init.d/nginx", "restart")
	default:
		// 回退：直接发信号
		out, err := exec.Command("nginx", "-s", "reload").CombinedOutput()
		return string(out), err
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func nginxTest() (string, error) {
	out, err := exec.Command("nginx", "-t").CombinedOutput()
	return string(out), err
}

// nginxInstalled 判断 nginx 是否已安装
func nginxInstalled() bool {
	return which("nginx")
}

// nginxConfigPath 查找 nginx 主配置文件
func nginxConfigPath() string {
	candidates := []string{
		"/etc/nginx/nginx.conf",
		"/usr/local/nginx/conf/nginx.conf",
		"/usr/local/etc/nginx/nginx.conf",
		"/opt/homebrew/etc/nginx/nginx.conf",
	}
	for _, p := range candidates {
		if pathExists(p) {
			return p
		}
	}
	return candidates[0] // 默认 Debian 系
}

func handleNginxStatus(w http.ResponseWriter, r *http.Request) {
	pm := detectPkgManager()
	resp := map[string]interface{}{
		"installed":    nginxInstalled(),
		"config_path":  nginxConfigPath(),
		"pkg_manager":  detectPkgName(pm),
		"nginx_binary": which("nginx"),
	}
	writeJSON(w, resp)
}

func handleNginxInstall(w http.ResponseWriter, r *http.Request) {
	if nginxInstalled() {
		writeJSON(w, map[string]interface{}{"ok": true, "message": "nginx 已安装", "installed": true})
		return
	}
	pm := detectPkgManager()
	if len(pm.Install) == 0 {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "无法识别的包管理器，请手动安装 nginx"})
		return
	}
	// 更新源（忽略失败，源可能已是最新）
	runCmd(pm.Update)
	// 安装
	out, err := runCmd(pm.Install)
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "安装失败: " + cleanErr(err, out)})
		return
	}
	// 尝试启动
	startNginx()
	writeJSON(w, map[string]interface{}{"ok": true, "message": "nginx 安装完成", "installed": nginxInstalled()})
}

func handleNginxRestart(w http.ResponseWriter, r *http.Request) {
	if !nginxInstalled() {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "nginx 未安装"})
		return
	}
	// 先测试配置
	if testOut, err := nginxTest(); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "配置有误，已取消重启:\n" + cleanErr(err, testOut)})
		return
	}
	out, err := restartNginx()
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "重启失败: " + cleanErr(err, out)})
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "message": "nginx 重启成功"})
}

func handleNginxTest(w http.ResponseWriter, r *http.Request) {
	if !nginxInstalled() {
		writeJSON(w, map[string]interface{}{"ok": false, "message": "nginx 未安装"})
		return
	}
	out, err := nginxTest()
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "message": cleanErr(err, out)})
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "message": "配置测试通过:\n" + strings.TrimSpace(out)})
}

// handleNginxConfig 读取/写入 nginx 配置
func handleNginxConfig(w http.ResponseWriter, r *http.Request) {
	path := nginxConfigPath()

	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(path)
		if err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "message": "读取配置失败: " + err.Error(), "path": path})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "path": path, "content": string(data)})

	case http.MethodPost:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "message": "请求解析失败: " + err.Error()})
			return
		}
		// 备份原配置
		backup := path + ".bak." + time.Now().Format("20060102150405")
		if orig, err := os.ReadFile(path); err == nil {
			os.WriteFile(backup, orig, 0644)
		}
		if err := os.WriteFile(path, []byte(req.Content), 0644); err != nil {
			writeJSON(w, map[string]interface{}{"ok": false, "message": "写入失败: " + err.Error()})
			return
		}
		// 校验配置
		if testOut, err := nginxTest(); err != nil {
			// 回滚
			os.WriteFile(path, mustReadFile(backup), 0644)
			writeJSON(w, map[string]interface{}{
				"ok":      false,
				"message": "配置校验失败，已自动回滚:\n" + cleanErr(err, testOut),
				"backup":  backup,
			})
			return
		}
		writeJSON(w, map[string]interface{}{"ok": true, "message": "配置已保存并通过校验", "backup": backup})

	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func detectPkgName(pm pkgManager) string {
	if len(pm.Install) > 0 {
		return pm.Install[0]
	}
	return "unknown"
}

func runCmd(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func startNginx() {
	if pathExists("/run/systemd/system") {
		runCmd([]string{"systemctl", "enable", "nginx"})
		runCmd([]string{"systemctl", "start", "nginx"})
	} else if which("service") {
		runCmd([]string{"service", "nginx", "start"})
	} else {
		runCmd([]string{"nginx"})
	}
}

func which(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func mustReadFile(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

// filepath 保留引用，避免某些平台未使用
var _ = filepath.Join
