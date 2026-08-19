//go:build linux

package ziplineimport

import (
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// markDTO 对应 mark/list 响应里 data.marks / data.saveMarks 的单条标记。
//
// 滑索数据在 data.saveMarks（data.marks 是官方点位/资源/敌人）。pos 的 x/z 张成水平面、
// y 是高度，与 Ziplines.json 的坐标语义一致。pos 缺省时为 nil（连线之类没有落点的标记）。
type markDTO struct {
	TemplateID string   `json:"templateId"`
	MapID      string   `json:"mapId"`
	LevelID    string   `json:"levelId"`
	Pos        *markPos `json:"pos"`
}

type markPos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type marksPayload struct {
	Data struct {
		// marks 是官方点位/资源/敌人，非滑索，解析时有意忽略（与 cpp 一致）。
		// 保留字段仅是记录接口结构，代码不读它。
		Marks     []markDTO `json:"marks"`
		SaveMarks []markDTO `json:"saveMarks"`
	} `json:"data"`
}

// capturedResponse 是一条被抄下的 mark/list 响应（URL 与响应体）。
type capturedResponse struct {
	url  string
	body []byte
}

// isMarkListResponse 判断请求是否命中 mark/list 接口。只匹配路径片段，
// 避免被 query 里的参数出现顺序影响。
func isMarkListResponse(path string) bool {
	return strings.Contains(path, "/map/mark/list")
}

// saveMarksPresent 判断响应是否为合法的 mark/list（至少带 data.saveMarks 结构）。
// 未登录时 saveMarks 是空数组，也算 present（只是内容为空）。
func saveMarksPresent(body []byte) bool {
	var p marksPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return false
	}
	return p.Data.SaveMarks != nil
}

// queryValue 取 URL query 里某个参数的值；取不到返回空串。
func queryValue(rawurl, key string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

// marksByMap 解析一份 mark/list 响应，按每条标记自己的 mapId 归集（标记缺 mapId 时
// 回退到请求 URL 上的 mapId），再按 templateIDs 过滤、丢弃没有 pos 的标记。
// 返回 mapId -> 滑索列表。
//
// 与 cpp 一致，只读 data.saveMarks（用户的滑索标记）；data.marks 是官方点位/资源/敌人，
// 不是滑索，绝不能并入——否则未登录时 marks 非空会把 covered 撑满、导致提前判成抓齐。
func marksByMap(body []byte, templateIDs []string, fallbackMapID string) map[string][]ziplineMark {
	var p marksPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil
	}
	out := make(map[string][]ziplineMark)
	for _, m := range p.Data.SaveMarks {
		if len(templateIDs) > 0 && !contains(templateIDs, m.TemplateID) {
			continue
		}
		if m.Pos == nil {
			continue
		}
		mapID := m.MapID
		if mapID == "" {
			mapID = fallbackMapID
		}
		if mapID == "" {
			continue
		}
		out[mapID] = append(out[mapID], ziplineMark{
			TemplateID: m.TemplateID,
			LevelID:    m.LevelID,
			X:          m.Pos.X,
			Y:          m.Pos.Y,
			Z:          m.Pos.Z,
		})
	}
	return out
}

// coveredMaps 汇总所有响应里「出现真实标记」的地图集合。不按 template 过滤（与 cpp 的
// covered 一致）：只要任何标记带落点就算该图被覆盖；真正是否作为滑索留下由落盘时的
// template_ids 过滤决定。
func coveredMaps(responses []capturedResponse) map[string]bool {
	out := make(map[string]bool)
	for _, r := range responses {
		for id := range marksByMap(r.body, nil, queryValue(r.url, "mapId")) {
			out[id] = true
		}
	}
	return out
}

// dedupMarks 去掉完全重合（template_id/level_id/x/y/z 完全相同）的重复标记，并按该键
// 排定落盘顺序。与 cpp PersistCaptured 的去重逻辑一致。
func dedupMarks(marks []ziplineMark) []ziplineMark {
	if len(marks) == 0 {
		return marks
	}
	key := func(m ziplineMark) string {
		return m.TemplateID + "\x00" + m.LevelID + "\x00" +
			strconv.FormatFloat(m.X, 'g', -1, 64) + "\x00" +
			strconv.FormatFloat(m.Y, 'g', -1, 64) + "\x00" +
			strconv.FormatFloat(m.Z, 'g', -1, 64)
	}
	sort.SliceStable(marks, func(i, j int) bool { return key(marks[i]) < key(marks[j]) })
	out := marks[:0]
	var last string
	for i, m := range marks {
		k := key(m)
		if i == 0 || k != last {
			out = append(out, m)
			last = k
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
