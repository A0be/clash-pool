package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// Config clash-pool 主配置
type Config struct {
	// Subscriptions clash 订阅源列表（机场订阅 / 公开聚合订阅）
	Subscriptions []string `yaml:"subscriptions"`
	// Check 测速验活配置
	Check struct {
		// TestURL 延迟测试 URL
		TestURL string `yaml:"test_url"`
		// Timeout 单节点超时（毫秒）
		Timeout int `yaml:"timeout_ms"`
		// MaxDelay 存活节点最大延迟（毫秒），超过则剔除
		MaxDelay int `yaml:"max_delay_ms"`
		// Interval 定时复测间隔（分钟），0 表示只跑一次
		Interval int `yaml:"interval_min"`
	} `yaml:"check"`
	// Output 输出配置
	Output struct {
		// APIAddr 代理池 HTTP API 监听地址（/get /all /count）
		APIAddr string `yaml:"api_addr"`
		// SubFile 生成新订阅文件的路径
		SubFile string `yaml:"sub_file"`
	} `yaml:"output"`
}

// defaultConfig 返回内置默认配置
func defaultConfig() Config {
	var c Config
	c.Subscriptions = []string{}
	c.Check.TestURL = "https://www.gstatic.com/generate_204"
	c.Check.Timeout = 5000
	c.Check.MaxDelay = 3000
	c.Check.Interval = 30
	c.Output.APIAddr = "127.0.0.1:8080"
	c.Output.SubFile = "pool.yaml"
	return c
}

// loadConfig 从 YAML 文件加载配置，文件不存在时返回默认配置
func loadConfig(path string) (Config, error) {
	c := defaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("解析配置 %s 失败: %w", path, err)
	}
	return c, nil
}

func main() {
	log.SetFlags(log.LstdFlags)

	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	log.Printf("clash-pool 启动")
	log.Printf("  订阅源数量: %d", len(cfg.Subscriptions))
	log.Printf("  测速 URL: %s (超时 %dms, 剔除延迟 >%dms)",
		cfg.Check.TestURL, cfg.Check.Timeout, cfg.Check.MaxDelay)
	log.Printf("  API 监听: %s", cfg.Output.APIAddr)
	log.Printf("  订阅输出: %s", cfg.Output.SubFile)

	// TODO(阶段1): fetcher — 拉取订阅
	// TODO(阶段1): parser — 解析 Clash YAML / Base64 分享链接并去重
	// TODO(阶段2): checker — 管理 mihomo 内核子进程, 通过 external-controller API 并发测速
	// TODO(阶段3): pool — 存活节点池, 输出新订阅 + HTTP API
	// TODO(阶段4): 定时循环刷新
	log.Printf("功能开发中: 订阅拉取/解析、mihomo 测速验活、代理池输出将按路线图逐步实现")
}
