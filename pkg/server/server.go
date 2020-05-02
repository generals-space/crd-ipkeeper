package server

import (
	"net"
	"net/http"
	"os"

	restful "github.com/emicklei/go-restful"
	"k8s.io/klog"

	"github.com/generals-space/crd-ipkeeper/pkg/restapi"
)

// CNIServer ...
type CNIServer struct {
	config     *Configuration
	handler    *CNIServerHandler
	httpServer *http.Server
}

// NewCNIServer ...
func NewCNIServer(config *Configuration) *CNIServer {
	cniServer := &CNIServer{
		config:  config,
		handler: newCNIServerHandler(config),
	}
	cniServer.createHandler()
	return cniServer
}

// Run 启动Unix http服务器.
func (s *CNIServer) Run() {
	unixListener, err := net.Listen("unix", s.config.BindSocket)
	if err != nil {
		klog.Errorf("bind socket to %s failed %v", s.config.BindSocket, err)
		return
	}
	defer os.Remove(s.config.BindSocket)

	klog.Infof("start listen on %s", s.config.BindSocket)
	klog.Fatal(s.httpServer.Serve(unixListener))
}

// createHandler 挂载 cni server 的 rest api 接口, 实际的处理方法在 s.handler 成员对象中.
func (s *CNIServer) createHandler() {
	wsContainer := restful.NewContainer()
	wsContainer.EnableContentEncoding(true)

	ws := new(restful.WebService)
	ws.Path("/api/v1").Consumes(restful.MIME_JSON).Produces(restful.MIME_JSON)
	wsContainer.Add(ws)

	ws.Route(
		ws.POST("/add").To(s.handler.handleAdd).Reads(restapi.PodRequest{}),
	)
	// 处理Pod移除的事件, 从cni网桥拨出宿主机端的veth等操作.
	// ...不过我觉得根本没必要, 所以 s.handler.handleDel() 其实是个空函数.
	ws.Route(
		ws.POST("/del").To(s.handler.handleDel).Reads(restapi.PodRequest{}),
	)

	s.httpServer = &http.Server{
		Handler: wsContainer,
	}
	return
}
