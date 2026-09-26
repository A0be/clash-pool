package parse

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestParseClashYAML(t *testing.T) {
	content := []byte(`proxies:
  - name: 香港01
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: pass
  - name: 美国01
    type: trojan
    server: 5.6.7.8
    port: 443
    password: pw
`)
	got, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse 报错: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("期望 2 个节点, 实际 %d", len(got))
	}
	if got[0]["name"] != "香港01" || got[0]["type"] != "ss" {
		t.Errorf("第一个节点字段错误: %v", got[0])
	}
}

func TestParseBase64LinkList(t *testing.T) {
	links := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pass")) +
		"@1.2.3.4:8388#测试\n" +
		"trojan://pw@hk.example.com:443?sni=hk.example.com#香港\n"
	content := []byte(base64.StdEncoding.EncodeToString([]byte(links)))

	got, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse 报错: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("期望 2 个节点, 实际 %d", len(got))
	}
	if got[0]["type"] != "ss" || got[1]["type"] != "trojan" {
		t.Errorf("解析类型错误: %v %v", got[0], got[1])
	}
}

func TestParsePlainLinkList(t *testing.T) {
	content := []byte("hysteria2://auth@us.example.com:443?sni=us.example.com#美国\n")
	got, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse 报错: %v", err)
	}
	if len(got) != 1 || got[0]["type"] != "hysteria2" {
		t.Fatalf("解析结果错误: %v", got)
	}
}

func TestParseSS(t *testing.T) {
	// SIP002 base64 userinfo
	link := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:p@ss")) +
		"@1.2.3.4:8388#节点A"
	m, err := parseSS(link)
	if err != nil {
		t.Fatalf("parseSS 报错: %v", err)
	}
	if m["cipher"] != "aes-128-gcm" || m["password"] != "p@ss" ||
		m["server"] != "1.2.3.4" || m["port"] != 8388 || m["name"] != "节点A" {
		t.Errorf("SIP002 字段错误: %v", m)
	}

	// SIP002 明文 userinfo
	m, err = parseSS("ss://aes-256-gcm:pass@1.2.3.4:8388#节点B")
	if err != nil {
		t.Fatalf("parseSS 明文报错: %v", err)
	}
	if m["cipher"] != "aes-256-gcm" || m["password"] != "pass" {
		t.Errorf("明文字段错误: %v", m)
	}

	// 旧式整体 base64
	m, err = parseSS("ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pass@9.9.9.9:8388")) + "#旧格式")
	if err != nil {
		t.Fatalf("parseSS 旧式报错: %v", err)
	}
	if m["server"] != "9.9.9.9" || m["port"] != 8388 || m["name"] != "旧格式" {
		t.Errorf("旧式字段错误: %v", m)
	}
}

