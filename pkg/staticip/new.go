package staticip

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	crdClientset "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned"
	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

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
	sip.Spec.Avaliable, sip.Spec.IPMap = initIPMap(sip.Spec.IPPool)

	return sip
}

// initIPMap 创建 IP 与 Pod 的映射表.
// 参数 IPsStr 为以逗号分隔的点分十进制IP字符串, 如 "192.168.0.1/24,192.168.0.2/24"
func initIPMap(IPsStr string) (ipList []string, ipMap map[string]*ipkv1.OwnerPod) {
	ipMap = map[string]*ipkv1.OwnerPod{}
	ipList = []string{}
	for _, v := range strings.Split(IPsStr, ",") {
		ipMap[v] = nil
		ipList = append(ipList, v)
	}
	return
}

// GetStaticIP 获取属于目标 owner 对象的 StaticIP 资源对象.
// 其中 owner 可能是 Pod/Deployment 等.
func GetStaticIP(
	crdClient crdClientset.Interface,
	owner apimmetav1.Object,
	ownerKind string,
) (sip *ipkv1.StaticIP, err error) {
	ownerName := owner.GetName()
	ownerNS := owner.GetNamespace()

	sipName := GenerateSIPName(ownerKind, ownerName)

	return crdClient.IpkeeperV1().StaticIPs(ownerNS).Get(sipName, metav1.GetOptions{})
}
