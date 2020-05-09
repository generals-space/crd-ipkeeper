package controller

import (
	"context"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kubeInformers "k8s.io/client-go/informers"
	cgkuber "k8s.io/client-go/kubernetes"
	cgscheme "k8s.io/client-go/kubernetes/scheme"
	cgcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	cglistersappsv1 "k8s.io/client-go/listers/apps/v1"
	cglisterscorev1 "k8s.io/client-go/listers/core/v1"
	cgcache "k8s.io/client-go/tools/cache"
	cgleaderelection "k8s.io/client-go/tools/leaderelection"
	cgrecord "k8s.io/client-go/tools/record"
	cgworkqueue "k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	crdClientset "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned"
	crdScheme "github.com/generals-space/crd-ipkeeper/pkg/client/clientset/versioned/scheme"
	crdInformers "github.com/generals-space/crd-ipkeeper/pkg/client/informers/externalversions"
	crdLister "github.com/generals-space/crd-ipkeeper/pkg/client/listers/ipkeeper/v1"
)

const controllerAgentName = "ipkeeper-controller"

// Controller ...
type Controller struct {
	CrdPodName string
	CrdPodNS   string

	kubeClient cgkuber.Interface
	crdClient  crdClientset.Interface

	kuberInformerFactory kubeInformers.SharedInformerFactory
	crdInformerFactory   crdInformers.SharedInformerFactory

	deployLister cglistersappsv1.DeploymentLister
	deploySynced cgcache.InformerSynced

	podLister cglisterscorev1.PodLister
	podSynced cgcache.InformerSynced

	sipLister crdLister.StaticIPLister
	sipSynced cgcache.InformerSynced

	// queue 的主要作用就是限流, 接收与处理是分为两个部分单独完成的.
	addSIPQueue    cgworkqueue.RateLimitingInterface
	addDeployQueue cgworkqueue.RateLimitingInterface
	// 我曾经尝试过修改 deployment 的 yaml 部署文件, 但无论修改其 metadata,
	// 还是修改 spec, 或是修改 spec.template 的 rs 部分, 都无法触发 update 事件,
	// 即无法进入 deployInformer 的 UpdateFunc() 回调中...
	// 反而是每隔一段时间(大概是30s, 或1min), 就会调用一次 UpdateFunc(), 不会主动触发???
	updateDeployQueue cgworkqueue.RateLimitingInterface
	// 由于每个 StaticIP 都绑定了 deployment/pod 作为 owner,
	// 在其 owner 被删除的同时会被关联删除, 所以其实也用不到 del 队列.
	delDeployQueue cgworkqueue.RateLimitingInterface

	addPodQueue cgworkqueue.RateLimitingInterface
	delPodQueue cgworkqueue.RateLimitingInterface

	recorder   cgrecord.EventRecorder
	electionID string
	elector    *cgleaderelection.LeaderElector

	stopCh <-chan struct{}
}

func makeRecorder(kubeClient cgkuber.Interface) (recorder cgrecord.EventRecorder) {
	eventBroadcaster := cgrecord.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&cgcorev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events(""),
		},
	)
	recorder = eventBroadcaster.NewRecorder(
		cgscheme.Scheme,
		corev1.EventSource{
			Component: controllerAgentName,
		},
	)
	return
}

