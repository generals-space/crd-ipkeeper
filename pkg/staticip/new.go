package staticip

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgkuber "k8s.io/client-go/kubernetes"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	crdClientset "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned"
	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

// Helper StaticIP 资源对象相关操作的函数合集.
type Helper struct {
	kubeClient cgkuber.Interface
	crdClient  crdClientset.Interface
}

// New ...
func New(
	kubeClient cgkuber.Interface,
	crdClient crdClientset.Interface,
) (helper *Helper) {
	return &Helper{
		kubeClient: kubeClient,
		crdClient:  crdClient,
	}
}

// generateSIPName ...
// @param ownerKind: Pod, Deployment, Daemonset
func (h *Helper) generateSIPName(ownerKind, ownerName string) (name string) {
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
func (h *Helper) NewStaticIP(owner apimmetav1.Object, ownerKind string) (sip *ipkv1.StaticIP) {
	ownerName := owner.GetName()
	ownerNS := owner.GetNamespace()
	ownerAnno := owner.GetAnnotations()

	////////////////////////////
	sipName := h.generateSIPName(ownerKind, ownerName)
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
	sip.Spec.Avaliable, sip.Spec.IPMap = h.InitIPMap(sip.Spec.IPPool)
	sip.Spec.Used = []string{}
	sip.Spec.Ratio = fmt.Sprintf("%d/%d", len(sip.Spec.Used), len(sip.Spec.IPMap))

	return sip
}

// InitIPMap 创建 IP 与 Pod 的映射表.
// 参数 IPsStr 为以逗号分隔的点分十进制IP字符串, 如 "192.168.0.1/24,192.168.0.2/24"
func (h *Helper) InitIPMap(IPsStr string) (ipList []string, ipMap map[string]*ipkv1.OwnerPod) {
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
func (h *Helper) GetStaticIP(
	owner apimmetav1.Object,
	ownerKind string,
) (sip *ipkv1.StaticIP, err error) {
	ownerName := owner.GetName()
	ownerNS := owner.GetNamespace()

	sipName := h.generateSIPName(ownerKind, ownerName)

	return h.crdClient.IpkeeperV1().StaticIPs(ownerNS).Get(sipName, metav1.GetOptions{})
}

// CreateStaticIP 调用 crdClient 为目标资源(Pod/Deployment等)创建对应的 StaticIP 资源对象.
// @param ownerKind: 可选值 Pod, Deployment
// caller:
// 1. pkg/controller/handler_pod.go -> handleAddPod()
// 2. pkg/controller/handler_deploy.go -> handleAddDeploy()
func (h *Helper) CreateStaticIP(
	owner apimmetav1.Object,
	ownerKind string,
) (err error) {
	sip := h.NewStaticIP(owner, ownerKind)
	getOpt := &apimmetav1.GetOptions{}
	_, err = h.crdClient.IpkeeperV1().StaticIPs(sip.Namespace).Get(sip.Name, *getOpt)
	if err == nil {
		return fmt.Errorf("sip %s already exist, return", sip.Name)
	}
	// klog.Infof("try to create new sip: %s", sip.Name)

	_, err = h.crdClient.IpkeeperV1().StaticIPs(sip.Namespace).Create(sip)
	if err != nil {
		// if err.Error() == "already exists" {}
		utilruntime.HandleError(err)
		return fmt.Errorf("failed to create new sip for %s: %s", owner.GetName(), err)
	}
	// klog.Infof("success to create new sip object: %+v", actualSIP)
	return
}
