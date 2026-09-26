package parse

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// parseLink 解析单条分享链接, 支持 ss/vmess/trojan/vless/hysteria2
func parseLink(link string) (map[string]any, error) {
	switch {
	case strings.HasPrefix(link, "ss://"):
		return parseSS(link)
	case strings.HasPrefix(link, "vmess://"):
		return parseVMess(link)
	case strings.HasPrefix(link, "trojan://"):
		return parseTrojan(link)
	case strings.HasPrefix(link, "vless://"):
		return parseVless(link)
	case strings.HasPrefix(link, "hysteria2://"), strings.HasPrefix(link, "hy2://"):
		return parseHysteria2(link)
	}
	return nil, fmt.Errorf("不支持的协议: %s", link)
}

// parseSS 解析 ss:// 链接, 兼容 SIP002(base64 或明文 userinfo) 与旧式整体 base64
func parseSS(link string) (map[string]any, error) {
	rest := strings.TrimPrefix(link, "ss://")

	// 取出 # 后的节点名
	name := ""
	if i := strings.Index(rest, "#"); i >= 0 {
		name = unescapeOrRaw(rest[i+1:])
		rest = rest[:i]
	}
	// 丢弃 ? 后的插件等参数
	if i := strings.Index(rest, "?"); i >= 0 {
		rest = rest[:i]
	}

	var cred, hostPort string
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		cred, hostPort = rest[:at], rest[at+1:]
		if strings.Contains(cred, ":") {
			// 明文 userinfo: method:password
			cred = unescapeOrRaw(cred)
		} else if dec, ok := decodeB64(cred); ok {
			// SIP002 base64 userinfo: base64(method:password)
			cred = dec
		}
	} else {
		// 旧式: 整体为 base64(method:password@host:port)
		dec, ok := decodeB64(rest)
		if !ok {
			return nil, errors.New("ss 链接解析失败: 无法解码")
		}
		at := strings.LastIndexByte(dec, '@')
		if at < 0 {
			return nil, errors.New("ss 链接缺少服务器地址")
		}
		cred, hostPort = dec[:at], dec[at+1:]
	}

	ci := strings.IndexByte(cred, ':')
	if ci < 0 {
		return nil, errors.New("ss 链接缺少加密方式或密码")
	}
	method, password := cred[:ci], cred[ci+1:]

	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return nil, fmt.Errorf("ss 服务器地址无效: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("ss 端口无效: %w", err)
	}

	return map[string]any{
		"name": name, "type": "ss", "server": host, "port": port,
		"cipher": method, "password": password, "udp": true,
	}, nil
}

// parseVMess 解析 vmess:// 链接(base64 编码的 v2rayN JSON)
func parseVMess(link string) (map[string]any, error) {
	dec, ok := decodeB64(strings.TrimPrefix(link, "vmess://"))
	if !ok {
		return nil, errors.New("vmess 链接 base64 解码失败")
	}

	var v struct {
		PS   string `json:"ps"`   // 节点名
		Add  string `json:"add"`  // 服务器
		Port any    `json:"port"` // 端口(字符串或数字)
		ID   string `json:"id"`   // UUID
		Aid  any    `json:"aid"`  // alterId
		Scy  string `json:"scy"`  // 加密方式
		Net  string `json:"net"`  // 传输层: tcp/ws/grpc/h2
		Host string `json:"host"` // ws/h2 伪装域名
		Path string `json:"path"` // ws/h2 路径 / grpc 服务名
		TLS  string `json:"tls"`  // "tls" 表示开启
		SNI  string `json:"sni"`
		Alpn string `json:"alpn"`
		FP   string `json:"fp"` // 指纹
	}
	if err := json.Unmarshal([]byte(dec), &v); err != nil {
		return nil, fmt.Errorf("vmess JSON 解析失败: %w", err)
	}
	if v.Add == "" || v.ID == "" {
		return nil, errors.New("vmess 链接缺少服务器或 UUID")
	}

	port := ToInt(v.Port)
	if port == 0 {
		return nil, errors.New("vmess 链接端口无效")
	}
	cipher := v.Scy
	if cipher == "" {
		cipher = "auto"
	}

	m := map[string]any{
		"name": v.PS, "type": "vmess", "server": v.Add, "port": port,
		"uuid": v.ID, "alterId": ToInt(v.Aid), "cipher": cipher, "udp": true,
	}

	if v.TLS == "tls" {
		m["tls"] = true
		if v.SNI != "" {
			m["servername"] = v.SNI
		}
		if v.Alpn != "" {
			m["alpn"] = strings.Split(v.Alpn, ",")
		}
		if v.FP != "" {
			m["client-fingerprint"] = v.FP
		}
	}

	switch v.Net {
	case "ws":
		m["network"] = "ws"
		ws := map[string]any{"path": v.Path}
		if v.Host != "" {
			ws["headers"] = map[string]any{"Host": v.Host}
		}
		m["ws-opts"] = ws
	case "grpc":
		m["network"] = "grpc"
		m["grpc-opts"] = map[string]any{"grpc-service-name": v.Path}
	case "h2":
		m["network"] = "h2"
		h2 := map[string]any{"path": v.Path}
		if v.Host != "" {
			h2["host"] = []string{v.Host}
		}
		m["h2-opts"] = h2
	}
	return m, nil
}

