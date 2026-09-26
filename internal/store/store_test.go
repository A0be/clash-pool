package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	snap := StateSnapshot{
		UpdatedAt: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC),
		Alive:     2,
		Dead:      1,
		Subscriptions: []SubStat{{Name: "机场A", Parsed: 3, Fake: 1}},
		Nodes: []NodeState{
			{Name: "香港01", Type: "ss", Region: "HK", DelayMS: 100},
			{Name: "US-02", Type: "trojan", Region: "US", DelayMS: 200},
		},
	}
	if err := SaveState(path, snap); err != nil {
		t.Fatalf("SaveState 报错: %v", err)
	}

	got, ok, err := LoadState(path)
	if err != nil || !ok {
		t.Fatalf("LoadState 报错: %v ok=%v", err, ok)
	}
	if got.Alive != 2 || got.Dead != 1 || len(got.Nodes) != 2 {
		t.Errorf("快照字段不一致: %+v", got)
	}
	if got.Nodes[0].Name != "香港01" || got.Nodes[0].DelayMS != 100 {
		t.Errorf("节点数据不一致: %+v", got.Nodes[0])
	}
	if len(got.Subscriptions) != 1 || got.Subscriptions[0].Fake != 1 {
		t.Errorf("订阅统计不一致: %+v", got.Subscriptions)
	}
}

func TestStateMissing(t *testing.T) {
	_, ok, err := LoadState(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil || ok {
		t.Errorf("文件不存在时应返回 ok=false err=nil, 实际 ok=%v err=%v", ok, err)
	}
}

func TestHistoryRecordAndRecent(t *testing.T) {
	dir := t.TempDir()
	h, err := OpenHistory(filepath.Join(dir, "history.db"), 24*time.Hour)
	if err != nil {
		t.Fatalf("OpenHistory 报错: %v", err)
	}
	defer h.Close()

	rows := []HistoryRow{
		{Name: "香港01", Region: "HK", DelayMS: 100},
		{Name: "香港01", Region: "HK", DelayMS: 120},
		{Name: "US-02", Region: "US", DelayMS: 200},
	}
	if err := h.Record(rows); err != nil {
		t.Fatalf("Record 报错: %v", err)
	}
	if err := h.Record([]HistoryRow{{Name: "香港01", Region: "HK", DelayMS: 90}}); err != nil {
		t.Fatalf("Record 第二轮报错: %v", err)
	}

	if n, _ := h.HistoryCount(); n != 4 {
		t.Errorf("总记录数 = %d, 期望 4", n)
	}

	recent, err := h.Recent("香港01", 2)
	if err != nil {
		t.Fatalf("Recent 报错: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("Recent 数量 = %d, 期望 2", len(recent))
	}
	// 新→旧: 最新一条应是第二轮的 90ms
	if recent[0].DelayMS != 90 || recent[1].DelayMS != 120 {
		t.Errorf("Recent 顺序错误: %+v", recent)
	}

	// 不存在的节点
	if got, _ := h.Recent("无此节点", 5); len(got) != 0 {
		t.Errorf("不存在节点应返回空, 实际 %v", got)
	}
}

func TestHistoryRetention(t *testing.T) {
	dir := t.TempDir()
	h, err := OpenHistory(filepath.Join(dir, "history.db"), time.Nanosecond)
	if err != nil {
		t.Fatalf("OpenHistory 报错: %v", err)
	}
	defer h.Close()

	// 保留时长 1ns → 写入后立即全部过期, 下一轮 Record 触发清理
	if err := h.Record([]HistoryRow{{Name: "a", Region: "HK", DelayMS: 1}}); err != nil {
		t.Fatal(err)
	}
	// 历史时间戳为秒级精度, 需跨过秒边界使过期判定生效
	time.Sleep(1100 * time.Millisecond)
	if err := h.Record([]HistoryRow{{Name: "b", Region: "US", DelayMS: 2}}); err != nil {
		t.Fatal(err)
	}
	if n, _ := h.HistoryCount(); n != 1 {
		t.Errorf("过期清理后应只剩 1 条, 实际 %d", n)
	}
}
