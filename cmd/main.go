package main

import (
	"os"

	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/server"
	"github.com/generals-space/crd-ipkeeper/pkg/signals"
	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

// stopHandler 收到退出信号清理 cniserver.sock 文件.
func stopHandler(sockPath string) {
	if util.Exists(sockPath) {
		err := os.Remove(sockPath)
		if err != nil {
			klog.Errorf("failed to remove cniserver sock file: %s", err)
		}
	}
	return
}

func main() {
	klog.SetOutput(os.Stdout)
	defer klog.Flush()

	config, err := server.ParseFlags()
	if err != nil {
		klog.Errorf("parse config failed %v", err)
		os.Exit(1)
	}

	signals.SetupSignalHandler(stopHandler, config.BindSocket)
	server.NewController(config)
	server.RunServer(config)
}
