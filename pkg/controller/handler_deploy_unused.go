package controller

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgcache "k8s.io/client-go/tools/cache"
)

func (c *Controller) enqueueDelDeploy(obj interface{}) {
	if !c.isLeader() {
		return
	}
	var key string
	var err error
	key, err = cgcache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	// deploy := obj.(*appsv1.Deployment)
	c.delDeployQueue.AddRateLimited(key)
}

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

	return
}
