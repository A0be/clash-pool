package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/A0be/clash-pool/internal/config"
	"github.com/A0be/clash-pool/internal/fetch"
	"github.com/A0be/clash-pool/internal/parse"
)

// derivePrefix 订阅源未命名时, 自动用域名或文件名作为节点前缀
func derivePrefix(source string) string {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		if u, err := url.Parse(source); err == nil && u.Host != "" {
			return u.Hostname()
		}
	}
	base := filepath.Base(source)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func main() {
	log.SetFlags(log.LstdFlags)

	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	if len(cfg.Subscriptions) == 0 {
		log.Fatalf("未配置订阅源: 请复制 config.example.yaml 为 config.yaml 并填入 subscriptions")
	}

	log.Printf("clash-pool 启动: %d 个订阅源", len(cfg.Subscriptions))

	// 阶段 1: 逐源拉取 → 解析 → 过滤假节点 → 加来源前缀
	var all []map[string]any
	fakeTotal := 0
	for i, sub := range cfg.Subscriptions {
		prefix := sub.Name
		if prefix == "" {
			prefix = derivePrefix(sub.URL)
		}

		content, err := fetch.Fetch(sub.URL)
		if err != nil {
			log.Printf("[%d/%d] %s 拉取失败: %v", i+1, len(cfg.Subscriptions), sub.URL, err)
			continue
		}
		proxies, err := parse.Parse(content)
		if err != nil {
			log.Printf("[%d/%d] %s 解析失败: %v", i+1, len(cfg.Subscriptions), sub.URL, err)
			continue
		}
		proxies, fake := parse.FilterFake(proxies)
		fakeTotal += fake
		proxies = parse.ApplyPrefix(proxies, prefix)
		all = append(all, proxies...)
		log.Printf("[%d/%d] %s: 解析 %d 个节点(过滤假节点 %d 个)",
			i+1, len(cfg.Subscriptions), prefix, len(proxies), fake)
	}
	if len(all) == 0 {
		log.Fatalf("所有订阅源均未解析出节点")
	}

	// 跨订阅源严格去重 + 同名节点追加序号
	total := len(all)
	all, dup := parse.Dedup(all)
	parse.UniquifyNames(all)

	// 协议分布统计
	byType := map[string]int{}
	for _, p := range all {
		byType[parse.Str(p, "type")]++
	}
	var parts []string
	for tp, n := range byType {
		parts = append(parts, fmt.Sprintf("%s=%d", tp, n))
	}
	sort.Strings(parts)

	log.Printf("汇总: 原始 %d 个节点, 过滤假节点 %d 个, 去重 %d 个, 最终保留 %d 个",
		total+fakeTotal+dup, fakeTotal, dup, len(all))
	log.Printf("协议分布: %s", strings.Join(parts, ", "))
	for i, p := range all {
		if i >= 5 {
			break
		}
		log.Printf("  示例: %s (%s, %s:%v)",
			parse.Str(p, "name"), parse.Str(p, "type"), parse.Str(p, "server"), p["port"])
	}

	// TODO(阶段2): checker — 管理 mihomo 内核, 通过 external-controller API 并发测速验活(支持多测试 URL)
	// TODO(阶段3): pool — 输出 Clash YAML / 分享链接 / Base64 订阅 + HTTP API(支持地区/协议筛选)
	// TODO(阶段4): 定时循环 + 状态持久化(节点状态 YAML + 延迟历史 SQLite) + Web 面板
	// TODO(阶段5, 可选): TG 频道/网页节点抓取
	log.Printf("阶段 1 完成: 订阅拉取与解析; 下一步开发 mihomo 测速验活")
}
