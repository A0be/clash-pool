# clash-pool

基于 [mihomo（Clash.Meta）](https://github.com/MetaCubeX/mihomo) 内核的代理池管理工具。

## 功能

- **订阅适配**：支持 Clash YAML 订阅与 Base64/明文分享链接（`ss://` `vmess://` `trojan://` `vless://` `hysteria2://`），多订阅聚合，URL 与本地文件均可作为订阅源
- **智能清洗**：自动过滤"剩余流量/到期时间/官网"类信息假节点；按 协议+服务器+端口+凭据 严格去重；节点名自动加来源前缀 `[机场A]`
- **自动测速验活**：自动下载 mihomo 内核（也支持手动放置），通过 `external-controller` API 并发测试节点延迟（默认 50 并发），自动剔除超时/失效节点
- **代理池输出**：
  - 生成新的 Clash 订阅文件（含 POOL 自动选择组），可直接导入任意 clash 客户端
  - 分享链接文本（`ss://` 等，每行一条）与 Base64 订阅两种额外格式
  - HTTP API：`GET /get`（随机存活节点）、`GET /all`（全部）、`GET /count`（数量），支持 `region`/`type` 筛选参数，可选 token 认证
- **SOCKS5/HTTP 直连入口**：常驻 mihomo 实例开放混合端口（默认 7890），客户端直连即走代理池，POOL 组（url-test）自动选择当前延迟最低的节点
- **定时循环**：按间隔自动刷新订阅并复测存活
- **持久化**：节点状态存 YAML，延迟历史存 SQLite

## 架构

```
Clash 订阅源 → 拉取与解析(去重) → mihomo 内核 → 并发测速验活 → 代理池输出
                     ↑                                    │
                     └──────────── 定时循环刷新 ←──────────┘
```

## 使用

```bash
# 1. 复制配置并填入你的订阅链接
cp config.example.yaml config.yaml

# 2. 运行
go run .
```

运行后即可：

```bash
# SOCKS5 直接调用代理池(自动走最低延迟节点)
curl -x socks5h://127.0.0.1:7890 https://www.google.com

# HTTP 代理方式同理
curl -x http://127.0.0.1:7890 https://www.google.com

# 从池中随机取一个香港节点
curl "http://127.0.0.1:8080/get?region=hk"

# 查看全部存活节点 / 数量
curl http://127.0.0.1:8080/all
curl http://127.0.0.1:8080/count

# 把生成的 pool.yaml 导入任意 clash 客户端也可使用
```

## 路线图

- [x] 阶段 0：项目骨架与配置
- [x] 阶段 1：订阅拉取与解析（Clash YAML / Base64 / 明文链接、假节点过滤、严格去重、来源前缀）
- [x] 阶段 2：mihomo 内核管理（自动下载 / 手动放置兜底）+ 并发测速验活（默认 50 并发）
- [x] 阶段 3：代理池输出（Clash YAML / 分享链接 / Base64 订阅 + HTTP API 地区/协议筛选 + 可选 token）+ SOCKS5/HTTP 直连入口
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
