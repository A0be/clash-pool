package pool

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/A0be/clash-pool/internal/parse"
)

// ShareLink 把代理节点编码为分享链接, 不支持的类型返回空串
func ShareLink(p map[string]any) string {
	switch parse.Str(p, "type") {
	case "ss":
		return ssLink(p)
	case "vmess":
		return vmessLink(p)
	case "trojan":
		return trojanLink(p)
	case "vless":
		return vlessLink(p)
	case "hysteria2":
		return hy2Link(p)
	}
	return ""
}

// hostPort 拼接 server:port, IPv6 地址加方括号
func hostPort(p map[string]any) string {
	s := parse.Str(p, "server")
	if strings.Contains(s, ":") {
		s = "[" + s + "]"
	}
	return fmt.Sprintf("%s:%d", s, parse.ToInt(p["port"]))
}

// alpnString 把 alpn 字段([]any 或 []string)拼为逗号分隔字符串
func alpnString(p map[string]any) string {
	switch v := p["alpn"].(type) {
	case []any:
		parts := make([]string, 0, len(v))
		for _, x := range v {
			parts = append(parts, fmt.Sprint(x))
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(v, ",")
	case string:
		return v
	}
	return ""
}

// wsOpts 取 ws-opts 中的 path 与 Host 头
func wsOpts(p map[string]any) (path, host string) {
	m, ok := p["ws-opts"].(map[string]any)
	if !ok {
		return
	}
	path = parse.Str(m, "path")
	if h, ok := m["headers"].(map[string]any); ok {
		host = parse.Str(h, "Host")
	}
	return
}

// grpcService 取 grpc-opts 服务名
func grpcService(p map[string]any) string {
	m, ok := p["grpc-opts"].(map[string]any)
	if !ok {
		return ""
	}
	return parse.Str(m, "grpc-service-name")
}

// addNetQuery 把传输层(ws/grpc)参数写入查询串
func addNetQuery(q url.Values, p map[string]any) {
	switch parse.Str(p, "network") {
	case "ws":
		q.Set("type", "ws")
		path, host := wsOpts(p)
		if path != "" {
			q.Set("path", path)
		}
		if host != "" {
			q.Set("host", host)
		}
	case "grpc":
		q.Set("type", "grpc")
		if s := grpcService(p); s != "" {
			q.Set("serviceName", s)
		}
	}
}

// ssLink SIP002: ss://base64(method:password)@host:port#name
func ssLink(p map[string]any) string {
	user := base64.RawURLEncoding.EncodeToString([]byte(
		parse.Str(p, "cipher") + ":" + parse.Str(p, "password")))
	return fmt.Sprintf("ss://%s@%s#%s", user, hostPort(p), url.PathEscape(parse.Str(p, "name")))
}

// vmessLink v2rayN 格式: vmess://base64(json)
func vmessLink(p map[string]any) string {
	network := parse.Str(p, "network")
	if network == "" {
		network = "tcp"
	}
	cipher := parse.Str(p, "cipher")
	if cipher == "" {
		cipher = "auto"
	}
	v := map[string]any{
		"ps":   parse.Str(p, "name"),
		"add":  parse.Str(p, "server"),
		"port": fmt.Sprintf("%d", parse.ToInt(p["port"])),
		"id":   parse.Str(p, "uuid"),
		"aid":  fmt.Sprintf("%d", parse.ToInt(p["alterId"])),
		"scy":  cipher,
		"net":  network,
		"tls":  "",
	}
	if p["tls"] == true {
		v["tls"] = "tls"
	}
	if s := parse.Str(p, "servername"); s != "" {
		v["sni"] = s
	}
	if a := alpnString(p); a != "" {
		v["alpn"] = a
	}
	if fp := parse.Str(p, "client-fingerprint"); fp != "" {
		v["fp"] = fp
	}
	switch network {
	case "ws":
		path, host := wsOpts(p)
		v["path"] = path
		v["host"] = host
	case "grpc":
		v["path"] = grpcService(p)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(b)
}

// trojanLink trojan://password@host:port?params#name
func trojanLink(p map[string]any) string {
	q := url.Values{}
	if v := parse.Str(p, "sni"); v != "" {
		q.Set("sni", v)
	}
	if a := alpnString(p); a != "" {
		q.Set("alpn", a)
	}
	if v := parse.Str(p, "client-fingerprint"); v != "" {
		q.Set("fp", v)
	}
	if p["skip-cert-verify"] == true {
		q.Set("allowInsecure", "1")
	}
	addNetQuery(q, p)
	u := url.URL{
		Scheme:   "trojan",
		User:     url.User(parse.Str(p, "password")),
		Host:     hostPort(p),
		RawQuery: q.Encode(),
		Fragment: parse.Str(p, "name"),
	}
	return u.String()
}

// vlessLink vless://uuid@host:port?params#name
func vlessLink(p map[string]any) string {
	q := url.Values{}
	if v := parse.Str(p, "flow"); v != "" {
		q.Set("flow", v)
	}
	if v := parse.Str(p, "client-fingerprint"); v != "" {
		q.Set("fp", v)
	}
	security := ""
	if p["tls"] == true {
		security = "tls"
	}
	if ro, ok := p["reality-opts"].(map[string]any); ok {
		security = "reality"
		if v := parse.Str(ro, "public-key"); v != "" {
			q.Set("pbk", v)
		}
		if v := parse.Str(ro, "short-id"); v != "" {
			q.Set("sid", v)
		}
	}
	if security != "" {
		q.Set("security", security)
		if v := parse.Str(p, "servername"); v != "" {
			q.Set("sni", v)
		}
		if a := alpnString(p); a != "" {
			q.Set("alpn", a)
		}
	}
	addNetQuery(q, p)
	u := url.URL{
		Scheme:   "vless",
		User:     url.User(parse.Str(p, "uuid")),
		Host:     hostPort(p),
		RawQuery: q.Encode(),
		Fragment: parse.Str(p, "name"),
	}
	return u.String()
}

// hy2Link hysteria2://password@host:port?params#name
func hy2Link(p map[string]any) string {
	q := url.Values{}
	if v := parse.Str(p, "sni"); v != "" {
		q.Set("sni", v)
	}
	if p["skip-cert-verify"] == true {
		q.Set("insecure", "1")
	}
	if v := parse.Str(p, "obfs"); v != "" {
		q.Set("obfs", v)
	}
	if v := parse.Str(p, "obfs-password"); v != "" {
		q.Set("obfs-password", v)
	}
	u := url.URL{
		Scheme:   "hysteria2",
		User:     url.User(parse.Str(p, "password")),
		Host:     hostPort(p),
		RawQuery: q.Encode(),
		Fragment: parse.Str(p, "name"),
	}
	return u.String()
}
