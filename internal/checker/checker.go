// Package checker 基于 mihomo 内核的并发测速验活
package checker

import (
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/A0be/clash-pool/internal/mihomo"
	"github.com/A0be/clash-pool/internal/parse"
)

// Options 测速配置
type Options struct {
	Binary      string // mihomo 内核路径
	WorkDir     string // 实例工作目录(存放临时配置与日志)
	TestURL     string // 延迟测试 URL
	TimeoutMS   int    // 单节点超时(毫秒)
	MaxDelayMS  int    // 存活节点最大延迟(毫秒)
	Concurrency int    // 并发测速数
}

// Result 单个节点的测速结果
type Result struct {
	Proxy   map[string]any
	DelayMS int // 存活时为往返延迟, 失效时为 0
}

// Run 启动 mihomo 实例并发测速, 返回存活(延迟达标)与失效节点, 存活按延迟升序
func Run(proxies []map[string]any, opt Options) (alive, dead []Result, err error) {
	if len(proxies) == 0 {
		return nil, nil, nil
	}
	if opt.Concurrency <= 0 {
		opt.Concurrency = 50
	}

	inst, err := mihomo.Start(opt.Binary, opt.WorkDir, proxies)
	if err != nil {
		return nil, nil, err
	}
	defer inst.Stop()

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, opt.Concurrency)

	for _, p := range proxies {
		wg.Add(1)
		go func(p map[string]any) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name := parse.Str(p, "name")
			delay, err := inst.Delay(name, opt.TestURL, opt.TimeoutMS)

			mu.Lock()
			defer mu.Unlock()
			if err == nil && delay <= opt.MaxDelayMS {
				alive = append(alive, Result{Proxy: p, DelayMS: delay})
			} else {
				dead = append(dead, Result{Proxy: p})
			}
		}(p)
	}
	wg.Wait()

	sort.Slice(alive, func(i, j int) bool { return alive[i].DelayMS < alive[j].DelayMS })
	log.Printf("mihomo 测速完成: 存活 %d, 失效 %d", len(alive), len(dead))
	return alive, dead, nil
}

// Summary 返回结果摘要字符串
func Summary(alive, dead []Result) string {
	return fmt.Sprintf("存活 %d / 失效 %d", len(alive), len(dead))
}
