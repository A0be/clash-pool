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
		// Interval 定时复测间隔(分钟), 0 表示只跑一次
		Interval int `yaml:"interval_min"`
	} `yaml:"check"`
	// Output 输出配置
	Output struct {
		// APIAddr 代理池 HTTP API 监听地址(/get /all /count)
		APIAddr string `yaml:"api_addr"`
		// SubFile 生成新订阅文件的路径
		SubFile string `yaml:"sub_file"`
	} `yaml:"output"`
}

// Default 返回内置默认配置
func Default() Config {
	var c Config
	c.Subscriptions = []Subscription{}
	c.Check.TestURL = "https://www.gstatic.com/generate_204"
	c.Check.Timeout = 5000
	c.Check.MaxDelay = 3000
	c.Check.Interval = 30
	c.Output.APIAddr = "127.0.0.1:8080"
	c.Output.SubFile = "pool.yaml"
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
