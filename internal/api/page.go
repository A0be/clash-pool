package api

// statusPage 状态面板(单文件 HTML, 原生 JS 轮询 API 渲染)
const statusPage = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>clash-pool 状态</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 14px/1.6 -apple-system, "PingFang SC", "Segoe UI", sans-serif;
         margin: 0; padding: 24px; background: #f6f7f9; color: #171717; }
  @media (prefers-color-scheme: dark) { body { background: #171717; color: #e5e5e5; } }
  h1 { font-size: 18px; margin: 0 0 16px; }
  .cards { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 16px; }
  .card { background: #fff; border: 1px solid rgba(0,0,0,.1); border-radius: 10px;
          padding: 12px 16px; min-width: 120px; }
  @media (prefers-color-scheme: dark) { .card { background: #262626; border-color: rgba(255,255,255,.12); } }
  .card .num { font-size: 22px; font-weight: 600; }
  .card .lbl { font-size: 12px; opacity: .65; }
  table { border-collapse: collapse; width: 100%; background: #fff;
          border: 1px solid rgba(0,0,0,.1); border-radius: 10px; overflow: hidden; }
  @media (prefers-color-scheme: dark) { table { background: #262626; border-color: rgba(255,255,255,.12); } }
  th, td { padding: 8px 12px; text-align: left; border-bottom: 1px solid rgba(0,0,0,.06); }
  @media (prefers-color-scheme: dark) { th, td { border-color: rgba(255,255,255,.08); } }
  th { font-size: 12px; opacity: .65; font-weight: 500; }
  tr:last-child td { border-bottom: none; }
  .tag { display: inline-block; padding: 1px 8px; border-radius: 99px;
         font-size: 12px; background: rgba(0,0,0,.06); }
  @media (prefers-color-scheme: dark) { .tag { background: rgba(255,255,255,.1); } }
  .good { color: #05994f; } .mid { color: #b45309; } .bad { color: #dc2626; }
  #updated { font-size: 12px; opacity: .6; margin-top: 12px; }
</style>
</head>
<body>
<h1>clash-pool 代理池状态</h1>
<div class="cards" id="cards"></div>
<table>
  <thead><tr><th>节点</th><th>地区</th><th>协议</th><th>延迟</th></tr></thead>
  <tbody id="rows"></tbody>
</table>
<div id="updated"></div>
<script>
function delayClass(d) { return d < 300 ? 'good' : d < 1000 ? 'mid' : 'bad'; }
function esc(s) { const d = document.createElement('td'); d.textContent = s; return d.innerHTML; }
async function refresh() {
  try {
    const [stats, all] = await Promise.all([
      fetch('stats').then(r => r.json()),
      fetch('all').then(r => r.json()),
    ]);
    let cards = '<div class="card"><div class="num">' + stats.total + '</div><div class="lbl">存活节点</div></div>';
    for (const [region, n] of Object.entries(stats.regions || {})) {
      cards += '<div class="card"><div class="num">' + n + '</div><div class="lbl">' + region + '</div></div>';
    }
    document.getElementById('cards').innerHTML = cards;

    const rows = (all.nodes || []).map(n =>
      '<tr><td>' + esc(n.name) + '</td>' +
      '<td><span class="tag">' + esc(n.region) + '</span></td>' +
      '<td>' + esc(n.type) + '</td>' +
      '<td class="' + delayClass(n.delay_ms) + '">' + n.delay_ms + ' ms</td></tr>');
    document.getElementById('rows').innerHTML = rows.join('');
    document.getElementById('updated').textContent = '更新于 ' + new Date().toLocaleTimeString() + ' · 30 秒自动刷新';
  } catch (e) {
    document.getElementById('updated').textContent = '加载失败: ' + e;
  }
}
refresh();
setInterval(refresh, 30000);
</script>
</body>
</html>`
