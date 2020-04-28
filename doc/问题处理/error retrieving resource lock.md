# error retrieving resource lock

## 问题描述

pod启动成功, 但是貌似在进行分布式资源锁竞争的时候出现了错误, 报错的也不是本工程中的代码.

```
[root@k8s-master-01 crd-ipkeeper]# k logs -f crd-ipkeeper-zqzjb
I0427 13:31:30.465415   44826 kubeclient.go:16] no --kubeconfig, use in-cluster kubernetes config
I0427 13:31:30.467605   44826 config.go:43] bind socket: /var/run/cniserver.sock
W0427 13:31:30.467668   44826 client_config.go:543] Neither --kubeconfig nor --master was specified.  Using the inClusterConfig.  This might not work.
I0427 13:31:30.469388   44826 leaderelection.go:242] attempting to acquire leader lease  /crd-ipkeeper...
E0427 13:31:30.469488   44826 leaderelection.go:331] error retrieving resource lock /crd-ipkeeper: an empty namespace may not be set when a resource name is provided
E0427 13:31:33.921378   44826 leaderelection.go:331] error retrieving resource lock /crd-ipkeeper: an empty namespace may not be set when a resource name is provided
```

当时关于这个`ConfigMap`类型的资源锁的定义是这样的

```go
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
```

其中`c.CrdPodNS`为环境变量`POD_NS`的值, 不过我忘记在`yaml`文件中加了, 添加上如下代码块即可

```yaml
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NS
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```
