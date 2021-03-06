package controller

import (
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
)

//////////////////////////////////////////////////////////////
// enqueue 前期操作
func (c *Controller) enqueueAddPod(obj interface{}) {
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

	pod := obj.(*corev1.Pod)
	// 查询是否存在当前 Pod 对应的 StaticIP 对象
	_, err = c.sipHelper.GetPodOwnerSIP(pod)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.addPodQueue.AddRateLimited(key)
	return
}

func (c *Controller) enqueueDelPod(obj interface{}) {
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
	pod := obj.(*corev1.Pod)
	// 直属于 Pod 的 StaticIP 资源会随 Pod 一同被移除, 不需要额外操作.
	if pod.OwnerReferences == nil {
		return
	}
	// 查询是否存在当前 Pod 对应的 StaticIP 对象
	_, err = c.sipHelper.GetPodOwnerSIP(pod)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	c.delPodQueue.AddRateLimited(key)
	return
}

//////////////////////////////////////////////////////////////
// process 实际操作 Add 部分
func (c *Controller) runAddPodWorker() {
	// 注意这里for的用法.
	// 传统的用法可形容为: for init; condition; post { }
	// 另外 for {} 作为while语句使用,
	// 但还有一种, for condition { }, 这里用的就是第三种形式.
	// 当 condition 返回 false 时结束, 这也表示 workqueue 被关闭了.
	for c.processNextAddPodWorkItem() {
	}
}

func (c *Controller) processNextAddPodWorkItem() bool {
	var err error
	obj, shutdown := c.addPodQueue.Get()
	if shutdown {
		return false
	}
	err = c.processNextWorkItem(obj, c.addPodQueue, c.handleAddPod)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) handleAddPod(key string) (err error) {
	pod, err := c.getPodFromKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	// 当 pod 中不含 ipaddr 等字段时即为 nil, 也不需要处理.
	if pod == nil {
		return nil
	}
	var sip *ipkv1.StaticIP
	// 查询是否存在当前 Pod 对应的 StaticIP 对象
	sip, err = c.sipHelper.GetPodOwnerSIP(pod)
	if err != nil {
		klog.Warning("failed to find static ip for pod %s: %s", pod.Name, err)
		return nil
	}
	// 当 Pod 没有 Owner 时, err 为 nil, sip 也为 nil, 此时需要为其创建对应 StaticIP 对象.
	if sip == nil {
		return c.sipHelper.CreateStaticIP(pod, "Pod")
	}

	return
}

//////////////////////////////////////////////////////////////
// process 实际操作 Del 部分
func (c *Controller) runDelPodWorker() {
	// 注意这里for的用法.
	// 传统的用法可形容为: for init; condition; post { }
	// 另外 for {} 作为while语句使用,
	// 但还有一种, for condition { }, 这里用的就是第三种形式.
	// 当 condition 返回 false 时结束, 这也表示 workqueue 被关闭了.
	for c.processNextDelPodWorkItem() {
	}
}

func (c *Controller) processNextDelPodWorkItem() bool {
	var err error
	obj, shutdown := c.delPodQueue.Get()
	if shutdown {
		return false
	}
	err = c.processNextWorkItem(obj, c.delPodQueue, c.handleDelPod)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) handleDelPod(key string) (err error) {
	pod, err := c.getPodFromKey(key)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	// 当 pod 中不含 ipaddr 等字段时即为 nil, 也不需要处理.
	if pod == nil {
		return nil
	}
	// 查询是否存在当前 Pod 对应的 StaticIP 对象
	sip, err := c.sipHelper.GetPodOwnerSIP(pod)
	if err != nil {
		klog.Warning("failed to find static ip for pod %s: %s", pod.Name, err)
		return nil
	}

	////////////////////////////////////////////////////////////////////
	return c.sipHelper.ReleaseIP(sip, pod)
}