// parseTrojan 解析 trojan:// 链接
func parseTrojan(link string) (map[string]any, error) {
	u, err := url.Parse(link)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("trojan 链接格式无效")
	}
	password := userInfoAll(u)
	if password == "" {
		return nil, errors.New("trojan 链接缺少密码")
	}
	port := ToInt(u.Port())
	if port == 0 {
		port = 443
	}

	m := map[string]any{
		"name": u.Fragment, "type": "trojan", "server": u.Hostname(),
		"port": port, "password": password, "udp": true,
	}
	q := u.Query()
	if v := q.Get("sni"); v != "" {
		m["sni"] = v
	}
	if v := q.Get("alpn"); v != "" {
		m["alpn"] = strings.Split(v, ",")
	}
	if v := q.Get("fp"); v != "" {
		m["client-fingerprint"] = v
	}
	if isTrue(q.Get("allowInsecure")) || isTrue(q.Get("insecure")) || isTrue(q.Get("skip-cert-verify")) {
		m["skip-cert-verify"] = true
	}
	applyTransport(m, q.Get("type"), q.Get("path"), q.Get("host"), q.Get("serviceName"))
	return m, nil
}

// parseVless 解析 vless:// 链接
func parseVless(link string) (map[string]any, error) {
	u, err := url.Parse(link)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("vless 链接格式无效")
	}
	uuid := u.User.Username()
	if uuid == "" {
		return nil, errors.New("vless 链接缺少 UUID")
	}
	port := ToInt(u.Port())
	if port == 0 {
		port = 443
	}

	m := map[string]any{
		"name": u.Fragment, "type": "vless", "server": u.Hostname(),
		"port": port, "uuid": uuid, "udp": true,
	}
	q := u.Query()
	if v := q.Get("flow"); v != "" {
		m["flow"] = v
	}
	if v := q.Get("fp"); v != "" {
		m["client-fingerprint"] = v
	}
	switch security := q.Get("security"); security {
	case "tls", "reality":
		m["tls"] = true
		if v := q.Get("sni"); v != "" {
			m["servername"] = v
		}
		if v := q.Get("alpn"); v != "" {
			m["alpn"] = strings.Split(v, ",")
		}
		if security == "reality" {
			ro := map[string]any{}
			if v := q.Get("pbk"); v != "" {
				ro["public-key"] = v
			}
			if v := q.Get("sid"); v != "" {
				ro["short-id"] = v
			}
			m["reality-opts"] = ro
		}
	}
	applyTransport(m, q.Get("type"), q.Get("path"), q.Get("host"), q.Get("serviceName"))
	return m, nil
}

// parseHysteria2 解析 hysteria2:// / hy2:// 链接
func parseHysteria2(link string) (map[string]any, error) {
	u, err := url.Parse(link)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("hysteria2 链接格式无效")
	}
	auth := userInfoAll(u)
	if auth == "" {
		return nil, errors.New("hysteria2 链接缺少认证密码")
	}
	port := ToInt(u.Port())
	if port == 0 {
		port = 443
	}

	m := map[string]any{
		"name": u.Fragment, "type": "hysteria2", "server": u.Hostname(),
		"port": port, "password": auth, "udp": true,
	}
	q := u.Query()
	if v := q.Get("sni"); v != "" {
		m["sni"] = v
	}
	if isTrue(q.Get("insecure")) || isTrue(q.Get("allowInsecure")) {
		m["skip-cert-verify"] = true
	}
	if v := q.Get("obfs"); v != "" {
		m["obfs"] = v
		if p := q.Get("obfs-password"); p != "" {
			m["obfs-password"] = p
		}
	}
	return m, nil
}

// applyTransport 按 type 参数写入 ws/grpc 传输层选项
func applyTransport(m map[string]any, network, path, host, serviceName string) {
	switch network {
	case "ws":
		m["network"] = "ws"
		ws := map[string]any{"path": path}
		if host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		m["ws-opts"] = ws
	case "grpc":
		m["network"] = "grpc"
		m["grpc-opts"] = map[string]any{"grpc-service-name": serviceName}
	}
}

// userInfoAll 取出 userinfo 全部内容(兼容密码中含有冒号的情况)
func userInfoAll(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	s := u.User.Username()
	if extra, ok := u.User.Password(); ok {
		s += ":" + extra
	}
	return s
}

func isTrue(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func unescapeOrRaw(s string) string {
	if d, err := url.PathUnescape(s); err == nil {
		return d
	}
	return s
}

// decodeB64 依次尝试 4 种 base64 变体解码, 解码结果含控制字符则视为失败
func decodeB64(s string) (string, bool) {
	s = removeWhitespace(s)
	if s == "" {
		return "", false
	}
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			if hasControlChars(b) {
				continue
			}
			return string(b), true
		}
	}
	return "", false
}

func removeWhitespace(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hasControlChars(b []byte) bool {
	for _, c := range b {
		if c < 0x20 && c != '\n' && c != '\r' && c != '\t' {
			return true
		}
	}
	return false
}
