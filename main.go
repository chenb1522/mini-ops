package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
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
)

func main() {
	port := flag.String("port", envOr("MINIOPS_PORT", defaultPort), "监听端口")
	pass := flag.String("pass", envOr("MINIOPS_PASS", ""), "登录密码（留空则自动生成）")
	noAuth := flag.Bool("no-auth", false, "禁用登录验证（仅限内网可信环境）")
	uninstall := flag.Bool("uninstall", false, "卸载 mini-ops（停止服务并删除相关文件）")
	flag.Parse()

	if *uninstall {
		doUninstall()
		return
	}

	if *pass == "" {
		*pass = genPassword()
		log.Printf("[mini-ops] 已生成随机密码，请记录: %s", *pass)
	}

	mux := http.NewServeMux()

	// 静态资源（embed 路径为 web/xxx，需去掉前缀映射到 /static/xxx）
	webSub, _ := fs.Sub(webFS, "web")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(webSub))))

	// 登录页（无需认证）
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		data, _ := webFS.ReadFile("web/login.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// 首页处理器
	indexHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, _ := webFS.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}

	// API 路由
	api := http.NewServeMux()
	api.HandleFunc("/api/system", handleSystem)
	api.HandleFunc("/api/stats", handleStats)
	api.HandleFunc("/api/nginx/status", handleNginxStatus)
	api.HandleFunc("/api/nginx/install", handleNginxInstall)
	api.HandleFunc("/api/nginx/restart", handleNginxRestart)
	api.HandleFunc("/api/nginx/config", handleNginxConfig)
	api.HandleFunc("/api/nginx/test", handleNginxTest)

	if *noAuth {
		mux.Handle("/api/", api)
		mux.HandleFunc("/", indexHandler)
	} else {
		// 登录接口（校验通过则种 cookie）
		mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, map[string]interface{}{"ok": false, "message": "请求格式错误"})
				return
			}
			if !safeEqual(req.Password, *pass) {
				writeJSON(w, map[string]interface{}{"ok": false, "message": "密码错误"})
				return
			}
			token := sessionToken(*pass)
			http.SetCookie(w, &http.Cookie{
				Name:     "miniops_token",
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				MaxAge:   86400 * 7,
			})
			writeJSON(w, map[string]interface{}{"ok": true})
		})

		// 认证中间件（cookie 或 Basic Auth 均可）
		mux.Handle("/api/", authMiddleware(*pass, api))

		// 覆盖根路径：未登录时跳转登录页
		mux.HandleFunc("/", indexHandlerWithAuth(*pass))
	}

	addr := ":" + *port
	log.Printf("[mini-ops] 服务已启动: http://0.0.0.0%s", addr)
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

// authMiddleware 认证中间件：支持 cookie token 或 HTTP Basic Auth
func authMiddleware(pass string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if checkAuth(r, pass) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"未授权"}`))
	})
}

func checkAuth(r *http.Request, pass string) bool {
	// 1. cookie token
	if c, err := r.Cookie("miniops_token"); err == nil && safeEqual(c.Value, sessionToken(pass)) {
		return true
	}
	// 2. Basic Auth（兼容 API 工具调用，密码作为密码位）
	if _, p, ok := r.BasicAuth(); ok && safeEqual(p, pass) {
		return true
	}
	return false
}

// indexHandlerWithAuth 带登录检查的首页处理器
func indexHandlerWithAuth(pass string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !checkAuth(r, pass) {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		data, _ := webFS.ReadFile("web/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}

func sessionToken(pass string) string {
	sum := sha256.Sum256([]byte("miniops:" + pass))
	return hex.EncodeToString(sum[:16])
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
