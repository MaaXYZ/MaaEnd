//go:build windows

package main

// disableAsyncPreempt 在 Windows 下是空操作：Windows 的 goroutine 抢占不依赖信号，
// 不会打断阻塞系统调用，因此不存在 zmq 收到 EINTR 而崩溃的场景。
func disableAsyncPreempt() {}
