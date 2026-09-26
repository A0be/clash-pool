package pool

import (
	"encoding/base64"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// OutputPaths 输出文件路径
type OutputPaths struct {
	ClashFile string
	LinksFile string
	B64File   string
}

// OutputStats 输出统计
type OutputStats struct {
	ClashCount int
	LinkCount  int
}

// WriteOutputs 写出三种格式的代理池文件:
// 1. Clash YAML 订阅(含 POOL 自动选择组, 可直接导入客户端)
// 2. 分享链接文本(每行一条)
// 3. Base64 订阅(分享链接整体的 base64)
func WriteOutputs(paths OutputPaths, nodes []Node, testURL string) (OutputStats, error) {
	var stats OutputStats

	// 1. Clash YAML 订阅
	names := make([]string, 0, len(nodes))
	proxies := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.Name)
		proxies = append(proxies, n.Proxy)
	}
	cfg := map[string]any{
		"proxies": proxies,
		"proxy-groups": []map[string]any{{
			"name": "POOL", "type": "url-test", "url": testURL,
			"interval": 300, "tolerance": 50, "proxies": names,
		}},
		"rules": []string{"MATCH,POOL"},
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return stats, err
	}
	if err := os.WriteFile(paths.ClashFile, data, 0o644); err != nil {
		return stats, err
	}
	stats.ClashCount = len(nodes)

	// 2. 分享链接文本(跳过无法编码的类型)
	links := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Link != "" {
			links = append(links, n.Link)
		}
	}
	linksText := strings.Join(links, "\n") + "\n"
	if err := os.WriteFile(paths.LinksFile, []byte(linksText), 0o644); err != nil {
		return stats, err
	}
	stats.LinkCount = len(links)

	// 3. Base64 订阅
	b64 := base64.StdEncoding.EncodeToString([]byte(linksText))
	if err := os.WriteFile(paths.B64File, []byte(b64), 0o644); err != nil {
		return stats, err
	}
	return stats, nil
}
