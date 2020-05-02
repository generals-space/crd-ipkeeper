package main

import (
	// "path/filepath"
	// "k8s.io/client-go/util/homedir"

	"flag"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/klog"

	crdClientset "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned"
	"github.com/generals-space/crd-ipkeeper/pkg/controller"
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
	flag.Set("v", "5")
	flag.Parse()
	klog.SetOutput(os.Stdout)
	defer klog.Flush()

	config := server.ParseFlags()

	// home := homedir.HomeDir()
	// kubeConfigPath := filepath.Join(home, ".kube", "config")
	// kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	// kubeConfigPath 为 "" 时, 会自动调用 InClusterConfig() 去获取挂载到 Pod 内部中的配置文件
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		klog.Errorf("failed to get kube config: %v", err)
		os.Exit(1)
	}

	stopCh := signals.SetupSignalHandler(stopHandler, config.BindSocket)

	kubeClient := kubernetes.NewForConfigOrDie(kubeConfig)
	crdClient := crdClientset.NewForConfigOrDie(kubeConfig)
	c, err := controller.NewController(kubeClient, crdClient)
	go c.Run(stopCh)

	cniServer := server.NewCNIServer(config, kubeClient, crdClient)
	cniServer.Run()
}
