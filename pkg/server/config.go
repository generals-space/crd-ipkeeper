package server

import (
	"flag"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/kubeclient"
)

// Configuration ...
type Configuration struct {
	BindSocket     string
	KubeConfigFile string
	KubeClient     kubernetes.Interface
}

// ParseFlags ...
// TODO: validate configuration
func ParseFlags() (*Configuration, error) {
	var (
		argBindSocket     = pflag.String("bind-socket", "/var/run/cniserver.sock", "The socket daemon bind to.")
		argKubeConfigFile = pflag.String("kubeconfig", "", "Path to kubeconfig file with authorization and master location information. If not set use the inCluster token.")
	)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	// Init for glog calls in kubernetes packages
	flag.CommandLine.Parse(make([]string, 0))

	config := &Configuration{
		BindSocket:     *argBindSocket,
		KubeConfigFile: *argKubeConfigFile,
	}
	// 这里注意结构体成员不能与`:=`一起使用, 需要预告声明err变量, 
	// 否则会出现`non-name config.KubeClient on left side of :=`的报错.
	var err error
	config.KubeClient, err = kubeclient.NewKubeClient(config.KubeConfigFile)
	if err != nil {
		return nil, err
	}
	klog.Infof("bind socket: %s", config.BindSocket)
	return config, nil
}
