//go:build linux

package ziplineimport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 同一张图在不同账号下必须并存，同账号才整张替换；旧版无 account_id 的记录是第三个 key。
func TestReplaceMapKeysByAccountAndMap(t *testing.T) {
	rec := &recordFile{}
	rec.replaceMap(ziplineMapRecord{AccountID: "aaaa", MapID: "map01", Marks: []ziplineMark{{TemplateID: "t1"}}})
	rec.replaceMap(ziplineMapRecord{AccountID: "bbbb", MapID: "map01", Marks: []ziplineMark{{TemplateID: "t2"}}})
	rec.replaceMap(ziplineMapRecord{MapID: "map01", Marks: []ziplineMark{{TemplateID: "legacy"}}})
	if len(rec.Maps) != 3 {
		t.Fatalf("三个不同 key 应并存，got %d", len(rec.Maps))
	}

	rec.replaceMap(ziplineMapRecord{AccountID: "aaaa", MapID: "map01", Marks: []ziplineMark{{TemplateID: "t3"}}})
	if len(rec.Maps) != 3 {
		t.Fatalf("同 (account, map) 应整张替换而不是追加，got %d", len(rec.Maps))
	}
	if got := rec.Maps[0].Marks[0].TemplateID; got != "t3" {
		t.Fatalf("同 (account, map) 替换后标记 = %q, want t3", got)
	}
	if got := rec.Maps[1].Marks[0].TemplateID; got != "t2" {
		t.Fatalf("替换不应波及其他账号，map[1] = %q, want t2", got)
	}
	if got := rec.Maps[2].Marks[0].TemplateID; got != "legacy" {
		t.Fatalf("替换不应波及旧版记录，map[2] = %q, want legacy", got)
	}
}

// 空 account_id 不落字段，避免把旧版记录改写成带空字段的形式（与 cpp ZiplineStore::save 一致）。
func TestSaveOmitsEmptyAccountID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Ziplines.json")
	rec := &recordFile{}
	rec.replaceMap(ziplineMapRecord{MapID: "map01", Marks: []ziplineMark{{TemplateID: "legacy"}}})
	rec.replaceMap(ziplineMapRecord{AccountID: "abcd1234abcd1234", MapID: "map02", Marks: []ziplineMark{{TemplateID: "t1"}}})
	if err := rec.save(path); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed struct {
		Maps []map[string]any `json:"maps"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Maps) != 2 {
		t.Fatalf("maps = %d, want 2", len(parsed.Maps))
	}
	if _, ok := parsed.Maps[0]["account_id"]; ok {
		t.Fatalf("旧版记录不应写出 account_id 字段: %v", parsed.Maps[0])
	}
	if got := parsed.Maps[1]["account_id"]; got != "abcd1234abcd1234" {
		t.Fatalf("新记录 account_id = %v, want abcd1234abcd1234", got)
	}
}

// 旧格式文件读回后原样保留，新账号记录追加而不是覆盖旧记录。
func TestLoadKeepsLegacyAndAddsScopedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Ziplines.json")
	legacy := `{"updated_at":"2026-01-01T00:00:00Z","maps":[{"map_id":"map01","fetched_at":"2026-01-01T00:00:00Z","marks":[{"template_id":"legacy","level_id":"l1","x":1,"y":2,"z":3}]}]}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	rec, err := loadRecord(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rec.replaceMap(ziplineMapRecord{AccountID: "abcd1234abcd1234", MapID: "map01", Marks: []ziplineMark{{TemplateID: "t1"}}})
	if len(rec.Maps) != 2 {
		t.Fatalf("旧记录应保留并与新账号记录并存，got %d", len(rec.Maps))
	}
	if rec.Maps[0].AccountID != "" || rec.Maps[0].Marks[0].TemplateID != "legacy" {
		t.Fatalf("旧记录被改动: %+v", rec.Maps[0])
	}
}
