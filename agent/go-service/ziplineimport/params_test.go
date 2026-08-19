//go:build linux

package ziplineimport

import "testing"

// SKPORT 快速失败判定：国际服主机及其子域命中；形近的国服域名与钓鱼域名不得误伤。
func TestIsGlobalRegionHost(t *testing.T) {
	cases := map[string]bool{
		"game.skport.com":     true,
		"skport.com":          true,
		"api.skport.com":      true,
		"GAME.SKPORT.COM":     true,
		"game.skland.com":     false,
		"zonai.skland.com":    false,
		"evil-skport.com":     false,
		"skport.com.evil.tld": false,
		"":                    false,
	}
	for host, want := range cases {
		if got := isGlobalRegionHost(host); got != want {
			t.Fatalf("isGlobalRegionHost(%q) = %v, want %v", host, got, want)
		}
	}
}
