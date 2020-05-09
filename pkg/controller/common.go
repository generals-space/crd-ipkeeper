package controller

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimerrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgcache "k8s.io/client-go/tools/cache"
	cgworkqueue "k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

// getDeployFromKey 从 Lister 成员中取得指定 ns/name 的 deploy 对象.
// 另外还有对参数 key 的解析, 对获得的 deploy 对象是否拥有 ippool 等字段的判断等操作.
// key 的格式如 kube-system/devops-deploy
// caller: c.handleAddDeploy(), c.handleDelDeploy()
func (c *Controller) getDeployFromKey(key string) (deploy *appsv1.Deployment, err error) {
	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cgcache.SplitMetaNamespaceKey(key)
	if err != nil {
		err = fmt.Errorf("invalid resource key: %s", key)
		return
	}

	// 从 Lister 缓存中取对象
	deploy, err = c.deployLister.Deployments(ns).Get(name)
	if err != nil {
		// 如果 deploy 对象被删除了, 就会走到这里, 所以应该在这里加入执行
		if apimerrors.IsNotFound(err) {
			klog.Infof("deploy doesn't exist: %s/%s ...", ns, name)
			return
		}
		err = fmt.Errorf("failed to list deploy by: %s/%s", ns, name)
		return
	}

	// 能加入到 addDeployQueue 都是已经经过 enqueueAddDeploy() 方法筛选过的,
	// 但还是要检查一遍
	if deploy.Annotations[util.IPPoolAnnotation] == "" &&
		deploy.Annotations[util.GatewayAnnotation] == "" {
		klog.Fatal("deploy doesn't exist: %s/%s ...", ns, name)
		return nil, nil
	}
	return
}

// getDeployFromKey 从 Lister 成员中取得指定 ns/name 的 pod 对象.
// 具体操作基本等同于 c.getDeployFromKey()
func (c *Controller) getPodFromKey(key string) (pod *corev1.Pod, err error) {
	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cgcache.SplitMetaNamespaceKey(key)
	if err != nil {
		err = fmt.Errorf("invalid resource key: %s", key)
		return
	}

	// 从 Lister 缓存中取对象
	pod, err = c.podLister.Pods(ns).Get(name)
	if err != nil {
		// 如果 pod 对象被删除了, 就会走到这里, 所以应该在这里加入执行
		if apimerrors.IsNotFound(err) {
			klog.Infof("pod doesn't exist: %s/%s ...", ns, name)
			return
		}
		err = fmt.Errorf("failed to list pod by: %s/%s", ns, name)
		return
	}

	return
}

// processNextWorkItem 调用 handler 处理具体的事件,
// 并根据其结果对 queue 调用 Done() 和 Forget(), 表示该 obj 事件已经处理成功或失败.
// caller: c.processNextAddDeployWorkItem(), c.processNextDelDeployWorkItem()
// 这个函数是从 ta 们中抽离出来的, 原本的形式可以到 c.processNextAddDeployWorkItem() 函数体中查看.
func (c *Controller) processNextWorkItem(
	obj interface{},
	queue cgworkqueue.RateLimitingInterface,
	handler func(key string) (err error),
) (err error) {
	defer queue.Done(obj)
	var key string
	var ok bool

	// key 的格式如 kube-system/devops-deploy
	if key, ok = obj.(string); !ok {
		queue.Forget(obj)
		utilruntime.HandleError(fmt.Errorf("expected string in queue but got %#v", obj))
		return nil
	}
	err = handler(key)
	if err != nil {
		return fmt.Errorf("error syncing '%s': %s", key, err.Error())
	}

	queue.Forget(obj)
	klog.Infof("Successfully synced '%s'", key)
	return nil
}
