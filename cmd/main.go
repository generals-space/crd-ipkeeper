package main

import (
	"os"

	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/server"
	"github.com/generals-space/crd-ipkeeper/pkg/signals"
)

func main() {
	klog.SetOutput(os.Stdout)
	defer klog.Flush()

	config, err := server.ParseFlags()
	if err != nil {
		klog.Errorf("parse config failed %v", err)
		os.Exit(1)
	}

	signals.SetupSignalHandler()
	server.NewController(config)
	server.RunServer(config)
}