func TestParseVMess(t *testing.T) {
	jsonConf := `{"ps":"东京","add":"jp.example.com","port":"443","id":"uuid-1234",` +
		`"aid":"0","scy":"auto","net":"ws","host":"cdn.example.com","path":"/ws",` +
		`"tls":"tls","sni":"jp.example.com"}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(jsonConf))
	m, err := parseVMess(link)
	if err != nil {
		t.Fatalf("parseVMess 报错: %v", err)
	}
	if m["server"] != "jp.example.com" || m["port"] != 443 || m["uuid"] != "uuid-1234" {
		t.Errorf("基础字段错误: %v", m)
	}
	if m["tls"] != true || m["servername"] != "jp.example.com" {
		t.Errorf("TLS 字段错误: %v", m)
	}
	ws, ok := m["ws-opts"].(map[string]any)
	if !ok || ws["path"] != "/ws" {
		t.Fatalf("ws-opts 错误: %v", m["ws-opts"])
	}
	headers := ws["headers"].(map[string]any)
	if headers["Host"] != "cdn.example.com" {
		t.Errorf("ws Host 错误: %v", headers)
	}
}

func TestParseTrojan(t *testing.T) {
	link := "trojan://pw@hk.example.com:443?sni=hk.example.com&allowInsecure=1&type=ws&path=%2Fws&host=cdn.example.com#香港"
	m, err := parseTrojan(link)
	if err != nil {
		t.Fatalf("parseTrojan 报错: %v", err)
	}
	if m["password"] != "pw" || m["sni"] != "hk.example.com" || m["skip-cert-verify"] != true {
		t.Errorf("基础字段错误: %v", m)
	}
	ws := m["ws-opts"].(map[string]any)
	if ws["path"] != "/ws" {
		t.Errorf("ws path 错误: %v", ws)
	}
	if ws["headers"].(map[string]any)["Host"] != "cdn.example.com" {
		t.Errorf("ws Host 错误: %v", ws)
	}
}

func TestParseVlessReality(t *testing.T) {
	link := "vless://uuid-1@1.2.3.4:443?security=reality&pbk=PUBKEY&sid=abcd&fp=chrome&flow=xtls-rprx-vision&type=tcp#现实"
	m, err := parseVless(link)
	if err != nil {
		t.Fatalf("parseVless 报错: %v", err)
	}
	if m["uuid"] != "uuid-1" || m["tls"] != true || m["flow"] != "xtls-rprx-vision" {
		t.Errorf("基础字段错误: %v", m)
	}
	ro := m["reality-opts"].(map[string]any)
	if ro["public-key"] != "PUBKEY" || ro["short-id"] != "abcd" {
		t.Errorf("reality-opts 错误: %v", ro)
	}
	if m["client-fingerprint"] != "chrome" {
		t.Errorf("指纹错误: %v", m["client-fingerprint"])
	}
}

func TestParseHysteria2(t *testing.T) {
	link := "hysteria2://auth@us.example.com:443?sni=us.example.com&insecure=1&obfs=salamander&obfs-password=ob#美国"
	m, err := parseHysteria2(link)
	if err != nil {
		t.Fatalf("parseHysteria2 报错: %v", err)
	}
	if m["password"] != "auth" || m["sni"] != "us.example.com" ||
		m["skip-cert-verify"] != true || m["obfs"] != "salamander" || m["obfs-password"] != "ob" {
		t.Errorf("字段错误: %v", m)
	}
}

func TestFilterFake(t *testing.T) {
	proxies := []map[string]any{
		{"name": "剩余流量:10GB", "type": "ss", "server": "a", "port": 1, "password": "x"},
		{"name": "官网 example.com", "type": "ss", "server": "b", "port": 2, "password": "y"},
		{"name": "香港 01", "type": "ss", "server": "c", "port": 3, "password": "z"},
	}
	real, fake := FilterFake(proxies)
	if fake != 2 || len(real) != 1 || real[0]["name"] != "香港 01" {
		t.Errorf("FilterFake 结果错误: real=%v fake=%d", real, fake)
	}
}

func TestDedup(t *testing.T) {
	proxies := []map[string]any{
		{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 8388, "password": "same"},
		{"name": "B", "type": "ss", "server": "1.1.1.1", "port": 8388, "password": "same"},
		{"name": "C", "type": "ss", "server": "1.1.1.1", "port": 8388, "password": "diff"},
	}
	unique, dup := Dedup(proxies)
	if dup != 1 || len(unique) != 2 {
		t.Errorf("Dedup 结果错误: unique=%d dup=%d", len(unique), dup)
	}
}

func TestApplyPrefixAndUniquify(t *testing.T) {
	proxies := []map[string]any{
		{"name": "香港", "type": "ss", "server": "a", "port": 1},
		{"name": "香港", "type": "ss", "server": "b", "port": 2},
	}
	proxies = ApplyPrefix(proxies, "机场A")
	if proxies[0]["name"] != "[机场A] 香港" {
		t.Errorf("前缀错误: %v", proxies[0]["name"])
	}
	UniquifyNames(proxies)
	names := []string{fmt.Sprint(proxies[0]["name"]), fmt.Sprint(proxies[1]["name"])}
	if names[0] != "[机场A] 香港" || names[1] != "[机场A] 香港-2" {
		t.Errorf("同名序号错误: %v", names)
	}
	if !strings.Contains(names[0], "机场A") {
		t.Errorf("前缀丢失")
	}
}
