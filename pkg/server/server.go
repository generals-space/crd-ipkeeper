package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	restful "github.com/emicklei/go-restful"
	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/restapi"
)

var (
	reqLogString  = "[%s] Incoming %s %s %s request from %s"
	respLogString = "[%s] Outcoming response to %s %s %s with %d status code in %vms"
)

// RunServer 启动Unix http服务器.
func RunServer(cfg *Configuration) {
	csh, err := newCNIServerHandler(cfg)
	if err != nil {
		klog.Fatalf("create cni server handler failed %v", err)
		return
	}
	server := http.Server{
		Handler: createHandler(csh),
	}
	unixListener, err := net.Listen("unix", cfg.BindSocket)
	if err != nil {
		klog.Errorf("bind socket to %s failed %v", cfg.BindSocket, err)
		return
	}
	defer os.Remove(cfg.BindSocket)

	klog.Infof("start listen on %s", cfg.BindSocket)
	klog.Fatal(server.Serve(unixListener))
}

// createHandler 挂载 cni server 的 rest api 接口.
func createHandler(csh *CNIServerHandler) http.Handler {
	wsContainer := restful.NewContainer()
	wsContainer.EnableContentEncoding(true)

	ws := new(restful.WebService)
	ws.Path("/api/v1").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	wsContainer.Add(ws)

	ws.Route(
		ws.POST("/add").To(csh.handleAdd).Reads(restapi.PodRequest{}),
	)
	ws.Route(
		ws.POST("/del").To(csh.handleDel).Reads(restapi.PodRequest{}),
	)

	ws.Filter(requestAndResponseLogger)

	return wsContainer
}

// web-service filter function used for request and response logging.
func requestAndResponseLogger(
	req *restful.Request,
	resp *restful.Response,
	chain *restful.FilterChain,
) {
	klog.Infof(formatRequestLog(req))
	start := time.Now()
	chain.ProcessFilter(req, resp)
	elapsed := float64((time.Since(start)) / time.Millisecond)
	klog.Infof(formatResponseLog(resp, req, elapsed))
}

// formatRequestLog formats request log string.
func formatRequestLog(request *restful.Request) string {
	uri := ""
	if request.Request.URL != nil {
		uri = request.Request.URL.RequestURI()
	}

	return fmt.Sprintf(
		reqLogString,
		time.Now().Format(time.RFC3339), request.Request.Proto,
		request.Request.Method, uri, request.Request.RemoteAddr,
	)
}

// formatResponseLog formats response log string.
func formatResponseLog(response *restful.Response, request *restful.Request, reqTime float64) string {
	uri := ""
	if request.Request.URL != nil {
		uri = request.Request.URL.RequestURI()
	}
	return fmt.Sprintf(
		respLogString,
		time.Now().Format(time.RFC3339), request.Request.RemoteAddr,
		request.Request.Method, uri, response.StatusCode(), reqTime,
	)
}
