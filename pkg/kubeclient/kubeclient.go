package kubeclient

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

// NewKubeClient 初始化 kube client 并返回.
// 如果传入的kube config文件不存在, 则尝试使用InClusterConfig()的方式来构建.
// client-go工程中貌似没有与此函数有类似功能的函数, 这里也只是做一个封装.
func NewKubeClient(kubeConfigFilePath string) (client kubernetes.Interface, err error) {
	var cfg *rest.Config
	if kubeConfigFilePath == "" {
		klog.Infof("no --kubeconfig, use in-cluster kubernetes config")
		cfg, err = rest.InClusterConfig()
		if err != nil {
			klog.Errorf("use in cluster config failed %v", err)
			return nil, err
		}
	} else {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
		if err != nil {
			klog.Errorf("use --kubeconfig %s failed %v", kubeConfigFilePath, err)
			return nil, err
		}
	}
	client, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Errorf("init kubernetes client failed %v", err)
		return nil, err
	}
	return
}
