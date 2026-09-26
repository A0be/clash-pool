// Package parse 解析订阅内容为统一的代理节点列表
package parse

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse 自动识别订阅内容格式并解析出代理列表。
// 支持三种格式: Clash YAML / Base64 编码的分享链接列表 / 明文分享链接列表
func Parse(content []byte) ([]map[string]any, error) {
	text := strings.TrimSpace(string(content))
	if text == "" {
		return nil, errors.New("订阅内容为空")
	}

	// 含 proxies: 键 → Clash YAML 配置
	if strings.Contains(text, "proxies:") {
		return parseClashYAML(content)
	}

	// 尝试整体 Base64 解码出链接列表
	if decoded, ok := decodeB64(text); ok && strings.Contains(decoded, "://") {
		text = decoded
	}
	return parseLinkList(text)
}

// parseClashYAML 解析 Clash YAML 中的 proxies 列表
func parseClashYAML(content []byte) ([]map[string]any, error) {
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, errors.New("Clash YAML 解析失败: " + err.Error())
	}

	var proxies []map[string]any
	for _, p := range doc.Proxies {
		// 缺少必要字段的条目直接跳过
		if Str(p, "type") == "" || Str(p, "server") == "" || toInt(p["port"]) == 0 {
			continue
		}
		proxies = append(proxies, p)
	}
	if len(proxies) == 0 {
		return nil, errors.New("YAML 中未找到有效代理(缺少 proxies 字段或条目字段不全)")
	}
	return proxies, nil
}

// parseLinkList 解析分享链接列表(按空白分隔, 每行一条)
func parseLinkList(text string) ([]map[string]any, error) {
	var proxies []map[string]any
	var bad []string
	for _, tok := range strings.Fields(text) {
		if !strings.Contains(tok, "://") {
			continue
		}
		m, err := parseLink(tok)
		if err != nil {
			bad = append(bad, err.Error())
			continue
		}
		proxies = append(proxies, m)
	}
	if len(proxies) == 0 {
		if len(bad) > 0 {
			return nil, errors.New("链接解析全部失败: " + strings.Join(bad, "; "))
		}
		return nil, errors.New("未在订阅中找到代理链接")
	}
	return proxies, nil
}
