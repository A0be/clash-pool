package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/A0be/clash-pool/internal/api"
	"github.com/A0be/clash-pool/internal/checker"
	"github.com/A0be/clash-pool/internal/config"
	"github.com/A0be/clash-pool/internal/fetch"
	"github.com/A0be/clash-pool/internal/mihomo"
	"github.com/A0be/clash-pool/internal/parse"
	"github.com/A0be/clash-pool/internal/pool"
	"github.com/A0be/clash-pool/internal/store"
)

// historyRetention 延迟历史保留时长
const historyRetention = 7 * 24 * time.Hour

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

// cycleResult 一轮循环的结果
type cycleResult struct {
	newPool *pool.Pool // 基于本轮存活节点构建的池
	stats   store.StateSnapshot
	rows    []store.HistoryRow // 本轮延迟记录(写 SQLite)
}

// runCycle 执行一轮完整流程: 拉取订阅 → 解析清洗去重 → mihomo 测速验活 → 构池
func runCycle(cfg config.Config, binary string) (*cycleResult, error) {
	// 1. 逐源拉取 → 解析 → 过滤假节点 → 加来源前缀
	var all []map[string]any
	var subStats []store.SubStat
	fakeTotal := 0
	for i, sub := range cfg.Subscriptions {
		prefix := sub.Name
		if prefix == "" {
			prefix = derivePrefix(sub.URL)
		}

		content, err := fetch.Fetch(sub.URL)
		if err != nil {
			log.Printf("[%d/%d] %s 拉取失败: %v", i+1, len(cfg.Subscriptions), sub.URL, err)
			subStats = append(subStats, store.SubStat{Name: prefix})
			continue
		}
		proxies, err := parse.Parse(content)
		if err != nil {
			log.Printf("[%d/%d] %s 解析失败: %v", i+1, len(cfg.Subscriptions), sub.URL, err)
			subStats = append(subStats, store.SubStat{Name: prefix})
			continue
		}
		proxies, fake := parse.FilterFake(proxies)
		parse.EnsureNames(proxies)
		proxies = parse.ApplyPrefix(proxies, prefix)
		all = append(all, proxies...)
		subStats = append(subStats, store.SubStat{Name: prefix, Parsed: len(proxies), Fake: fake})
		fakeTotal += fake
		log.Printf("[%d/%d] %s: 解析 %d 个节点(过滤假节点 %d 个)",
			i+1, len(cfg.Subscriptions), prefix, len(proxies), fake)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("所有订阅源均未解析出节点")
	}

	// 2. 跨订阅源严格去重 + 同名节点追加序号
	total := len(all)
	all, dup := parse.Dedup(all)
	parse.UniquifyNames(all)

	byType := map[string]int{}
	for _, p := range all {
		byType[parse.Str(p, "type")]++
	}
	var parts []string
	for tp, n := range byType {
		parts = append(parts, fmt.Sprintf("%s=%d", tp, n))
	}
	sort.Strings(parts)
	log.Printf("汇总: 原始 %d 个节点, 过滤假节点 %d 个, 去重 %d 个, 最终保留 %d 个 (协议: %s)",
		total+fakeTotal+dup, fakeTotal, dup, len(all), strings.Join(parts, ", "))

	// 3. mihomo 并发测速验活
	alive, dead, err := checker.Run(all, checker.Options{
		Binary:      binary,
		WorkDir:     "data",
		TestURL:     cfg.Check.TestURL,
		TimeoutMS:   cfg.Check.Timeout,
		MaxDelayMS:  cfg.Check.MaxDelay,
		Concurrency: cfg.Check.Concurrency,
	})
	if err != nil {
		return nil, fmt.Errorf("测速验活失败: %w", err)
	}
	log.Printf("测速完成: 存活 %d / 失效 %d", len(alive), len(dead))
	if len(alive) == 0 {
		return nil, fmt.Errorf("无存活节点, 代理池为空")
	}

	// 4. 构池 + 生成持久化数据
	p := pool.Build(alive)
	nodes := p.Nodes()
	snap := store.StateSnapshot{
		UpdatedAt:     time.Now(),
		Alive:         len(alive),
		Dead:          len(dead),
		Subscriptions: subStats,
	}
	var rows []store.HistoryRow
	for _, n := range nodes {
		snap.Nodes = append(snap.Nodes, store.NodeState{
			Name: n.Name, Type: n.Type, Region: n.Region, DelayMS: n.DelayMS,
		})
		rows = append(rows, store.HistoryRow{
			Name: n.Name, Region: n.Region, DelayMS: n.DelayMS,
		})
	}
	return &cycleResult{newPool: p, stats: snap, rows: rows}, nil
}

