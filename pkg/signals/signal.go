package signals

import (
	"os"
	"os/signal"
	"syscall"
)

var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// SetupSignalHandler 设置信号处理机制, 但不涉及具体的处理操作.
// 具体的处理操作需要通过 fn 参数传入.
func SetupSignalHandler(fn func(string), arg string) {
	// sigCh接收信号, 注意之后的清理操作有可能失败, 失败后不能直接退出.
	sigCh := make(chan os.Signal, 1)
	// 一般delete pod时, 收到的是SIGTERM信号.
	signal.Notify(sigCh, shutdownSignals...)

	go func() {
		<-sigCh
		// 调用fn(), 执行真正的清理工作.
		fn(arg)
		os.Exit(1)
	}()
}
