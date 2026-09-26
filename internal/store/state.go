// Package store 持久化: 节点状态 YAML + 延迟历史 SQLite
package store

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// SubStat 单个订阅源的解析统计
type SubStat struct {
	Name   string `yaml:"name"`
	Parsed int    `yaml:"parsed"`
	Fake   int    `yaml:"fake"`
}

// NodeState 节点状态快照
type NodeState struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Region  string `yaml:"region"`
	DelayMS int    `yaml:"delay_ms"`
}

// StateSnapshot 一轮循环结束时的状态快照
type StateSnapshot struct {
	UpdatedAt     time.Time   `yaml:"updated_at"`
	Alive         int         `yaml:"alive"`
	Dead          int         `yaml:"dead"`
	Subscriptions []SubStat   `yaml:"subscriptions,omitempty"`
	Nodes         []NodeState `yaml:"nodes"`
}

// SaveState 原子写出状态 YAML(先写临时文件再 rename, 避免写一半被读到)
func SaveState(path string, snap StateSnapshot) error {
	data, err := yaml.Marshal(snap)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadState 读取状态 YAML(文件不存在返回零值与 false)
func LoadState(path string) (StateSnapshot, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StateSnapshot{}, false, nil
		}
		return StateSnapshot{}, false, err
	}
	var snap StateSnapshot
	if err := yaml.Unmarshal(data, &snap); err != nil {
		return StateSnapshot{}, false, err
	}
	return snap, true, nil
}