// persist 写出状态 YAML + 记录延迟历史 + 输出代理池文件
func persist(cfg config.Config, res *cycleResult, hist *store.History) {
	if err := store.SaveState("data/state.yaml", res.stats); err != nil {
		log.Printf("状态持久化失败: %v", err)
	}
	if hist != nil {
		if err := hist.Record(res.rows); err != nil {
			log.Printf("延迟历史写入失败: %v", err)
		}
	}
	stats, err := pool.WriteOutputs(pool.OutputPaths{
		ClashFile: cfg.Output.SubFile,
		LinksFile: cfg.Output.LinksFile,
		B64File:   cfg.Output.B64File,
	}, res.newPool.Nodes(), cfg.Check.TestURL)
	if err != nil {
		log.Printf("写出代理池文件失败: %v", err)
		return
	}
	log.Printf("输出完成: Clash 订阅(%d 节点) → %s, 分享链接(%d 条) → %s, Base64 → %s",
		stats.ClashCount, cfg.Output.SubFile, stats.LinkCount, cfg.Output.LinksFile, cfg.Output.B64File)
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

	// mihomo 内核
	binary, err := mihomo.EnsureKernel("bin")
	if err != nil {
		log.Fatalf("获取 mihomo 内核失败: %v", err)
	}
	log.Printf("使用 mihomo 内核: %s", binary)

	// 延迟历史库
	hist, err := store.OpenHistory("data/history.db", historyRetention)
	if err != nil {
		log.Fatalf("打开延迟历史库失败: %v", err)
	}
	defer hist.Close()

	// 第一轮
	res, err := runCycle(cfg, binary)
	if err != nil {
		log.Fatalf("首轮运行失败: %v", err)
	}
	p := res.newPool
	persist(cfg, res, hist)

	// SOCKS5/HTTP 混合入口
	var serveInst *mihomo.Instance
	if cfg.Serve.MixedPort > 0 {
		serveInst, err = mihomo.StartServe(binary, "data", p.ProxyMaps(), mihomo.ServeOptions{
			MixedPort: cfg.Serve.MixedPort,
			TestURL:   cfg.Check.TestURL,
		})
		if err != nil {
			log.Fatalf("启动 SOCKS5 入口失败: %v", err)
		}
		// 闭包引用: 定时刷新会重启实例并更新 serveInst, 退出时停的是当前实例
		defer func() { serveInst.Stop() }()
		log.Printf("SOCKS5/HTTP 入口已就绪: 127.0.0.1:%d (自动走最低延迟节点)", cfg.Serve.MixedPort)
	}

	// HTTP API + 状态面板
	go api.Listen(p, cfg.Output.APIAddr, cfg.Output.APIToken)
	log.Printf("HTTP API 已就绪: http://%s (GET /get /all /count /stats, 参数 region/type%s)",
		cfg.Output.APIAddr, tokenHint(cfg.Output.APIToken))

	// 定时循环: 全量刷新订阅 + 复测 + 热更新池与 SOCKS5 入口
	if cfg.Check.Interval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(cfg.Check.Interval) * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				log.Printf("—— 定时刷新开始 ——")
				res, err := runCycle(cfg, binary)
				if err != nil {
					log.Printf("本轮刷新失败(保留旧池): %v", err)
					continue
				}
				// 热更新: API 立即读到新节点; SOCKS5 入口重启加载新配置
				p.Replace(res.newPool.Nodes())
				persist(cfg, res, hist)
				if cfg.Serve.MixedPort > 0 {
					serveInst.Stop()
					serveInst, err = mihomo.StartServe(binary, "data", res.newPool.ProxyMaps(), mihomo.ServeOptions{
						MixedPort: cfg.Serve.MixedPort,
						TestURL:   cfg.Check.TestURL,
					})
					if err != nil {
						log.Printf("SOCKS5 入口重启失败: %v", err)
					} else {
						log.Printf("SOCKS5 入口已重载: 127.0.0.1:%d", cfg.Serve.MixedPort)
					}
				}
				log.Printf("—— 定时刷新完成, 下一轮 %d 分钟后 ——", cfg.Check.Interval)
			}
		}()
		log.Printf("定时刷新已启用: 每 %d 分钟", cfg.Check.Interval)
	}

	log.Printf("代理池运行中, 按 Ctrl+C 退出")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	log.Printf("收到退出信号, 正在停止...")
}

// tokenHint API 认证提示文案
func tokenHint(token string) string {
	if token == "" {
		return ", 未启用认证"
	}
	return ", 需 token 认证"
}
