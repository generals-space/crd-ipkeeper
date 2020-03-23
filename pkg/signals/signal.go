package signals

import (
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

var shutdownSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// SetupSignalHandler 设置信号处理机制, 但不涉及具体的处理操作.
// 具体的处理操作需要通过 fn 参数传入.
func SetupSignalHandler() {
	// sigCh接收信号, 注意之后的清理操作有可能失败, 失败后不能直接退出.
	sigCh := make(chan os.Signal, 1)
	// 一般delete pod时, 收到的是SIGTERM信号.
	signal.Notify(sigCh, shutdownSignals...)
	var sockPath = "/var/run/cniserver.sock"

	go func() {
		var err error

		<-sigCh

		if util.Exists(sockPath) {
			err = os.Remove(sockPath)
			if err != nil {
				klog.Errorf("failed to remove cniserver sock file: %s", err)
			}
		}

		os.Exit(1)
	}()
}
