# clash-pool

基于 [mihomo（Clash.Meta）](https://github.com/MetaCubeX/mihomo) 内核的代理池管理工具。

## 功能

- **订阅适配**：支持 Clash YAML 订阅与 Base64/明文分享链接（`ss://` `vmess://` `trojan://` `vless://` `hysteria2://`），多订阅聚合，URL 与本地文件均可作为订阅源
- **智能清洗**：自动过滤"剩余流量/到期时间/官网"类信息假节点；按 协议+服务器+端口+凭据 严格去重；节点名自动加来源前缀 `[机场A]`
- **自动测速验活**：借助 mihomo 内核 `external-controller` API 并发测试节点延迟，自动剔除超时/失效节点
- **代理池输出**：
  - 生成新的 Clash 订阅文件，可直接导入任意 clash 客户端
  - HTTP API：`GET /get`（随机存活节点）、`GET /all`（全部存活节点）、`GET /count`（数量）
- **定时循环**：按间隔自动刷新订阅并复测存活
- **持久化**：节点状态存 YAML，延迟历史存 SQLite

## 架构

```
Clash 订阅源 → 拉取与解析(去重) → mihomo 内核 → 并发测速验活 → 代理池输出
                     ↑                                    │
                     └──────────── 定时循环刷新 ←──────────┘
```

## 快速开始

```bash
# 1. 复制配置并填入你的订阅链接
cp config.example.yaml config.yaml

# 2. 运行
go run .
```

## 路线图

- [x] 阶段 0：项目骨架与配置
- [x] 阶段 1：订阅拉取与解析（Clash YAML / Base64 / 明文链接、假节点过滤、严格去重、来源前缀）
- [ ] 阶段 2：mihomo 内核管理（自动下载 / 启动 / 停止）+ 并发测速验活（多测试 URL）
- [ ] 阶段 3：代理池输出（Clash YAML / 分享链接 / Base64 订阅 + HTTP API 地区/协议筛选）
- [ ] 阶段 4：定时循环 + 持久化（节点状态 YAML + 延迟历史 SQLite）+ Web 状态面板
- [ ] 阶段 5（可选）：TG 频道/网页节点抓取

## 致谢

- [mihomo](https://github.com/MetaCubeX/mihomo) — Clash.Meta 内核
- [snakem982/proxypool](https://github.com/snakem982/proxypool) — Mihomo/Clash 节点池参考
- [ssrlive/proxypool](https://github.com/ssrlive/proxypool) — 节点抓取聚合参考
- [wzdnzd/aggregator](https://github.com/wzdnzd/aggregator) — Clash API 测活方案参考
- [jhao104/proxy_pool](https://github.com/jhao104/proxy_pool) — 代理池 API 设计参考

## License

MIT
