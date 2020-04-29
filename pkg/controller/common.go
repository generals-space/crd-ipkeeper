package controller

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
)

func newSIP(ownerKind string, obj interface{}) (err error) {
	deploy := &appsv1.Deployment{}
	if ownerKind == "deploy" {

	}

	////////////////////////////
	sipOwnerKind := "deploy"
	sipName := fmt.Sprintf("%s-%s-%s", deploy.Namespace, sipOwnerKind, deploy.Name)
	sip := &ipkv1.StaticIPs{
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
		Spec: ipkv1.StaticIPsSpec{
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
