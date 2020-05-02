package controller

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apimerrors "k8s.io/apimachinery/pkg/api/errors"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
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

//////////////////////////////////////////////////////////////

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
func (c *Controller) enqueueUpdateDeploy(oldObj, newObj interface{}) {

}
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

//////////////////////////////////////////////////////////////

func (c *Controller) runAddDeployWorker() {
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

	// 尝试使用 newSIP() 函数构造 SIP 对象, 不过需要用到反射.
	sipOwnerKind := "deploy"
	sipName := fmt.Sprintf("%s-%s-%s", deploy.Namespace, sipOwnerKind, deploy.Name)

	getOpt := &apimmetav1.GetOptions{}
	_, err = c.crdClient.IpkeeperV1().StaticIPs(deploy.Namespace).Get(sipName, *getOpt)
	if err == nil {
		klog.Infof("sip %s already exist, return", sipName)
		return
	}
	klog.Infof("try to create new sip: %s", sipName)
	sip := &ipkv1.StaticIP{
		ObjectMeta: apimmetav1.ObjectMeta{
			Name:      sipName,
			Namespace: deploy.Namespace,
			OwnerReferences: []apimmetav1.OwnerReference{
				// NewControllerRef() 第1个参数为所属对象 owner,
				// 第2个参数为 owner 的 gvk 信息对象.
				*apimmetav1.NewControllerRef(
					deploy,
					// deploy.GroupVersionKind() 的打印结果为 "/, Kind=" (不是字符串类型)
					// 而 WithKind("Deployment") 的打印结果为 "apps/v1, Kind=Deployment"
					appsv1.SchemeGroupVersion.WithKind("Deployment"),
				),
			},
		},
		Spec: ipkv1.StaticIPSpec{
			Namespace: deploy.Namespace,
			OwnerKind: sipOwnerKind,
			IPPool:    deploy.Annotations[util.IPPoolAnnotation],
			Gateway:   deploy.Annotations[util.GatewayAnnotation],
			IPMap:     c.initIPMap(deploy.Annotations[util.IPPoolAnnotation]),
		},
	}
	klog.V(3).Infof("new sip ojbect: %+v", sip)
	actualSIP, err := c.crdClient.IpkeeperV1().StaticIPs(deploy.Namespace).Create(sip)
	if err != nil {
		// if err.Error() == "already exists" {}
		klog.Fatalf("failed to create new sip for deploy %s: %s", deploy.Name, err)
		utilruntime.HandleError(err)
		return err
	}
	klog.Infof("success to create new sip object: %+v", actualSIP)
	return
}

//////////////////////////////////////////////////////////////

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
	sipOwnerKind := "deploy"
	sipName := fmt.Sprintf("%s-%s-%s", deploy.Namespace, sipOwnerKind, deploy.Name)
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
