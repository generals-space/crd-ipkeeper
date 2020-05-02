package server

import (
	"flag"

	"github.com/spf13/pflag"
)

// Configuration ...
type Configuration struct {
	BindSocket     string
	KubeConfigFile string
}

// ParseFlags ...
// TODO: validate configuration
func ParseFlags() *Configuration {
	var (
		argBindSocket     = pflag.String("bind-socket", "/var/run/cniserver.sock", "The socket daemon bind to.")
		argKubeConfigFile = pflag.String("kubeconfig", "", "Path to kubeconfig file with authorization and master location information. If not set use the inCluster token.")
	)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	// Init for glog calls in kubernetes packages
	flag.CommandLine.Parse(make([]string, 0))

	return &Configuration{
		BindSocket:     *argBindSocket,
		KubeConfigFile: *argKubeConfigFile,
	}
}
