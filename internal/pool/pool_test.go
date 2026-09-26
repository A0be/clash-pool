package pool

import (
	"os"
	"testing"

	"github.com/A0be/clash-pool/internal/checker"
	"github.com/A0be/clash-pool/internal/parse"
)

func TestDetectRegion(t *testing.T) {
	cases := map[string]string{
		"[机场A] 香港 01":       "HK",
		"HKG-Premium":        "HK",
		"🇸🇬 Singapore-01":   "SG",
		"JP 东京 BGP":          "JP",
		"US-Los Angeles":     "US",
		"Russia-Moscow":      "EU", // 俄罗斯归入 EU
		"TW-台北":              "TW",
		"Mars-01":            "other",
	}
	for name, want := range cases {
		if got := DetectRegion(name); got != want {
			t.Errorf("DetectRegion(%q) = %q, 期望 %q", name, got, want)
		}
	}
}

// TestLinkRoundtrip 编码 → 解析 回环测试: 编码出的分享链接必须能被自己的解析器还原
func TestLinkRoundtrip(t *testing.T) {
	cases := []map[string]any{
		{"name": "测试SS", "type": "ss", "server": "1.2.3.4", "port": 8388,
			"cipher": "aes-256-gcm", "password": "p@ss:word"},
		{"name": "测试VMess", "type": "vmess", "server": "jp.example.com", "port": 443,
			"uuid": "u-1", "alterId": 0, "cipher": "auto", "tls": true,
			"servername": "jp.example.com", "network": "ws",
			"ws-opts": map[string]any{"path": "/ws", "headers": map[string]any{"Host": "cdn.example.com"}}},
		{"name": "测试Trojan", "type": "trojan", "server": "hk.example.com", "port": 443,
			"password": "pw", "sni": "hk.example.com", "skip-cert-verify": true},
		{"name": "测试VLESS", "type": "vless", "server": "1.2.3.4", "port": 443,
			"uuid": "u-2", "tls": true, "flow": "xtls-rprx-vision",
			"reality-opts": map[string]any{"public-key": "PUB", "short-id": "ab"}},
		{"name": "测试HY2", "type": "hysteria2", "server": "us.example.com", "port": 443,
			"password": "auth", "obfs": "salamander", "obfs-password": "ob"},
	}
	for _, p := range cases {
		link := ShareLink(p)
		if link == "" {
			t.Fatalf("ShareLink 返回空: %v", p)
		}
		got, err := parse.Parse([]byte(link))
		if err != nil {
			t.Fatalf("回环解析失败 %s: %v", link, err)
		}
		if len(got) != 1 {
			t.Fatalf("回环解析数量错误: %d (%s)", len(got), link)
		}
		g := got[0]
		if parse.Str(g, "name") != parse.Str(p, "name") ||
			parse.Str(g, "type") != parse.Str(p, "type") ||
			parse.Str(g, "server") != parse.Str(p, "server") ||
			parse.ToInt(g["port"]) != parse.ToInt(p["port"]) {
			t.Errorf("回环基础字段不一致:\n原: %v\n得: %v", p, g)
		}
		// 协议关键字段
		switch parse.Str(p, "type") {
		case "ss":
			if parse.Str(g, "cipher") != "aes-256-gcm" || parse.Str(g, "password") != "p@ss:word" {
				t.Errorf("ss 回环字段错误: %v", g)
			}
		case "vmess":
			if parse.Str(g, "uuid") != "u-1" || g["tls"] != true || parse.Str(g, "network") != "ws" {
				t.Errorf("vmess 回环字段错误: %v", g)
			}
		case "trojan":
			if parse.Str(g, "password") != "pw" || parse.Str(g, "sni") != "hk.example.com" ||
				g["skip-cert-verify"] != true {
				t.Errorf("trojan 回环字段错误: %v", g)
			}
		case "vless":
			if parse.Str(g, "uuid") != "u-2" || g["tls"] != true ||
				parse.Str(g, "flow") != "xtls-rprx-vision" {
				t.Errorf("vless 回环字段错误: %v", g)
			}
			ro := g["reality-opts"].(map[string]any)
			if parse.Str(ro, "public-key") != "PUB" || parse.Str(ro, "short-id") != "ab" {
				t.Errorf("vless reality 回环错误: %v", ro)
			}
		case "hysteria2":
			if parse.Str(g, "password") != "auth" || parse.Str(g, "obfs") != "salamander" ||
				parse.Str(g, "obfs-password") != "ob" {
				t.Errorf("hysteria2 回环字段错误: %v", g)
			}
		}
	}
}

func TestShareLinkUnsupported(t *testing.T) {
	if l := ShareLink(map[string]any{"type": "http", "name": "x"}); l != "" {
		t.Errorf("http 类型不应有分享链接: %s", l)
	}
}

func TestPoolFilterGetCount(t *testing.T) {
	p := Build([]checker.Result{
		{Proxy: map[string]any{"name": "香港01", "type": "ss", "server": "a", "port": 1, "password": "x"}, DelayMS: 100},
		{Proxy: map[string]any{"name": "US-02", "type": "trojan", "server": "b", "port": 2, "password": "y"}, DelayMS: 200},
		{Proxy: map[string]any{"name": "东京03", "type": "ss", "server": "c", "port": 3, "password": "z"}, DelayMS: 300},
	})

	if got := p.Count("", ""); got != 3 {
		t.Errorf("Count 全部 = %d, 期望 3", got)
	}
	if got := p.Count("hk", ""); got != 1 {
		t.Errorf("Count hk = %d, 期望 1", got)
	}
	if got := p.Count("", "ss"); got != 2 {
		t.Errorf("Count ss = %d, 期望 2", got)
	}
	n := p.Get("hk", "trojan") // 类型不匹配
	if n != nil {
		t.Errorf("Get(hk,trojan) 应为 nil, 得 %v", n)
	}
	n = p.Get("us", "")
	if n == nil || n.Name != "US-02" {
		t.Errorf("Get(us) 错误: %v", n)
	}
}

func TestWriteOutputs(t *testing.T) {
	p := Build([]checker.Result{
		{Proxy: map[string]any{"name": "香港01", "type": "ss", "server": "1.1.1.1", "port": 8388,
			"cipher": "aes-256-gcm", "password": "x"}, DelayMS: 100},
	})
	dir := t.TempDir()
	stats, err := WriteOutputs(OutputPaths{
		ClashFile: dir + "/pool.yaml",
		LinksFile: dir + "/links.txt",
		B64File:   dir + "/b64.txt",
	}, p.Nodes(), "https://www.gstatic.com/generate_204")
	if err != nil {
		t.Fatalf("WriteOutputs 报错: %v", err)
	}
	if stats.ClashCount != 1 || stats.LinkCount != 1 {
		t.Errorf("统计错误: %+v", stats)
	}

	// 验证生成的 Clash 订阅能被解析器读回
	content, err := os.ReadFile(dir + "/pool.yaml")
	if err != nil {
		t.Fatal(err)
	}
	proxies, err := parse.Parse(content)
	if err != nil {
		t.Fatalf("生成的 pool.yaml 解析失败: %v", err)
	}
	if len(proxies) != 1 || parse.Str(proxies[0], "name") != "香港01" {
		t.Errorf("pool.yaml 内容错误: %v", proxies)
	}
}
