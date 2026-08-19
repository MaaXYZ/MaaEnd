//go:build linux

package ziplineimport

import (
	"testing"
)

const markListBody = `{
    "code": 0,
    "data": {
        "marks": [
            {"templateId": "OFFICIAL_A", "mapId": "map01", "levelId": "lv", "pos": {"x": 1, "y": 2, "z": 3}}
        ],
        "saveMarks": [
            {"templateId": "5d53bdb714ba42c1e1a1b748b55b686f", "mapId": "map01", "levelId": "l1", "pos": {"x": 10, "y": 20, "z": 30}},
            {"templateId": "0f45150a59b97bd0de9a4eed7a0fbf23", "mapId": "map02", "levelId": "l2", "pos": {"x": 40, "y": 50, "z": 60}}
        ]
    }
}`

// 混合类型 saveMarks：滑索架 + 供电桩 + 无关自定义标记。空过滤必须全量保留——
// 供电结构丢失会让寻路把所有滑索判成不通电（回归锁：pipeline 曾误加 template_ids 过滤）。
const markListBodyMixed = `{
    "code": 0,
    "data": {
        "marks": [],
        "saveMarks": [
            {"templateId": "5d53bdb714ba42c1e1a1b748b55b686f", "mapId": "map01", "levelId": "l1", "pos": {"x": 1, "y": 2, "z": 3}},
            {"templateId": "5cc89ec3a2a9b4f00870c16936334bdf", "mapId": "map01", "levelId": "l1", "pos": {"x": 4, "y": 5, "z": 6}},
            {"templateId": "some_custom_pin", "mapId": "map01", "levelId": "", "pos": {"x": 7, "y": 8, "z": 9}}
        ]
    }
}`

// 空 saveMarks + 非空 marks（未登录时的服务端返回）：绝不能据此判定地图被覆盖。
const markListBodyNoSaveMarks = `{
    "code": 0,
    "data": {
        "marks": [
            {"templateId": "OFFICIAL_A", "mapId": "map01", "levelId": "lv", "pos": {"x": 1, "y": 2, "z": 3}}
        ],
        "saveMarks": []
    }
}`

// 只读 saveMarks：data.marks（官方点位）不得计入 covered，否则未登录时会提前判成抓齐。
func TestMarksByMapIgnoresOfficialMarks(t *testing.T) {
	r := capturedResponse{url: "https://zonai.skland.com/web/v1/game/endfield/map/mark/list?mapId=map01", body: []byte(markListBodyNoSaveMarks)}
	covered := coveredMaps([]capturedResponse{r})
	if len(covered) != 0 {
		t.Fatalf("空 saveMarks 不应有任何覆盖，got %v", covered)
	}
	byMap := marksByMap(r.body, nil, "map01")
	if len(byMap) != 0 {
		t.Fatalf("空 saveMarks 不应产出标记，got %v", byMap)
	}
}

// 空 template_ids = 不过滤、全部保留，与 win32 参考一致。
func TestMarksByMapKeepsAllWhenNoFilter(t *testing.T) {
	r := capturedResponse{url: "https://zonai.skland.com/web/v1/game/endfield/map/mark/list?mapId=map01", body: []byte(markListBodyMixed)}
	byMap := marksByMap(r.body, nil, "")
	if len(byMap["map01"]) != 3 {
		t.Fatalf("空过滤应全量保留（含供电结构与无关标记），got %d", len(byMap["map01"]))
	}
}

// 正常 saveMarks 按各自 mapId 归集，并按 template_ids 过滤。
func TestMarksByMapGroupAndFilter(t *testing.T) {
	r := capturedResponse{url: "https://zonai.skland.com/web/v1/game/endfield/map/mark/list", body: []byte(markListBody)}
	covered := coveredMaps([]capturedResponse{r})
	if !covered["map01"] || !covered["map02"] {
		t.Fatalf("应有 map01/map02 被覆盖，got %v", covered)
	}
	// 仅保留一种滑索 template。
	byMap := marksByMap(r.body, []string{"5d53bdb714ba42c1e1a1b748b55b686f"}, "")
	if len(byMap) != 1 || len(byMap["map01"]) != 1 {
		t.Fatalf("template 过滤后应只剩 map01 一条，got %v", byMap)
	}
}
