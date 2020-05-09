package server

import (
	"net/http"
	"time"

	"github.com/emicklei/go-restful"
	apimmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cgkuber "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	crdClientset "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned"
	"github.com/generals-space/crd-ipkeeper/pkg/restapi"
	"github.com/generals-space/crd-ipkeeper/pkg/staticip"
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
		// 如果一个 Pod 被 deploy/daemonset 等所有, 那么 StaticIP 一定已经创建好了.
		// 而如果只是一个单 Pod 资源, 那要等到该 Pod 创建完成后再去补上 StaticIP 对象了.
		if pod.OwnerReferences != nil {
			sip, err := staticip.GetPodOwnerSIP(csh.kubeClient, csh.crdClient, pod)
			if err != nil {
				klog.Errorf("get sip from owner failed %v", err)
				resp.WriteHeaderAndEntity(http.StatusInternalServerError, err)
				return
			}
			ipAddr, gateway, err = staticip.AccquireIP(csh.crdClient, sip, pod)
			// ipAddr, gateway, err = csh.getAndOccupyOneIPByOwner(pod)
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

		if pod.OwnerReferences == nil {
			err = staticip.CreateAndRequireIP(csh.crdClient, pod)
			if err != nil {
				klog.Errorf("get ipAddr and gateway from owner failed %v", err)
				resp.WriteHeaderAndEntity(http.StatusInternalServerError, err)
				return
			}
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
