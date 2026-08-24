package main

import "strings"

// asyncPreemptOffEnv 返回把 GODEBUG=asyncpreemptoff=1 合并进 env 后的新环境变量列表。
// changed 为 false 表示 env 里已经关闭了异步抢占，调用方无需再做任何事。
func asyncPreemptOffEnv(env []string) (result []string, changed bool) {
	for i, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || name != "GODEBUG" {
			continue
		}
		for _, setting := range strings.Split(value, ",") {
			if setting == "asyncpreemptoff=1" {
				return env, false
			}
		}
		result = append([]string{}, env...)
		if value == "" {
			result[i] = "GODEBUG=asyncpreemptoff=1"
		} else {
			result[i] = "GODEBUG=" + value + ",asyncpreemptoff=1"
		}
		return result, true
	}
	result = append(append([]string{}, env...), "GODEBUG=asyncpreemptoff=1")
	return result, true
}
