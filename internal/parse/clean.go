package parse

import (
	"fmt"
	"regexp"
)

// fakeNameRe 匹配机场订阅中的"信息节点"名称(剩余流量/到期时间/官网等), 可按需调整
var fakeNameRe = regexp.MustCompile(
	`(?i)剩余|流量|到期|过期|套餐|官网|客服|邀请|重置|订阅|更新|通知|工单|邮箱|发布|电报|频道|地址|expire|traffic|official|website|telegram`)

// FilterFake 过滤信息类假节点, 返回真实节点与被过滤数量
func FilterFake(proxies []map[string]any) (real []map[string]any, fake int) {
	for _, p := range proxies {
		if fakeNameRe.MatchString(Str(p, "name")) {
			fake++
			continue
		}
		real = append(real, p)
	}
	return real, fake
}

// Dedup 严格去重: 协议+服务器+端口+凭据(uuid/password) 相同视为同一节点
func Dedup(proxies []map[string]any) (unique []map[string]any, dup int) {
	seen := map[string]bool{}
	for _, p := range proxies {
		key := dedupKey(p)
		if seen[key] {
			dup++
			continue
		}
		seen[key] = true
		unique = append(unique, p)
	}
	return unique, dup
}

func dedupKey(p map[string]any) string {
	cred := Str(p, "uuid")
	if cred == "" {
		cred = Str(p, "password")
	}
	return fmt.Sprintf("%s|%s|%d|%s", Str(p, "type"), Str(p, "server"), toInt(p["port"]), cred)
}

// ApplyPrefix 为节点名添加来源前缀: "[来源] 原名"
func ApplyPrefix(proxies []map[string]any, prefix string) []map[string]any {
	if prefix == "" {
		return proxies
	}
	for _, p := range proxies {
		p["name"] = fmt.Sprintf("[%s] %s", prefix, Str(p, "name"))
	}
	return proxies
}

// UniquifyNames 对同名节点追加序号(原名 / 原名-2 / 原名-3 ...)
func UniquifyNames(proxies []map[string]any) {
	count := map[string]int{}
	for _, p := range proxies {
		base := Str(p, "name")
		count[base]++
		if n := count[base]; n > 1 {
			p["name"] = fmt.Sprintf("%s-%d", base, n)
		}
	}
}

// Str 从代理 map 中取字符串字段, 非字符串值转为字符串表示
func Str(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// toInt 宽松地把 YAML/JSON 中的数字表示转为 int
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := parseIntSafe(n)
		return i
	}
	return 0
}

func parseIntSafe(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}
