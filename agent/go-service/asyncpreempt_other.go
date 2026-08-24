//go:build !windows

package main

import (
	"os"
	"syscall"

	"github.com/rs/zerolog/log"
)

// disableAsyncPreempt 关闭 Go 运行时基于 SIGURG 的异步抢占。
// MAA 原生库通过 zmq 与本进程做阻塞式 IPC；抢占信号打断阻塞系统调用后，
// zmq 会以未捕获的 C++ 异常终止整个进程（SIGABRT），而不是让当前调用失败并走正常错误处理。
// GODEBUG 只在运行时启动时读一次，main 里 os.Setenv 已经来不及生效，
// 因此这里用同一份可执行文件重新 exec 自己，让新进程从头读取带该开关的环境变量。
func disableAsyncPreempt() {
	env, changed := asyncPreemptOffEnv(os.Environ())
	if !changed {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if err := syscall.Exec(exe, os.Args, env); err != nil {
		log.Warn().
			Err(err).
			Msg("Failed to re-exec go-service with asyncpreemptoff")
	}
}
