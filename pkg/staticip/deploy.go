package staticip

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cgkuber "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	crdClientset "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned"
)

// RenewStaticIP ...
// caller: handleUpdateDeploy()
func RenewStaticIP(
	kubeClient cgkuber.Interface,
	crdClient crdClientset.Interface,
	oldSIP, newSIP *ipkv1.StaticIP,
) (err error) {
	sip, pods, err := renewStaticIP(oldSIP, newSIP)
	// 将更新 StaticIP 对象, 再移除多余的 pods
	_, err = crdClient.IpkeeperV1().StaticIPs(newSIP.Namespace).Update(sip)
	if err != nil {
		return fmt.Errorf("failed to update static ip: %s", err)
	}

	for _, pod := range pods {
		err = kubeClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{})
		if err != nil {
			klog.Warningf("failed to delete redundant pod: %+v: %s", pod, err)
		}
	}

	return
}

func renewStaticIP(
	oldSIP, newSIP *ipkv1.StaticIP,
) (sip *ipkv1.StaticIP, pods []*ipkv1.OwnerPod, err error) {
	pods = []*ipkv1.OwnerPod{}
	// 遍历 oldSIP 已经分配出去的 IP 列表, 若有成员未在 newSIP 的 IPPool 中,
	// 则需要将已经分配了这些 IP 的 Pod 移除.
	for _, ip := range oldSIP.Spec.Used {
		ownerPod := oldSIP.Spec.IPMap[ip]
		_, ok := newSIP.Spec.IPMap[ip]

		if ok {
			// 如果已分配的 IP 仍属于新的 StaticIP 范围, 则加入到 Used 列表中,
			newSIP.Spec.Used = append(newSIP.Spec.Used, ip)
			newSIP.Spec.IPMap[ip] = ownerPod
		} else if ownerPod != nil {
			// 如果该 IP 已被移除, 则占用此 IP 的 Pod 也需要被移除.
			// 不过移除的操作由主调函数完成.
			pods = append(pods, ownerPod)
		}
	}

	// 还有要将 avaliable 减去 used 的部分.
	for ip, pod := range newSIP.Spec.IPMap {
		if pod != nil {
			continue
		}
		newSIP.Spec.Avaliable = append(newSIP.Spec.Avaliable, ip)
	}

	// 因为本函数是为 Update 操作做准备, 而 Update 操作需要 StaticIP 对象
	// 拥有 resourceVersion 字段, 所以这里将更新后的信息赋值给 oldSIP,
	// 之后的 Update 操作也将使用 sip 作为目标对象.
	sip = oldSIP
	sip.Spec = newSIP.Spec
	return
}
