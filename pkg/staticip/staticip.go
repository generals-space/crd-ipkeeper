package staticip

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cgkuber "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	crdv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	ipkv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	crdClientset "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned"
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

// GetPodOwner 获取 Pod 的 owner 对象, 及其资源类型.
func GetPodOwner(
	kubeClient cgkuber.Interface,
	pod *corev1.Pod,
) (owner apimmetav1.Object, ownerKind string, err error) {
	// 如果 Pod 没有 owner, 则返回 Pod 本身.
	if pod.OwnerReferences == nil {
		if pod.Annotations[util.IPAddressAnnotation] != "" &&
			pod.Annotations[util.GatewayAnnotation] != "" {
			return pod, "Pod", nil
		}
		return nil, "", fmt.Errorf("the pod doesn't have ipaddr annotation %s, ignore", pod.Name)
	}

	ownerRef := pod.OwnerReferences[0]

	// deployment 通过 rs 管理 Pod, 但是 daemonset 却是直接管理的, 这两者要注意区分.
	if ownerRef.Kind == "ReplicaSet" {
		rs, err := kubeClient.AppsV1().ReplicaSets(pod.Namespace).Get(ownerRef.Name, apimmetav1.GetOptions{})
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
				deploy, err := kubeClient.AppsV1().Deployments(pod.Namespace).Get(rsOwner.Name, apimmetav1.GetOptions{})
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

// GetPodOwnerSIP ...
func GetPodOwnerSIP(
	kubeClient cgkuber.Interface,
	crdClient crdClientset.Interface,
	pod *corev1.Pod,
) (sip *ipkv1.StaticIP, err error) {
	owner, kind, err := GetPodOwner(kubeClient, pod)
	if err != nil {
		return
	}
	if kind == "Deployment" {
		deploy := owner.(*appsv1.Deployment)
		sipName := GenerateSIPName("Deployment", deploy.Name)
		sip, err = crdClient.IpkeeperV1().StaticIPs(deploy.Namespace).Get(sipName, apimmetav1.GetOptions{})
		if err != nil {
			if !strings.HasSuffix(err.Error(), "not found") {
				klog.Errorf("failed to get staticip for pod: %s", err)
			}
			return nil, err
		}
	} else if kind == "Pod" {
		pod := owner.(*corev1.Pod)
		sipName := GenerateSIPName("Pod", pod.Name)
		sip, err = crdClient.IpkeeperV1().StaticIPs(pod.Namespace).Get(sipName, apimmetav1.GetOptions{})
		if err != nil {
			if !strings.HasSuffix(err.Error(), "not found") {
				klog.Errorf("failed to get staticip for pod: %s", err)
			}
			return nil, err
		}
	}
	return
}

// AccquireIP 从目标 sip 对象的 IPMap 中找到可用的 IP 并返回,
// 同时修改 sip 对象的 Avaliable 和 Used 列表(但不更新, 更新操作由调用者完成).
// caller: pkg/server/handler.go -> CNIServerHandler.handleAdd()
// 调用时机为在创建 pause 容器, 调用 cni ipam 插件申请的过程中,
// 而不是在 controller 中通过监听 Pod 的 Add 事件,
// 因为后者触发在前者之前, 根本没有办法实现...
func AccquireIP(
	crdClient crdClientset.Interface,
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

	_, err = crdClient.IpkeeperV1().StaticIPs(sip.Namespace).Update(sip)
	if err != nil {
		return "", "", fmt.Errorf("failed to occupy one IP from sip: %s", sip.Name)
	}
	return
}

// ReleaseIP ...
func ReleaseIP(
	crdClient crdClientset.Interface,
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
	_, err = crdClient.IpkeeperV1().StaticIPs(sip.Namespace).Update(sip)
	if err != nil {
		klog.Errorf("failed to occupy one IP from sip: %s", sip.Name)
		return
	}
	return
}

// CreateAndRequireIP 为指定了静态IP的单个 Pod 资源(没有其他Owner), 创建 StaticIP 对象.
func CreateAndRequireIP(
	crdClient crdClientset.Interface,
	pod *corev1.Pod,
) (err error) {
	sip := NewStaticIP(pod, "Pod")
	sip.Spec.IPMap[sip.Spec.IPPool] = &crdv1.OwnerPod{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		UID:       pod.UID,
	}
	sip.Spec.Used = []string{sip.Spec.IPPool}
	sip.Spec.Avaliable = []string{}
	sip.Spec.Ratio = fmt.Sprintf("%d/%d", len(sip.Spec.Used), len(sip.Spec.IPMap))

	klog.V(3).Infof("new sip ojbect: %+v", sip)
	actualSIP, err := crdClient.IpkeeperV1().StaticIPs(pod.Namespace).Create(sip)
	if err != nil {
		// if err.Error() == "already exists" {}
		klog.Fatalf("failed to create new StaticIP for Pod %s: %s", pod.Name, err)
		return err
	}
	klog.Infof("success to create new sip object: %+v", actualSIP)
	return
}
