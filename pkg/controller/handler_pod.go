package controller

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	"github.com/generals-space/crd-ipkeeper/pkg/staticip"
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
	_, err = staticip.GetPodOwnerSIP(c.kubeClient, c.crdClient, pod)
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
	_, err = staticip.GetPodOwnerSIP(c.kubeClient, c.crdClient, pod)
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
	sip, err = staticip.GetPodOwnerSIP(c.kubeClient, c.crdClient, pod)
	if err != nil {
		klog.Warning("failed to find static ip for pod %s: %s", pod.Name, err)
		return nil
	}

	////////////////////////////////////////////////////////////////////
	// 在触发 AddFunc() 回调时, Pod 还处理 Pending 状态, Status 字段中并没有有效信息,
	// 所以无法直接使用 pod.Status.PodIP 获取该 Pod 的实际 IP.
	// 不只如此, 连 Metadata 部分大多也是空的, 无法通过 UID 与 StaticIP 关联.
	// 这里一直循环, 直到 pod 信息完整(sip 信息也会被补全)
	// 之后考虑使用 Watch 操作, 开单独的协程来完成这个操作.
	var flag bool
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second * 2)
		// 查询是否存在当前 Pod 对应的 StaticIP 对象
		sip, err := staticip.GetPodOwnerSIP(c.kubeClient, c.crdClient, pod)
		if err != nil {
			klog.Warning("failed to find static ip for pod %s: %s", pod.Name, err)
			return nil
		}
		// sip 初建时 Generation 为 1, 在 ipam 时期会被修改.
		if sip.Spec.OwnerKind == "Deployment" && sip.Generation > 1 {
			flag = true
			break
		}
	}
	if !flag {
		klog.Warningf("the pod %s doesn't get an ip at last", pod.Name)
		return
	}
	// staticip.ReallocIP(sip, pod, "add")

	_, err = c.crdClient.IpkeeperV1().StaticIPs(sip.Namespace).Update(sip)
	if err != nil {
		klog.Errorf("failed to occupy one IP from sip: %s", sip.Name)
		return
	}
	return
}

//////////////////////////////////////////////////////////////
// process 实际操作 Del 部分
func (c *Controller) runDelPodWorker() {
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
	sip, err := staticip.GetPodOwnerSIP(c.kubeClient, c.crdClient, pod)
	if err != nil {
		klog.Warning("failed to find static ip for pod %s: %s", pod.Name, err)
		return nil
	}

	////////////////////////////////////////////////////////////////////
	return staticip.ReleaseIP(c.crdClient, sip, pod)
}
