// Package fetch 拉取订阅内容, 支持 http(s) URL 与本地文件路径
package fetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Fetch 拉取订阅内容: http(s) URL 或本地文件路径
func Fetch(source string) ([]byte, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return fetchHTTP(source)
	}
	if _, err := os.Stat(source); err != nil {
		return nil, fmt.Errorf("订阅源无效(既非 URL 也非本地文件): %s", source)
	}
	return os.ReadFile(source)
}

func fetchHTTP(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// 机场通常按 User-Agent 判断客户端类型, 使用 clash.meta 以获取 clash 格式订阅
	req.Header.Set("User-Agent", "clash.meta")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// 上限 32MB, 防御异常超大响应
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("响应内容为空")
	}
	return body, nil
}
