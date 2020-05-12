package staticip

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	crdv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
)

// AccquireIP 从目标 sip 对象的 IPMap 中找到可用的 IP 并返回,
// 同时修改 sip 对象的 Avaliable 和 Used 列表(但不更新, 更新操作由调用者完成).
// caller: pkg/server/handler.go -> CNIServerHandler.handleAdd()
// 调用时机为在创建 pause 容器, 调用 cni ipam 插件申请的过程中,
// 而不是在 controller 中通过监听 Pod 的 Add 事件,
// 因为后者触发在前者之前, 根本没有办法实现...
func (h *Helper) AccquireIP(
	sip *ipkv1.StaticIP,
	pod *corev1.Pod,
) (ipaddr, gateway string, err error) {
	for ip, ownerPod := range sip.Spec.IPMap {
		if ownerPod == nil {
			ipaddr = ip
			gateway = sip.Spec.Gateway
			sip.Spec.IPMap[ip] = &crdv1.OwnerPod{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				UID:       pod.UID,
			}
			break
		}
	}
	// 如果没找到就直接返回错误.
	if ipaddr == "" && gateway == "" {
		return "", "", fmt.Errorf("no more IP avaliable in sip: %s", sip.Name)
	}

	newAvaliable := []string{}
	for _, ip := range sip.Spec.Avaliable {
		if ip == ipaddr {
			continue
		}
		newAvaliable = append(newAvaliable, ip)
	}
	sip.Spec.Avaliable = newAvaliable
	sip.Spec.Used = append(sip.Spec.Used, ipaddr)
	sip.Spec.Ratio = fmt.Sprintf("%d/%d", len(sip.Spec.Used), len(sip.Spec.IPMap))

	_, err = h.crdClient.IpkeeperV1().StaticIPs(sip.Namespace).Update(sip)
	if err != nil {
		return "", "", fmt.Errorf("failed to occupy one IP from sip: %s", sip.Name)
	}
	return
}

// ReleaseIP ...
func (h *Helper) ReleaseIP(
	sip *ipkv1.StaticIP,
	pod *corev1.Pod,
) (err error) {
	var podIP string
	/*
		// pod.Status.PodIP 是没有掩码位的, 所以不能这么用.
		podIP = pod.Status.PodIP
		_, ok := sip.Spec.IPMap[podIP]
		if !ok {
			klog.Warningf("get a ip that not belong to it: %s", podIP)
			return
		}
	*/
	for ip, ownerPod := range sip.Spec.IPMap {
		if ownerPod == nil {
			continue
		}
		if ownerPod.UID == pod.UID {
			podIP = ip
			break
		}
	}
	if podIP == "" {
		klog.Warningf("the pod: %s has an ip that not belong to it's staticip", pod.Name)
		return
	}
	newUsed := []string{}
	for _, ip := range sip.Spec.Used {
		if ip == podIP {
			continue
		}
		newUsed = append(newUsed, ip)
	}
	sip.Spec.Used = newUsed
	sip.Spec.Avaliable = append(sip.Spec.Avaliable, podIP)
	sip.Spec.IPMap[podIP] = nil
	sip.Spec.Ratio = fmt.Sprintf("%d/%d", len(sip.Spec.Used), len(sip.Spec.IPMap))
	_, err = h.crdClient.IpkeeperV1().StaticIPs(sip.Namespace).Update(sip)
	if err != nil {
		klog.Errorf("failed to occupy one IP from sip: %s", sip.Name)
		return
	}
	return
}
