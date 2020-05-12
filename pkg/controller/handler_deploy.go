package controller

import (
	appsv1 "k8s.io/api/apps/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/staticip"
	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

//////////////////////////////////////////////////////////////
// enqueue 前期操作
func (c *Controller) enqueueAddDeploy(obj interface{}) {
	if !c.isLeader() {
		return
	}
	var key string
	var err error
	// 将该对象(应该是将该对象的事件)放入 cache 缓存, 即在本地保存 deploy 资源列表,
	// 之前先从 cache 取数据, 以减轻 apiserver 的压力.
	// key 的格式如 kube-system/devops-deploy
	key, err = cgcache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	deploy := obj.(*appsv1.Deployment)
	if deploy.Annotations[util.IPPoolAnnotation] != "" &&
		deploy.Annotations[util.GatewayAnnotation] != "" {
		klog.Infof("enqueue add ip pool deploy %s", key)
		c.addDeployQueue.AddRateLimited(key)
	}
	return
}

// enqueueUpdateDeploy ...
// 貌似在发生 Add 事件时也会同时触发 Update 事件而调用此函数.
func (c *Controller) enqueueUpdateDeploy(oldObj, newObj interface{}) {
	if !c.isLeader() {
		return
	}

	oldD := oldObj.(*appsv1.Deployment)
	newD := newObj.(*appsv1.Deployment)

	if oldD.ResourceVersion == newD.ResourceVersion {
		// 这种情况一般是定时的 update, 并非由于资源对象发生变动而触发.
		return
	}

	var key string
	var err error
	// oldObj 与 newObj 得到的 key 是相同的.
	key, err = cgcache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	prev := oldD.Annotations[util.IPPoolAnnotation] != "" &&
		oldD.Annotations[util.GatewayAnnotation] != ""
	next := newD.Annotations[util.IPPoolAnnotation] != "" &&
		newD.Annotations[util.GatewayAnnotation] != ""

	if !prev && next {
		// 1. IPPool 注解从无到有: 要走的是 Add 流程
		// c.addDeployQueue.AddRateLimited(newKey)
	} else if prev && !next {
		// 2. IPPool 注解从有到无: 要走的是 Del 流程
		// c.delDeployQueue.AddRateLimited(newKey)
	} else if prev && next {
		// 3. IPPool 发生变化: StaticIP 资源不变, 内容需要进行修改
		if oldD.Annotations[util.IPPoolAnnotation] == newD.Annotations[util.IPPoolAnnotation] &&
			oldD.Annotations[util.GatewayAnnotation] == newD.Annotations[util.GatewayAnnotation] {
			// 如果 IPPool 和 Gateway 注解值未发生变动, 则无需操作
			return
		}
		klog.Infof("enqueue add ip pool deploy %s", key)
		c.updateDeployQueue.AddRateLimited(key)
	} else {
		// 其他的不管
	}
	return
}

//////////////////////////////////////////////////////////////
// process 实际操作 Add 部分
func (c *Controller) runAddDeployWorker() {
	// 注意这里for的用法.
	// 传统的用法可形容为: for init; condition; post { }
	// 另外 for {} 作为while语句使用,
	// 但还有一种, for condition { }, 这里用的就是第三种形式.
	// 当 condition 返回 false 时结束, 这也表示 workqueue 被关闭了.
	for c.processNextAddDeployWorkItem() {
	}
}

func (c *Controller) processNextAddDeployWorkItem() bool {
	var err error
	obj, shutdown := c.addDeployQueue.Get()
	if shutdown {
		return false
	}
	/*
		// We wrap this block in a func so we can defer c.addDeployQueue.Done.
		// 把下面的操作包裹在了一个 func 中, 主要就是为了在函数结束时调用这个 defer
		// 其实完全可以不需要用函数形式的.
		err := func(obj interface{}) error {
			defer c.addDeployQueue.Done(obj)
			var key string
			var ok bool
			// key 的格式如 kube-system/devops-deploy
			if key, ok = obj.(string); !ok {
				c.addDeployQueue.Forget(obj)
				utilruntime.HandleError(fmt.Errorf("expected string in addDeployQueue but got %#v", obj))
				return nil
			}
			// 在 handleAddDeploy 中处理 deploy 的新增事件.
			if err := c.handleAddDeploy(key); err != nil {
				return fmt.Errorf("error syncing '%s': %s", key, err.Error())
			}
			c.addDeployQueue.Forget(obj)
			klog.Infof("Successfully synced '%s'", key)
			return nil
		}(obj)
	*/
	err = c.processNextWorkItem(obj, c.addDeployQueue, c.handleAddDeploy)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) handleAddDeploy(key string) (err error) {
	deploy, err := c.getDeployFromKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	// 当 deploy 中不含 ippool 等字段时即为 nil, 也不需要处理.
	if deploy == nil {
		return nil
	}

	return c.sipHelper.CreateStaticIP(deploy, "Deployment")
}

//////////////////////////////////////////////////////////////
// process 实际操作 Update 部分
func (c *Controller) runUpdateDeployWorker() {
	// 注意这里for的用法.
	// 传统的用法可形容为: for init; condition; post { }
	// 另外 for {} 作为while语句使用,
	// 但还有一种, for condition { }, 这里用的就是第三种形式.
	// 当 condition 返回 false 时结束, 这也表示 workqueue 被关闭了.
	for c.processNextUpdateDeployWorkItem() {
	}
}

func (c *Controller) processNextUpdateDeployWorkItem() bool {
	var err error
	obj, shutdown := c.updateDeployQueue.Get()
	if shutdown {
		return false
	}
	err = c.processNextWorkItem(obj, c.updateDeployQueue, c.handleUpdateDeploy)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) handleUpdateDeploy(key string) (err error) {
	deploy, err := c.getDeployFromKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	// 当 deploy 中不含 ippool 等字段时即为 nil, 也不需要处理.
	if deploy == nil {
		return nil
	}

	oldSIP, err := c.sipHelper.GetStaticIP(deploy, "Deployment")
	newSIP := c.sipHelper.NewStaticIP(deploy, "Deployment")

	return staticip.RenewStaticIP(c.kubeClient, c.crdClient, oldSIP, newSIP)
}
