// Package api 代理池 HTTP API: /get /all /count
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/A0be/clash-pool/internal/parse"
	"github.com/A0be/clash-pool/internal/pool"
)

// Listen 启动 HTTP API(阻塞直到服务退出)
func Listen(p *pool.Pool, addr, token string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		handleGet(w, r, p, token)
	})
	mux.HandleFunc("/all", func(w http.ResponseWriter, r *http.Request) {
		handleAll(w, r, p, token)
	})
	mux.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		handleCount(w, r, p, token)
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("HTTP API 异常退出: %v", err)
	}
}

// nodeJSON 对外暴露的节点信息
type nodeJSON struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Server  string `json:"server"`
	Port    int    `json:"port"`
	Region  string `json:"region"`
	DelayMS int    `json:"delay_ms"`
	Link    string `json:"link,omitempty"` // 分享链接(http/direct 等类型为空)
}

func toNodeJSON(n pool.Node) nodeJSON {
	return nodeJSON{
		Name:    n.Name,
		Type:    n.Type,
		Server:  parse.Str(n.Proxy, "server"),
		Port:    parse.ToInt(n.Proxy["port"]),
		Region:  n.Region,
		DelayMS: n.DelayMS,
		Link:    n.Link,
	}
}

func handleGet(w http.ResponseWriter, r *http.Request, p *pool.Pool, token string) {
	if !authorized(r, token) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	q := r.URL.Query()
	n := p.Get(q.Get("region"), q.Get("type"))
	if n == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "无匹配节点"})
		return
	}
	writeJSON(w, http.StatusOK, toNodeJSON(*n))
}

func handleAll(w http.ResponseWriter, r *http.Request, p *pool.Pool, token string) {
	if !authorized(r, token) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	q := r.URL.Query()
	nodes := p.Filter(q.Get("region"), q.Get("type"))
	out := make([]nodeJSON, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, toNodeJSON(n))
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(out), "nodes": out})
}

func handleCount(w http.ResponseWriter, r *http.Request, p *pool.Pool, token string) {
	if !authorized(r, token) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	q := r.URL.Query()
	writeJSON(w, http.StatusOK, map[string]any{"count": p.Count(q.Get("region"), q.Get("type"))})
}

// authorized 校验访问令牌: ?token= 查询参数或 X-API-Token 头
func authorized(r *http.Request, token string) bool {
	if token == "" {
		return true
	}
	return r.URL.Query().Get("token") == token || r.Header.Get("X-API-Token") == token
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
