package controller

import (
	"context"
	"os"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cgscheme "k8s.io/client-go/kubernetes/scheme"
	cgleaderelection "k8s.io/client-go/tools/leaderelection"
	cgresourcelock "k8s.io/client-go/tools/leaderelection/resourcelock"
	cgrecord "k8s.io/client-go/tools/record"
	"k8s.io/klog"
)

const ipkeeperLeaderElector = "ipkeeper-controller-leader-elector"

func (c *Controller) setupLeaderElection() {
	var err error

	broadcaster := cgrecord.NewBroadcaster()
	recorder := broadcaster.NewRecorder(
		cgscheme.Scheme,
		apicorev1.EventSource{
			Component: ipkeeperLeaderElector,
			Host:      os.Getenv("KUBE_NODE_NAME"),
		},
	)
	// kuber 的分布式资源锁其实是通过创建 configmap/endpoints 资源对象实现的.
	// 当然, 对ta们的修改依赖于乐观锁机制.
	rlock := cgresourcelock.ConfigMapLock{
		ConfigMapMeta: metav1.ObjectMeta{
			Name: c.electionID,
			// 作为 rlock 的 configmap 资源当然要和 crd pod 资源在同一个 ns 下.
			Namespace: c.CrdPodNS,
		},
		Client: c.kubeClient.CoreV1(),
		LockConfig: cgresourcelock.ResourceLockConfig{
			Identity:      c.CrdPodName,
			EventRecorder: recorder,
		},
	}
	// 这里将 ttl 设置 8s 是有意义的, 因为下面分别对其除以 2 和 4.
	ttl := time.Second * 8
	c.elector, err = cgleaderelection.NewLeaderElector(
		cgleaderelection.LeaderElectionConfig{
			Lock:          &rlock,
			LeaseDuration: ttl,
			RenewDeadline: ttl / 2,
			RetryPeriod:   ttl / 4,
			// 成功当选为 leader, 或终止作为 leader 时调用如下回调.
			Callbacks: cgleaderelection.LeaderCallbacks{
				OnStartedLeading: c.run,
				OnStoppedLeading: c.onStoppedLeading,
				OnNewLeader:      c.onNewLeader,
			},
		},
	)
	if err != nil {
		klog.Fatalf("unexpected error starting leader election: %v", err)
	}
	// cgleaderelection 包中有一个 RunOrDie() 方法,
	// 结合了 NewLeaderElector() 与下面的 Run() 两个函数.
	c.elector.Run(context.Background())
}

func (c *Controller) onStoppedLeading() {
	klog.Info("I am not leader anymore")
}

func (c *Controller) onNewLeader(identity string) {
	klog.Infof("new leader elected: %v", identity)
}
