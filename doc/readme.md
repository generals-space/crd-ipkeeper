
对于Pod的处理(CURD)在`kubernetes`的`kubelet`子工程中完成, 在`Kubelet`结构体中存在`podManager`成员, 为`pkg/kubelet/pod`包下的成员.

`Kubelet`结构中`podManager`与`containerRuntime`是我们关注的重点.

对于`container`的操作在`pkg/kubelet/container/runtime.go`中定义.

- `pkg/kubelet/runonce.go` -> `RunOnce()` -> `runOnce()`
- `pkg/kubelet/kubelet.go` -> `kl.syncPod()`
- `pkg/kubelet/kuberuntime/kuberuntime_manager.go` -> `SyncPod()`
- `pkg/kubelet/kuberuntime/kuberuntime_container.go` -> `startContainer()`中的步骤
    1. 拉取镜像
    2. 创建容器
    3. 启动容器
    4. 运行post start钩子函数

------

在`kubernetes`工程的`controller manager`项目中存在很多控制器, 比如`Job`, `Endpoint`, `Service`等.

而`Pod`资源的controller定义则在`k8s.io/api/core/v1/types.go`中声明.

在`k8s.io/api`工程中声明的控制器应该是核心控制器, 包括`Pod`, `Event`, `Networking`等, 而在`kubernetes`工程中的控制器算是扩展控制器, 比如`Deployment`, `DaemonSet`, `CronJob`等.

然后在`kubernetes/pkg/controller/controller_utils.go`给出了一个接口, 用来对Pod做CURD操作的, 应该是给controller目录下其他资源一个通用的操作工具, 这也可见Pod资源的不一般.

