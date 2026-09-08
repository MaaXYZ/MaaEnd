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

const (
	// 与上游日志里出现过的真实 roleId 同为 10 位数字。
	testRoleID      = "1928666689"
	testOtherRoleID = "2028666689"
)

// markListURL 拼一条 mark/list 请求 URL；roleID 为空表示未登录/未选角色的请求。
func markListURL(mapID, roleID string) string {
	u := "https://zonai.skland.com/web/v1/game/endfield/map/mark/list?mapId=" + mapID
	if roleID != "" {
		u += "&roleId=" + roleID + "&serverId=1"
	}
	return u
}

// stubDeriveAccountID 替换账号标识计算，避免测试在包目录里生成真实盐文件。
func stubDeriveAccountID(t *testing.T) {
	t.Helper()
	original := deriveAccountID
	deriveAccountID = func(uid string) (string, error) { return "account-" + uid, nil }
	t.Cleanup(func() { deriveAccountID = original })
}

// 只读 saveMarks：data.marks（官方点位）不得计入 covered，否则未登录时会提前判成抓齐。
func TestMarksByMapIgnoresOfficialMarks(t *testing.T) {
	r := capturedResponse{url: markListURL("map01", testRoleID), body: []byte(markListBodyNoSaveMarks)}
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
	r := capturedResponse{url: markListURL("map01", testRoleID), body: []byte(markListBodyMixed)}
	byMap := marksByMap(r.body, nil, "")
	if len(byMap["map01"]) != 3 {
		t.Fatalf("空过滤应全量保留（含供电结构与无关标记），got %d", len(byMap["map01"]))
	}
}

// 正常 saveMarks 按各自 mapId 归集，并按 template_ids 过滤。
func TestMarksByMapGroupAndFilter(t *testing.T) {
	r := capturedResponse{url: markListURL("map01", testRoleID), body: []byte(markListBody)}
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

// covered 只认带合法 roleId 的响应：未登录（无 roleId）时官方点位再多也不算覆盖。
func TestCoveredMapsRequiresValidRoleID(t *testing.T) {
	body := []byte(markListBody)
	withoutRole := coveredMaps([]capturedResponse{{url: markListURL("map01", ""), body: body}})
	if len(withoutRole) != 0 {
		t.Fatalf("无 roleId 的响应不应推进覆盖，got %v", withoutRole)
	}
	invalidRole := coveredMaps([]capturedResponse{{url: markListURL("map01", "123"), body: body}})
	if len(invalidRole) != 0 {
		t.Fatalf("非法 roleId 的响应不应推进覆盖，got %v", invalidRole)
	}
	withRole := coveredMaps([]capturedResponse{{url: markListURL("map01", testRoleID), body: body}})
	if !withRole["map01"] || !withRole["map02"] {
		t.Fatalf("带合法 roleId 的响应应覆盖 map01/map02，got %v", withRole)
	}
}

// 单账号：多张图合并到同一个 account_id 下。
func TestAccountScopedMarksSingleAccount(t *testing.T) {
	stubDeriveAccountID(t)
	responses := []capturedResponse{
		{url: markListURL("map01", testRoleID), body: []byte(markListBody)},
		{url: markListURL("map02", testRoleID), body: []byte(markListBody)},
	}
	accountID, byMap, err := accountScopedMarks(responses, nil)
	if err != nil {
		t.Fatalf("accountScopedMarks: %v", err)
	}
	if want := "account-" + testRoleID; accountID != want {
		t.Fatalf("accountID = %q, want %q", accountID, want)
	}
	// 同图重复响应按并集保留，落盘阶段再去重（与 cpp 一致）。
	if len(byMap["map01"]) != 2 || len(byMap["map02"]) != 2 {
		t.Fatalf("应按地图归集并合并所有响应，got %v", byMap)
	}
}

// 没有可归属账号的标记：整批拒绝，绝不能落盘。
func TestAccountScopedMarksRefusesWithoutValidRoleID(t *testing.T) {
	stubDeriveAccountID(t)
	responses := []capturedResponse{
		{url: markListURL("map01", ""), body: []byte(markListBody)},
		{url: markListURL("map02", "123"), body: []byte(markListBody)},
	}
	if _, _, err := accountScopedMarks(responses, nil); err == nil {
		t.Fatal("无有效 roleId 时应拒绝落盘")
	}
}

// 一次导入出现两个账号：整批拒绝，避免把两个账号的坐标混在一起。
func TestAccountScopedMarksRefusesMultipleAccounts(t *testing.T) {
	stubDeriveAccountID(t)
	responses := []capturedResponse{
		{url: markListURL("map01", testRoleID), body: []byte(markListBody)},
		{url: markListURL("map02", testOtherRoleID), body: []byte(markListBody)},
	}
	if _, _, err := accountScopedMarks(responses, nil); err == nil {
		t.Fatal("多个 roleId 时应拒绝落盘")
	}
}

// 空 saveMarks 的响应不参与账号判定：它带的 roleId 不能算作第二个账号。
func TestAccountScopedMarksIgnoresEmptySaveMarksForAccount(t *testing.T) {
	stubDeriveAccountID(t)
	responses := []capturedResponse{
		{url: markListURL("map01", testOtherRoleID), body: []byte(markListBodyNoSaveMarks)},
		{url: markListURL("map02", testRoleID), body: []byte(markListBody)},
	}
	accountID, _, err := accountScopedMarks(responses, nil)
	if err != nil {
		t.Fatalf("空 saveMarks 不应污染账号判定: %v", err)
	}
	if want := "account-" + testRoleID; accountID != want {
		t.Fatalf("accountID = %q, want %q", accountID, want)
	}
}
