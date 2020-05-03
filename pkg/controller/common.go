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
	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

// GenerateSIPName ...
// @param ownerKind: Pod, Deployment, Daemonset
func GenerateSIPName(ownerKind, ownerName string) (name string) {
	var ownerShortKind string
	if ownerKind == "Deployment" {
		ownerShortKind = "deploy"
	} else if ownerKind == "Pod" {
		ownerShortKind = "pod"
	}
	return fmt.Sprintf("%s-%s", ownerShortKind, ownerName)
}

// NewStaticIP 根据传入的 owner 资源创建 StaticIP 对象.
// owner Deployment, Pod 等对象.
// ownerKind 目前没能找到通过 owner 获取 ownerKind 的方法, 暂时显式传入此参数.
func NewStaticIP(owner apimmetav1.Object, ownerKind string) (sip *ipkv1.StaticIP) {
	ownerName := owner.GetName()
	ownerNS := owner.GetNamespace()
	ownerAnno := owner.GetAnnotations()

	////////////////////////////
	sipName := GenerateSIPName(ownerKind, ownerName)
	sip = &ipkv1.StaticIP{
		ObjectMeta: apimmetav1.ObjectMeta{
			Name:      sipName,
			Namespace: ownerNS,
			OwnerReferences: []apimmetav1.OwnerReference{
				// NewControllerRef() 第1个参数为所属对象 owner,
				// 第2个参数为 owner 的 gvk 信息对象.
				*apimmetav1.NewControllerRef(
					owner,
					// deploy.GroupVersionKind() 的打印结果为 "/, Kind=" (不是字符串类型)
					// 而 xxx.WithKind("Deployment") 的打印结果为 "apps/v1, Kind=Deployment"
					appsv1.SchemeGroupVersion.WithKind(ownerKind),
				),
			},
		},
		Spec: ipkv1.StaticIPSpec{
			Namespace: ownerNS,
			OwnerKind: ownerKind,
			Gateway:   ownerAnno[util.GatewayAnnotation],
		},
	}
	if ownerKind == "Deployment" {
		sip.Spec.IPPool = ownerAnno[util.IPPoolAnnotation]
	} else if ownerKind == "Pod" {
		sip.Spec.IPPool = ownerAnno[util.IPAddressAnnotation]
	}
	sip.Spec.IPMap = initIPMap(sip.Spec.IPPool)

	return sip
}

// initIPMap 创建 IP 与 Pod 的映射表.
// 参数 IPsStr 为以逗号分隔的点分十进制IP字符串, 如 "192.168.0.1/24,192.168.0.2/24"
func initIPMap(IPsStr string) (ipMap map[string]*ipkv1.OwnerPod) {
	ipMap = map[string]*ipkv1.OwnerPod{}
	for _, v := range strings.Split(IPsStr, ",") {
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
