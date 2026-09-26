// Package pool 存活节点池: 地区识别、筛选获取、多格式输出
package pool

import (
	"math/rand"
	"regexp"
	"strings"
	"sync"

	"github.com/A0be/clash-pool/internal/checker"
	"github.com/A0be/clash-pool/internal/parse"
)

// Node 代理池中的存活节点
type Node struct {
	Name    string
	Type    string
	Region  string // HK/SG/JP/US... 或 other
	DelayMS int
	Proxy   map[string]any
	Link    string // 分享链接(不支持编码的类型为空)
}

// Pool 线程安全的存活节点池
type Pool struct {
	mu    sync.RWMutex
	nodes []Node
}

// regionRules 地区识别规则(按顺序匹配, 命中即返回)
var regionRules = []struct {
	Region  string
	Pattern *regexp.Regexp
}{
	{"HK", regexp.MustCompile(`(?i)香港|🇭🇰|\bHK\b|HKG|Hong ?Kong`)},
	{"TW", regexp.MustCompile(`(?i)台湾|台灣|台北|🇹🇼|\bTW\b|TPE|Taiwan`)},
	{"SG", regexp.MustCompile(`(?i)新加坡|狮城|獅城|🇸🇬|\bSG\b|SIN|Singapore`)},
	{"JP", regexp.MustCompile(`(?i)日本|东京|東京|大阪|🇯🇵|\bJP\b|NRT|KIX|Japan`)},
	{"KR", regexp.MustCompile(`(?i)韩国|韓國|首尔|首爾|🇰🇷|\bKR\b|ICN|Korea|Seoul`)},
	{"US", regexp.MustCompile(`(?i)美国|美國|洛杉矶|洛杉磯|圣何塞|聖何塞|凤凰城|硅谷|🇺🇸|\bUSA?\b|United States|Los Angeles|San Jose|LAX|SJC|Seattle`)},
	{"EU", regexp.MustCompile(`(?i)英国|英國|德国|德國|法国|法國|荷兰|荷蘭|俄罗斯|俄羅斯|伦敦|法兰克福|阿姆斯特丹|巴黎|🇬🇧|🇩🇪|🇫🇷|🇳🇱|🇷🇺|\bEU\b|Europe|London|Frankfurt|Amsterdam|Paris|Russia|Moscow`)},
}

// DetectRegion 根据节点名识别地区, 无法识别返回 other
func DetectRegion(name string) string {
	for _, r := range regionRules {
		if r.Pattern.MatchString(name) {
			return r.Region
		}
	}
	return "other"
}

// Build 由测速存活结果构建代理池
func Build(alive []checker.Result) *Pool {
	nodes := make([]Node, 0, len(alive))
	for _, a := range alive {
		name := parse.Str(a.Proxy, "name")
		nodes = append(nodes, Node{
			Name:    name,
			Type:    parse.Str(a.Proxy, "type"),
			Region:  DetectRegion(name),
			DelayMS: a.DelayMS,
			Proxy:   a.Proxy,
			Link:    ShareLink(a.Proxy),
		})
	}
	return &Pool{nodes: nodes}
}

// Nodes 返回节点快照(拷贝)
func (p *Pool) Nodes() []Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Node, len(p.nodes))
	copy(out, p.nodes)
	return out
}

// ProxyMaps 返回全部节点原始配置
func (p *Pool) ProxyMaps() []map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]map[string]any, 0, len(p.nodes))
	for _, n := range p.nodes {
		out = append(out, n.Proxy)
	}
	return out
}

// Filter 按地区/协议筛选(参数为空表示不限)
func (p *Pool) Filter(region, typ string) []Node {
	var out []Node
	for _, n := range p.Nodes() {
		if region != "" && !strings.EqualFold(n.Region, region) {
			continue
		}
		if typ != "" && !strings.EqualFold(n.Type, typ) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// Get 随机取一个匹配节点, 无匹配返回 nil
func (p *Pool) Get(region, typ string) *Node {
	nodes := p.Filter(region, typ)
	if len(nodes) == 0 {
		return nil
	}
	n := nodes[rand.Intn(len(nodes))]
	return &n
}

// Count 统计匹配节点数量
func (p *Pool) Count(region, typ string) int {
	return len(p.Filter(region, typ))
}

// Replace 原子替换池内全部节点(定时循环刷新时调用)
func (p *Pool) Replace(nodes []Node) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nodes = nodes
}

// Stats 池统计信息
type Stats struct {
	Total   int            `json:"total"`
	Regions map[string]int `json:"regions"`
	Types   map[string]int `json:"types"`
}

// Stats 返回当前池的地区/协议分布统计
func (p *Pool) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s := Stats{Total: len(p.nodes), Regions: map[string]int{}, Types: map[string]int{}}
	for _, n := range p.nodes {
		s.Regions[n.Region]++
		s.Types[n.Type]++
	}
	return s
}
