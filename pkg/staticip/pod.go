package staticip

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

// GetPodOwner 获取 Pod 的 owner 对象, 及其资源类型, 如果没有则返回其自身.
func (h *Helper) GetPodOwner(
	pod *corev1.Pod,
) (owner apimmetav1.Object, ownerKind string, err error) {
	// 如果 Pod 没有 owner, 则返回 Pod 本身.
	if pod.OwnerReferences == nil {
		if pod.Annotations[util.IPAddressAnnotation] != "" &&
			pod.Annotations[util.GatewayAnnotation] != "" {
			return pod, "Pod", nil
		}
		return nil, "", fmt.Errorf("the pod %s doesn't have ipaddr annotation, ignore", pod.Name)
	}

	ownerRef := pod.OwnerReferences[0]

	// deployment 通过 rs 管理 Pod, 但是 daemonset 却是直接管理的, 这两者要注意区分.
	if ownerRef.Kind == "ReplicaSet" {
		rs, err := h.kubeClient.
			AppsV1().
			ReplicaSets(pod.Namespace).
			Get(ownerRef.Name, apimmetav1.GetOptions{})

		if err != nil {
			klog.Errorf("failed to get replicaset for pod: %s", err)
			return nil, "", err
		}
		if rs.OwnerReferences == nil {
			// 如果 rs 没有引用者
		} else {
			rsOwner := rs.OwnerReferences[0]
			// 目前已知的 rs 的引用者只有 deployment
			if rsOwner.Kind == "Deployment" {
				deploy, err := h.kubeClient.
					AppsV1().
					Deployments(pod.Namespace).
					Get(rsOwner.Name, apimmetav1.GetOptions{})

				if err != nil {
					klog.Errorf("failed to get deploy for pod: %s", err)
					return nil, "", err
				}
				return deploy, "Deployment", nil
			}
		}
	} else if ownerKind == "DaemonSet" {
		//
	}

	return nil, "", fmt.Errorf("doesn't support resource type: %s", ownerRef.Kind)
}

// GetPodOwnerSIP 返回目标 Pod 对象对应的 StaticIP 对象.
// 如果是单 Pod 资源(没有 Owner), 可能在调用时相应的 StaticIP 对象还未能创建,
// 此时 err 为 nil, sip 也为 nil, 需要注意.
func (h *Helper) GetPodOwnerSIP(
	pod *corev1.Pod,
) (sip *ipkv1.StaticIP, err error) {
	owner, kind, err := h.GetPodOwner(pod)
	if err != nil {
		return
	}
	if kind == "Deployment" {
		deploy := owner.(*appsv1.Deployment)
		sipName := h.generateSIPName("Deployment", deploy.Name)
		sip, err = h.crdClient.
			IpkeeperV1().
			StaticIPs(deploy.Namespace).
			Get(sipName, apimmetav1.GetOptions{})
		if err != nil {
			if !strings.HasSuffix(err.Error(), "not found") {
				klog.Errorf("failed to get staticip for pod: %s", err)
			}
			return nil, err
		}
	} else if kind == "Pod" {
		// 由于这个函数是在处理 Deployment/Pod 资源的 Add 方法中被调用的,
		// 但由于单个 Pod 的 Add 方法被触发时, 还没有到创建 pause 容器与申请 IP 那一步,
		// 所以单个 Pod 资源在执行到这里(get sip)的时候一定会出错.
		pod := owner.(*corev1.Pod)
		sipName := h.generateSIPName("Pod", pod.Name)
		sip, err = h.crdClient.
			IpkeeperV1().
			StaticIPs(pod.Namespace).
			Get(sipName, apimmetav1.GetOptions{})
		if err != nil {
			// 如果 Pod 没有 owner
			if !strings.HasSuffix(err.Error(), "not found") {
				klog.Errorf("failed to get staticip for pod: %s", err)
			} else {
				err = nil
			}
			return nil, err
		}
	}
	return
}
