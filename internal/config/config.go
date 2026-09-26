// Package config 加载与解析 clash-pool 主配置
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Subscription 订阅源: name 用作节点名前缀, url 支持 http(s) 与本地文件路径
type Subscription struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// Config clash-pool 主配置
type Config struct {
	// Subscriptions clash 订阅源列表(机场订阅 / 公开聚合订阅)
	Subscriptions []Subscription `yaml:"subscriptions"`
	// Check 测速验活配置
	Check struct {
		// TestURL 延迟测试 URL
		TestURL string `yaml:"test_url"`
		// Timeout 单节点超时(毫秒)
		Timeout int `yaml:"timeout_ms"`
		// MaxDelay 存活节点最大延迟(毫秒), 超过则剔除
		MaxDelay int `yaml:"max_delay_ms"`
		// Concurrency 并发测速数
		Concurrency int `yaml:"concurrency"`
		// Interval 定时复测间隔(分钟), 0 表示只跑一次
		Interval int `yaml:"interval_min"`
	} `yaml:"check"`
	// Serve 常驻代理入口配置
	Serve struct {
		// MixedPort SOCKS5+HTTP 混合代理端口, 客户端直连即走代理池; 0 = 关闭
		MixedPort int `yaml:"mixed_port"`
	} `yaml:"serve"`
	// Output 输出配置
	Output struct {
		// APIAddr 代理池 HTTP API 监听地址(/get /all /count)
		APIAddr string `yaml:"api_addr"`
		// APIToken API 访问令牌, 为空不认证; 设置后需 ?token= 或 X-API-Token 头
		APIToken string `yaml:"api_token"`
		// SubFile 生成新 Clash 订阅文件的路径
		SubFile string `yaml:"sub_file"`
		// LinksFile 分享链接文本输出路径(每行一条)
		LinksFile string `yaml:"links_file"`
		// B64File Base64 订阅输出路径
		B64File string `yaml:"b64_file"`
	} `yaml:"output"`
}

// Default 返回内置默认配置
func Default() Config {
	var c Config
	c.Subscriptions = []Subscription{}
	c.Check.TestURL = "https://www.gstatic.com/generate_204"
	c.Check.Timeout = 5000
	c.Check.MaxDelay = 3000
	c.Check.Concurrency = 50
	c.Check.Interval = 30
	c.Serve.MixedPort = 7890
	c.Output.APIAddr = "127.0.0.1:8080"
	c.Output.APIToken = ""
	c.Output.SubFile = "pool.yaml"
	c.Output.LinksFile = "pool-links.txt"
	c.Output.B64File = "pool-b64.txt"
	return c
}

// Load 从 YAML 文件加载配置, 文件不存在时返回默认配置
func Load(path string) (Config, error) {
	c := Default()
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
