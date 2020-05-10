# crd信息的格式化输出

在CRD的 yaml 声明中, 可以使用`additionalPrinterColumns`定义`kubectl get`打印出的信息格式, 如下.

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  ## metadata.name = plural值 + group值
  name: staticips.ipkeeper.generals.space
spec:
  group: ipkeeper.generals.space
  version: v1
  ## Namespaced 或 Cluster
  scope: Namespaced
  names:
    kind: StaticIP
    ## 单数
    singular: staticip
    ## 复数
    plural: staticips
    ## 缩写
    shortNames: ["sip"]
  ## `kubectl get sip`时的额外输出列
  additionalPrinterColumns:
  - name: OwnerKind
    type: string
    description: 所属资源(Pod, Deployment, Daemonset等)
    JSONPath: .spec.ownerKind
  - name: Namespace
    type: string
    JSONPath: .spec.namespace
  - name: IPPool
    type: string
    description: IP池
    JSONPath: .spec.ipPool
```

其输出格式为:

```console
$ k get sip
NAME                   OWNERKIND    NAMESPACE     IPPOOL
deploy-devops-deploy   Deployment   kube-system   172.16.91.142/24,172.16.91.143/24
```

我之前想加一个类似于 Pod 或 Deployment 信息中 `Ready` 列, 用于表示资源的使用情况, 大概格式如下

```console
$ k get deploy
NAME            READY   UP-TO-DATE   AVAILABLE   AGE
coredns         2/2     2            2           20d
```

但是`additionalPrinterColumns`并没有类似`fmt.Sprintf("%d/%d")`这种程序级的能力, 所以只能找找 Pod 或 Deployment 是如何实现的(因为`kubect get pod pod名称 -o yaml`的输出中并没有`1/1`这种形式的字段存在).

以`v1.16.2`为例, [pkg/printers/internalversion/printers.go](https://github.com/kubernetes/kubernetes/blob/v1.16.2/pkg/printers/internalversion/printers.go) 中的`podColumnDefinitions/deploymentColumnDefinitions`分别声明了 Pod/Deployment 信息输出时的列格式, 最终要调用的函数分别为`printPod/printDeployment`.

这两个函数最终也是使用`fmt.Sprintf()`完成格式化打印的, 但是我没找到CRD注册这个`Handler`的示例(deployment是在[pkg/registry/apps/deployment/storage/storage.go](https://github.com/kubernetes/kubernetes/blob/v1.16.2/pkg/registry/apps/deployment/storage/storage.go)文件中注册的, 而且形式还是`TableConvertor`), 所以目前还是先使用一个字段存储这种信息好了.
