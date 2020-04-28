package signals

import (
	"os"
	"os/signal"
	"syscall"
)

var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// SetupSignalHandler 设置信号处理机制, 但不涉及具体的处理操作.
// 具体的处理操作需要通过 fn 参数传入.
func SetupSignalHandler(fn func(string), arg string) (<-chan struct{}) {
	// 几乎所有 informer 相关的方法都需要一个 struct 类型的 chan 为参数,
	// 做为结束通知的通道.
	stopCh := make(chan struct{})
	// sigCh接收信号, 注意之后的清理操作有可能失败, 失败后不能直接退出.
	sigCh := make(chan os.Signal, 1)
	// 一般 delete pod 时, 收到的是 SIGTERM 信号.
	signal.Notify(sigCh, shutdownSignals...)

	go func() {
		<-sigCh
		// 注意: 由于 close 不接受确定的只读类型通道, 所以不能在返回值列表中指定 stopCh,
		// 否则这里会提示错误.
		close(stopCh)
		// 调用fn(), 执行真正的清理工作.
		// 移除 .sock, .pid 等等
		fn(arg)
		os.Exit(1)
	}()

	return stopCh
}
