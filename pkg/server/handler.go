package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/emicklei/go-restful"
	corev1 "k8s.io/api/core/v1"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cgkuber "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	crdv1 "github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1"
	crdClientset "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned"
	"github.com/generals-space/crd-ipkeeper/pkg/restapi"
	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

// CNIServerHandler ...
type CNIServerHandler struct {
	Config     *Configuration
	kubeClient cgkuber.Interface
	crdClient  crdClientset.Interface
}

// newCNIServerHandler 挂载 cni server 的 rest api 接口.
func newCNIServerHandler(
	config *Configuration,
	kubeClient cgkuber.Interface,
	crdClient crdClientset.Interface,
) *CNIServerHandler {
	return &CNIServerHandler{
		Config:     config,
		kubeClient: kubeClient,
		crdClient:  crdClient,
	}
}

func (csh *CNIServerHandler) handleAdd(req *restful.Request, resp *restful.Response) {
	podReq := &restapi.PodRequest{}
	err := req.ReadEntity(podReq)
	if err != nil {
		klog.Errorf("parse add request failed %v", err)
		resp.WriteHeaderAndEntity(http.StatusBadRequest, err)
		return
	}
	klog.Infof("parsed request %v", podReq)

	var ipAddr, gateway string
	// 这里为什么要重试10次呢 ???
	for i := 0; i < 10; i++ {
		pod, err := csh.kubeClient.
			CoreV1().
			Pods(podReq.PodNamespace).
			Get(podReq.PodName, apimmetav1.GetOptions{})
		if err != nil {
			klog.Errorf("get pod %s/%s failed %v", podReq.PodNamespace, podReq.PodName, err)
			resp.WriteHeaderAndEntity(http.StatusInternalServerError, err)
			return
		}

		if pod.OwnerReferences != nil {
			ipAddr, gateway, err = csh.getAndOccupyOneIPByOwner(pod)
			if err != nil {
				klog.Errorf("get ipAddr and gateway from owner failed %v", err)
				resp.WriteHeaderAndEntity(http.StatusInternalServerError, err)
				return
			}
		} else {
			ipAddr = pod.Annotations[util.IPAddressAnnotation]
			gateway = pod.Annotations[util.GatewayAnnotation]
		}

		if ipAddr == "" || gateway == "" {
			// wait controller assign an address
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	// 如果 ipAddr 还是空, 说明此Pod/Deploy/DaemonSet没有声明固定IP的注解, 直接返回.
	if ipAddr == "" {
		resp.WriteHeaderAndEntity(
			http.StatusOK,
			restapi.PodResponse{
				DoNothing: true,
			},
		)
		return
	}

	klog.Infof("create container ip %s", ipAddr)

	err = csh.setVethPair(podReq, ipAddr, gateway)
	if err != nil {
		klog.Errorf("set veth pair failed %s", err)
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}
	resp.WriteHeaderAndEntity(
		http.StatusOK,
		restapi.PodResponse{
			IPAddress: ipAddr,
			Gateway:   gateway,
		},
	)
	return
}

// handleDel 处理Pod移除的事件
func (csh *CNIServerHandler) handleDel(req *restful.Request, resp *restful.Response) {
	resp.WriteHeader(http.StatusNoContent)
	return
}

// getAndOccupyOneIPByOwner 当发现请求来源的 Pod 属于其他资源时, 调用此函数.
func (csh *CNIServerHandler) getAndOccupyOneIPByOwner(pod *corev1.Pod) (ipAddr, gateway string, err error) {
	owner := pod.OwnerReferences[0]

	// deployment 通过 rs 管理 Pod, 但是 daemonset 却是直接管理的, 这两者要注意区分.
	if owner.Kind == "ReplicaSet" {
		rs, err := csh.kubeClient.AppsV1().ReplicaSets(pod.Namespace).Get(owner.Name, apimmetav1.GetOptions{})
		if err != nil {
			klog.Fatalf("failed to get replicaset for pod: %s", err)
			return "", "", err
		}
		if rs.OwnerReferences == nil {
			// 如果 rs 没有引用者
		} else {
			// 目前已知的 rs 的引用者只有 deployment
			rsOwner := rs.OwnerReferences[0]
			deploy, err := csh.kubeClient.AppsV1().Deployments(pod.Namespace).Get(rsOwner.Name, apimmetav1.GetOptions{})
			if err != nil {
				klog.Fatalf("failed to get deploy for pod: %s", err)
				return "", "", err
			}
			sipOwnerKind := "deploy"
			sipName := fmt.Sprintf("%s-%s-%s", deploy.Namespace, sipOwnerKind, deploy.Name)
			sip, err := csh.crdClient.IpkeeperV1().StaticIPs(deploy.Namespace).Get(sipName, apimmetav1.GetOptions{})
			if err != nil {
				klog.Fatalf("failed to get staticip for pod: %s", err)
				return "", "", err
			}
			return csh.getAndOccupyOneIP(sip, pod)
		}
	} else if owner.Kind == "DaemonSet" {

	}
	return
}

func (csh *CNIServerHandler) getAndOccupyOneIP(sip *crdv1.StaticIP, pod *corev1.Pod) (ipaddr, gateway string, err error) {
	for k, v := range sip.Spec.IPMap {
		if v == nil {
			ipaddr = k
			gateway = sip.Spec.Gateway
			sip.Spec.IPMap[k] = &crdv1.OwnerPod{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				UID:       pod.UID,
			}
			break
		}
	}
	if ipaddr == "" && gateway == "" {
		klog.Errorf("no more IP avaliable in sip: %s", sip.Name)
		return
	}
	_, err = csh.crdClient.IpkeeperV1().StaticIPs(sip.Namespace).Update(sip)
	if err != nil {
		klog.Errorf("failed to occupy one IP from sip: %s", sip.Name)
		return
	}
	klog.Infof("success to occupy one IP from sip: %s", sip.Name)
	return
}
