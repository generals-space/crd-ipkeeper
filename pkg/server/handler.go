package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/emicklei/go-restful"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/restapi"
	"github.com/generals-space/crd-ipkeeper/pkg/util"
)

// CNIServerHandler ...
type CNIServerHandler struct {
	Config     *Configuration
	KubeClient kubernetes.Interface
}

// newCNIServerHandler 挂载 cni server 的 rest api 接口.
func newCNIServerHandler(config *Configuration) (*CNIServerHandler, error) {
	csh := &CNIServerHandler{
		KubeClient: config.KubeClient,
		Config:     config,
	}
	return csh, nil
}

func (csh *CNIServerHandler) handleAdd(req *restful.Request, resp *restful.Response) {
	podReq := &restapi.PodRequest{}
	err := req.ReadEntity(podReq)
	if err != nil {
		klog.Errorf("parse add request failed %v", err)
		resp.WriteHeaderAndEntity(http.StatusBadRequest, err)
		return
	}
	klog.Infof("add port request %v", podReq)

	var ipAddr string
	for i := 0; i < 10; i++ {
		pod, err := csh.KubeClient.CoreV1().Pods(podReq.PodNamespace).Get(podReq.PodName, v1.GetOptions{})
		if err != nil {
			klog.Errorf("get pod %s/%s failed %v", podReq.PodNamespace, podReq.PodName, err)
			resp.WriteHeaderAndEntity(http.StatusInternalServerError, err)
			return
		}
		ipAddr = pod.Annotations[util.IpAddressAnnotation]

		if ipAddr == "" {
			// wait controller assign an address
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}

	klog.Infof("create container ip %s", ipAddr)

	err = csh.setNic(
		podReq.PodName,
		podReq.PodNamespace,
		podReq.NetNs,
		podReq.ContainerID,
		podReq.CNI0,
		ipAddr,
	)
	if err != nil {
		klog.Errorf("configure nic failed %v", err)
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}
	resp.WriteHeaderAndEntity(
		http.StatusOK,
		restapi.PodResponse{
			IPAddress:  strings.Split(ipAddr, "/")[0],
		},
	)
	return
}

func (csh *CNIServerHandler) handleDel(req *restful.Request, resp *restful.Response) {
	podReq := &restapi.PodRequest{}
	err := req.ReadEntity(podReq)
	if err != nil {
		klog.Errorf("parse del request failed %v", err)
		resp.WriteHeaderAndEntity(http.StatusBadRequest, err)
		return
	}
	klog.Infof("delete port request %v", podReq)
	err = csh.deleteNic(podReq.NetNs, podReq.ContainerID)
	if err != nil {
		klog.Errorf("del nic failed %v", err)
		resp.WriteHeaderAndEntity(http.StatusInternalServerError, err)
		return
	}
	resp.WriteHeader(http.StatusNoContent)
	return
}