// NewController 创建并返回 Controller 结构体对象.
func NewController(
	kubeClient cgkuber.Interface,
	crdClient crdClientset.Interface,
) (controller *Controller, err error) {
	// 所有 CRD controller 都把这一句放在第一位
	utilruntime.Must(crdScheme.AddToScheme(cgscheme.Scheme))

	kubeInformerFactory := kubeInformers.NewSharedInformerFactory(
		kubeClient, time.Second*30,
	)
	crdInformerFactory := crdInformers.NewSharedInformerFactory(
		crdClient, time.Second*30,
	)
	deployInformer := kubeInformerFactory.Apps().V1().Deployments()
	podInformer := kubeInformerFactory.Core().V1().Pods()
	sipInformer := crdInformerFactory.Ipkeeper().V1().StaticIPs()

	controller = &Controller{
		CrdPodName: os.Getenv("POD_NAME"),
		CrdPodNS:   os.Getenv("POD_NS"),

		kubeClient: kubeClient,
		crdClient:  crdClient,

		kuberInformerFactory: kubeInformerFactory,
		crdInformerFactory:   crdInformerFactory,

		sipLister: sipInformer.Lister(),
		sipSynced: sipInformer.Informer().HasSynced,

		deployLister: deployInformer.Lister(),
		deploySynced: deployInformer.Informer().HasSynced,

		podLister: podInformer.Lister(),
		podSynced: podInformer.Informer().HasSynced,

		addSIPQueue: cgworkqueue.NewNamedRateLimitingQueue(
			cgworkqueue.DefaultControllerRateLimiter(),
			"AddSIP",
		),
		addDeployQueue: cgworkqueue.NewNamedRateLimitingQueue(
			cgworkqueue.DefaultControllerRateLimiter(),
			"AddDeploy",
		),
		updateDeployQueue: cgworkqueue.NewNamedRateLimitingQueue(
			cgworkqueue.DefaultControllerRateLimiter(),
			"UpdateDeploy",
		),
		delDeployQueue: cgworkqueue.NewNamedRateLimitingQueue(
			cgworkqueue.DefaultControllerRateLimiter(),
			"DelDeploy",
		),
		addPodQueue: cgworkqueue.NewNamedRateLimitingQueue(
			cgworkqueue.DefaultControllerRateLimiter(),
			"AddPod",
		),
		delPodQueue: cgworkqueue.NewNamedRateLimitingQueue(
			cgworkqueue.DefaultControllerRateLimiter(),
			"DelPod",
		),
		recorder:   makeRecorder(kubeClient),
		electionID: "crd-ipkeeper",
	}

	deployInformer.Informer().AddEventHandler(
		cgcache.ResourceEventHandlerFuncs{
			AddFunc:    controller.enqueueAddDeploy,
			UpdateFunc: controller.enqueueUpdateDeploy,
			DeleteFunc: controller.enqueueDelDeploy,
		},
	)
	podInformer.Informer().AddEventHandler(
		cgcache.ResourceEventHandlerFuncs{
			// AddFunc 在 AddFunc 被触发时, Pod 还处于 Pending 状态,
			// 且 Status 块基本都还是空的, 无法从此处获取到 hostIP, podIP 等信息.
			// 这个时机应该处于 pause 容器创建之前(或者说过程中),
			// 因为此时 pause 还没有调用 cni 插件申请到 IP 地址.
			// AddFunc:    controller.enqueueAddPod,
			DeleteFunc: controller.enqueueDelPod,
		},
	)
	return
}

// Run 监听 deployment, daemonset 等类型资源的变动.
// caller: cmd/main.go
// @param stopCh: 在 SetupSignalHandler() 声明, 为无缓冲 channel,
// 当接收到 sigterm 信号时会将此 channel 关闭,
// 这也会导致传入此通道的 informer 与 各资源类型的 worker 终止退出.
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.addSIPQueue.ShutDown()
	defer c.addDeployQueue.ShutDown()
	defer c.updateDeployQueue.ShutDown()
	defer c.addPodQueue.ShutDown()
	defer c.delPodQueue.ShutDown()

	c.stopCh = stopCh
	// 创建分布式资源锁, 执行竞争, 并挂载回调处理函数
	// 当成为 leader 后, 执行 c.run() 正式开启业务流程
	// ...不过当 leader 身份得到又失去, 会发生什么? 从哪里开始重新执行?
	c.setupLeaderElection()
}

func (c *Controller) isLeader() bool {
	return c.elector.IsLeader()
}

// run 分布式资源锁竞争成功, 成为 leader 后, 执行此方法.
// caller: c.setupLeaderElection()
func (c *Controller) run(ctx context.Context) {
	klog.Infof("I am the new leader")

	// 在执行 WaitForCacheSync() (和 启动 Worker ???)之前, 一定要先运行如下语句.
	// 否则 WaitForCacheSync() 会卡住, 而且貌似根本进不去这个函数.
	c.kuberInformerFactory.Start(c.stopCh)
	c.crdInformerFactory.Start(c.stopCh)

	ok := cgcache.WaitForCacheSync(c.stopCh, c.sipSynced, c.deploySynced)
	if !ok {
		klog.Fatal("failed to wait for caches to sync")
		return
	}
	// 调用 controller 中的各个 worker 处理各自资源队列中的事件变动.
	klog.Info("Starting workers")
	go utilwait.Until(c.runAddDeployWorker, time.Second, c.stopCh)
	go utilwait.Until(c.runUpdateDeployWorker, time.Second, c.stopCh)
	// 貌似因为设置了 Owner, 所以当 deploy 被移除的时候, 被绑定的 StaticIP 也会被移除.
	// 不需要额外的 del 操作.
	// go utilwait.Until(c.runDelDeployWorker, time.Second, c.stopCh)

	// go utilwait.Until(c.runAddPodWorker, time.Second, c.stopCh)
	go utilwait.Until(c.runDelPodWorker, time.Second, c.stopCh)

	klog.Info("Started workers")
	<-c.stopCh
	klog.Info("Shutting down workers")
	return
}
