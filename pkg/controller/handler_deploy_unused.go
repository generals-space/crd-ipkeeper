package controller

import (
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog"
)

func (c *Controller) runDelDeployWorker() {
	for c.processNextDelDeployWorkItem() {
	}
}

func (c *Controller) processNextDelDeployWorkItem() bool {
	var err error
	obj, shutdown := c.delDeployQueue.Get()
	if shutdown {
		return false
	}
	err = c.processNextWorkItem(obj, c.delDeployQueue, c.handleDelDeploy)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) handleDelDeploy(key string) (err error) {
	deploy, err := c.getDeployFromKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	// 当 deploy 中不含 ippool 等字段时即为 nil, 也不需要处理.
	if deploy == nil {
		return nil
	}
	sipName := GenerateSIPName("Deployment", deploy.Name)
	klog.Infof("try to delete sip: %s", sipName)

	delOpt := &apimmetav1.DeleteOptions{}
	err = c.crdClient.IpkeeperV1().StaticIPs(deploy.Namespace).Delete(sipName, delOpt)
	if err != nil {
		klog.Fatal("failed to delete sip for deploy %s: %s", deploy.Name, err)
		utilruntime.HandleError(err)
		return err
	}
	klog.Infof("success to delete sip object: %+v", sipName)
	return
}
