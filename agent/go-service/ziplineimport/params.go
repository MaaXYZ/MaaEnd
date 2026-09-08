//go:build linux

package ziplineimport

import (
	"encoding/json"
	"strings"
)

// actionParam 是本自定义动作的入参。字段与 pipeline 的 custom_action_param 对应。
//
// url / timeout / template_ids 与 cpp-algo 的 WebView2 版共享（cpp 也读这几个键）；
// firefox / proxy_port 是 Linux 特有（firefox 可执行路径、本机受限 MITM 代理端口）。
//
// cpp 版的 mark_list_path / width / height / clear_login 在 Linux 被接受但有意忽略：接口
// 路径固定为 /map/mark/list，窗口由独立 Firefox 进程自己管理，每次导入都用一次性临时
// profile（等价于永远清除登录态），所以这三个键没有可生效的语义。json.Unmarshal 默认忽略
// 未知字段，不必为它们建结构体字段。
type actionParam struct {
	// 登录窗口打开的页面地址（沿用 cpp 版的 url 语义）。
	URL string `json:"url"`
	// 抓取窗口最长等待毫秒（沿用 cpp 的 timeout 语义；默认 10 分钟）。
	Timeout int64 `json:"timeout"`
	// 只保留这些 template_id 的标记（滑索架 id）；为空表示不过滤。
	TemplateIDs []string `json:"template_ids"`
	// firefox 可执行文件路径；为空用默认值 "firefox"。
	Firefox string `json:"firefox"`
	// 本地 MITM 代理监听端口；为空用随机空闲端口。
	ProxyPort int `json:"proxy_port"`
}

const (
	defaultMapURL  = "https://game.skland.com/map/endfield"
	defaultFirefox = "firefox"
	// 与 cpp 的 kDefaultTimeoutMs 一致：10 分钟。
	defaultTimeout = 10 * 60 * 1000
)

func parseParam(raw string) (actionParam, error) {
	p := actionParam{
		URL:     defaultMapURL,
		Firefox: defaultFirefox,
		Timeout: defaultTimeout,
	}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return actionParam{}, err
		}
	}
	if p.URL == "" {
		p.URL = defaultMapURL
	}
	if p.Firefox == "" {
		p.Firefox = defaultFirefox
	}
	if p.Timeout <= 0 {
		p.Timeout = defaultTimeout
	}
	return p, nil
}

// isGlobalRegionHost 判断页面主机是否属于国际服 SKPORT（skport.com 及其子域）。
// Linux 的 MITM 解密白名单只含国服 API 域名，国际服流量经 PAC 直连、代理全程不可见，
// 与其让用户干等满超时，不如在入口处快速失败并提示切回 CN。
func isGlobalRegionHost(host string) bool {
	host = strings.ToLower(host)
	return host == "skport.com" || strings.HasSuffix(host, ".skport.com")
}
