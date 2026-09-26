package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/A0be/clash-pool/internal/checker"
	"github.com/A0be/clash-pool/internal/config"
	"github.com/A0be/clash-pool/internal/fetch"
	"github.com/A0be/clash-pool/internal/mihomo"
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
		parse.EnsureNames(proxies)
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

	// 阶段 2: mihomo 内核 + 并发测速验活
	binary, err := mihomo.EnsureKernel("bin")
	if err != nil {
		log.Fatalf("获取 mihomo 内核失败: %v", err)
	}
	log.Printf("使用 mihomo 内核: %s", binary)

	alive, dead, err := checker.Run(all, checker.Options{
		Binary:      binary,
		WorkDir:     "data",
		TestURL:     cfg.Check.TestURL,
		TimeoutMS:   cfg.Check.Timeout,
		MaxDelayMS:  cfg.Check.MaxDelay,
		Concurrency: cfg.Check.Concurrency,
	})
	if err != nil {
		log.Fatalf("测速验活失败: %v", err)
	}
	log.Printf("测速完成: 存活 %d / 失效 %d (测试 URL: %s, 并发 %d, 剔除延迟 >%dms)",
		len(alive), len(dead), cfg.Check.TestURL, cfg.Check.Concurrency, cfg.Check.MaxDelay)

	for i, r := range alive {
		if i >= 10 {
			log.Printf("  ... 其余 %d 个存活节点省略", len(alive)-10)
			break
		}
		log.Printf("  #%d %s → %dms", i+1, parse.Str(r.Proxy, "name"), r.DelayMS)
	}
	if len(alive) == 0 {
		log.Fatalf("无存活节点, 代理池为空")
	}

	// TODO(阶段3): pool — 输出 Clash YAML / 分享链接 / Base64 订阅 + HTTP API(支持地区/协议筛选)
	// TODO(阶段4): 定时循环 + 状态持久化(节点状态 YAML + 延迟历史 SQLite) + Web 面板
	// TODO(阶段5, 可选): TG 频道/网页节点抓取
	log.Printf("阶段 2 完成: mihomo 测速验活; 下一步开发代理池输出")
}
