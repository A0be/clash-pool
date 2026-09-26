package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动, 无 CGO 依赖
)

// HistoryRow 一条延迟历史记录
type HistoryRow struct {
	TS      time.Time
	Name    string
	Region  string
	DelayMS int
}

// History 延迟历史库(SQLite)
type History struct {
	db          *sql.DB
	retentionNS int64 // 历史保留时长, 超出自动清理
}

// OpenHistory 打开(或创建)延迟历史库; retention 为历史保留时长(如 7*24h)
func OpenHistory(path string, retention time.Duration) (*History, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// 单连接即可: 写入低频批量, 避免 SQLITE_BUSY
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS history (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		ts        INTEGER NOT NULL,
		name      TEXT    NOT NULL,
		region    TEXT    NOT NULL DEFAULT '',
		delay_ms  INTEGER NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_history_name_ts ON history(name, ts)`); err != nil {
		db.Close()
		return nil, err
	}
	return &History{db: db, retentionNS: int64(retention)}, nil
}

// Close 关闭数据库
func (h *History) Close() error { return h.db.Close() }

// Record 批量写入一轮测速结果(同一时间戳), 顺带清理过期历史
func (h *History) Record(rows []HistoryRow) error {
	if len(rows) == 0 {
		return nil
	}
	ts := time.Now().Unix()
	tx, err := h.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO history (ts, name, region, delay_ms) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(ts, r.Name, r.Region, r.DelayMS); err != nil {
			return err
		}
	}
	if h.retentionNS > 0 {
		cutoff := time.Now().Add(-time.Duration(h.retentionNS)).Unix()
		if _, err := tx.Exec(`DELETE FROM history WHERE ts < ?`, cutoff); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Recent 查询某节点最近 n 条延迟记录(新→旧)
func (h *History) Recent(name string, n int) ([]HistoryRow, error) {
	rows, err := h.db.Query(
		`SELECT ts, name, region, delay_ms FROM history WHERE name = ? ORDER BY ts DESC, id DESC LIMIT ?`,
		name, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HistoryRow
	for rows.Next() {
		var r HistoryRow
		var ts int64
		if err := rows.Scan(&ts, &r.Name, &r.Region, &r.DelayMS); err != nil {
			return nil, err
		}
		r.TS = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

// HistoryCount 总历史记录数(用于测试与诊断)
func (h *History) HistoryCount() (int, error) {
	var n int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM history`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// Ping 校验数据库可用
func (h *History) Ping() error { return h.db.Ping() }
