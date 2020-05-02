package controller

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgworkqueue "k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
)

func newSIP(ownerKind string, obj interface{}) (err error) {
	deploy := &appsv1.Deployment{}
	if ownerKind == "deploy" {

	}

	////////////////////////////
	sipOwnerKind := "deploy"
	sipName := fmt.Sprintf("%s-%s-%s", deploy.Namespace, sipOwnerKind, deploy.Name)
	sip := &ipkv1.StaticIP{
		ObjectMeta: apimmetav1.ObjectMeta{
			Name:      sipName,
			Namespace: deploy.Namespace,
			OwnerReferences: []apimmetav1.OwnerReference{
				// NewControllerRef() 第1个参数为所属对象 owner,
				// 第2个参数为 owner 的 gvk 信息对象.
				*apimmetav1.NewControllerRef(
					deploy, deploy.GroupVersionKind(),
				),
			},
		},
		Spec: ipkv1.StaticIPSpec{
			Namespace: deploy.Namespace,
			OwnerKind: sipOwnerKind,
		},
	}
	fmt.Printf("sip %+v\n", sip)
	return
}

// initIPMap 创建 IP 与 Pod 的映射表
func (c *Controller) initIPMap(ipPoolAnnotation string) (ipMap map[string]*ipkv1.OwnerPod) {
	ipMap = map[string]*ipkv1.OwnerPod{}
	for _, v := range strings.Split(ipPoolAnnotation, ",") {
		ipMap[v] = nil
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
