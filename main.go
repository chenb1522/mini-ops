package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed web/*
var webFS embed.FS

const (
	defaultPort = "18080"
	defaultUser = "admin"
	appName     = "mini-ops"
)

func main() {
	port := flag.String("port", envOr("MINIOPS_PORT", defaultPort), "监听端口")
	user := flag.String("user", envOr("MINIOPS_USER", defaultUser), "登录用户名")
	pass := flag.String("pass", envOr("MINIOPS_PASS", ""), "登录密码（留空则自动生成）")
	noAuth := flag.Bool("no-auth", false, "禁用登录验证（仅限内网可信环境）")
	flag.Parse()

	if *pass == "" {
		*pass = genPassword()
		log.Printf("[mini-ops] 已生成随机密码，请记录: %s", *pass)
	}

	mux := http.NewServeMux()

	// 静态资源（embed 路径为 web/xxx，需去掉前缀映射到 /static/xxx）
	webSub, _ := fs.Sub(webFS, "web")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(webSub))))

	// 根路径 -> 首页
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, _ := webFS.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// API 路由
	api := http.NewServeMux()
	api.HandleFunc("/api/system", handleSystem)
	api.HandleFunc("/api/stats", handleStats)
	api.HandleFunc("/api/nginx/status", handleNginxStatus)
	api.HandleFunc("/api/nginx/install", handleNginxInstall)
	api.HandleFunc("/api/nginx/restart", handleNginxRestart)
	api.HandleFunc("/api/nginx/config", handleNginxConfig)
	api.HandleFunc("/api/nginx/test", handleNginxTest)

	// 认证中间件
	var handler http.Handler = api
	if !*noAuth {
		handler = authMiddleware(*user, *pass, api)
	}
	mux.Handle("/api/", handler)

	addr := ":" + *port
	log.Printf("[mini-ops] 服务已启动: http://0.0.0.0%s  (用户: %s)", addr, *user)
	log.Printf("[mini-ops] 请通过浏览器访问，默认用户名 %s", *user)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[mini-ops] 启动失败: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func genPassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 12)
	seed := timeSeed()
	for i := range b {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		b[i] = chars[seed%int64(len(chars))]
	}
	return string(b)
}

// authMiddleware 简单的 HTTP Basic Auth
func authMiddleware(user, pass string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || !safeEqual(u, user) || !safeEqual(p, pass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="mini-ops"`)
			http.Error(w, `{"error":"未授权"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func safeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var r byte
	for i := 0; i < len(a); i++ {
		r |= a[i] ^ b[i]
	}
	return r == 0
}

func timeSeed() int64 {
	return int64(os.Getpid()) * 1000003
}

// 工具：从 nginx -t 输出提取错误信息
func cleanErr(err error, output string) string {
	s := strings.TrimSpace(output)
	if s != "" {
		return s
	}
	if err != nil {
		return err.Error()
	}
	return fmt.Sprint("操作失败")
}
