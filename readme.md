## 适用场景

实际的业务场景中, 有时会希望项目更新时, 重建的Pod的IP地址保持不变. 

比如

1. Pod中的业务将日志输出到宿主机, 又使用ELK系统收集这些日志. 日志中可能会打印Pod的IP信息, 但是业务每次更新, 日志的中的IP信息就会发生变动, 在查询时就会很混乱.
2. 业务之间相互调用, 有些业务域要求提供调用方白名单. 还有些业务域会需要线上的数据访问, 要加入相应的防火墙权限等.

## 实现功能

由CNI插件在ipam阶段通过`restful API`调用, 读取当前生成的Pod资源文件中的注解, 设置该Pod的IP.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: devops
  annotations:
    ipkeeper.generals.space/ip_address: 172.16.91.181/24
    ipkeeper.generals.space/gateway: 172.16.91.2
spec:
  containers:
  - name: devops
    image: registry.cn-hangzhou.aliyuncs.com/generals-space/centos7:devops
    command: ["tail", "-f", "/etc/os-release"]
    securityContext:
      privileged: false
      capabilities:
        add: ["NET_ADMIN"]
```

```console
$ k get pod -o wide
NAME                                    READY   STATUS    RESTARTS   AGE     IP              NODE            NOMINATED NODE   READINESS GATES
coredns-67c766df46-cxjc2                0/1     Running   20         4h25m   172.16.91.161   k8s-master-01   <none>           <none>
coredns-67c766df46-q5g8x                0/1     Running   20         4h24m   172.16.91.162   k8s-master-01   <none>           <none>
devops                                  1/1     Running   0          4h26m   172.16.91.181   k8s-master-01   <none>           <none>
```

同时可通过查询`StaticIP`资源获取静态IP的分配情况.

```console
$ k get sip
NAME                   OWNERKIND    NAMESPACE     IPPOOL
deploy-devops-deploy   Deployment   kube-system   172.16.91.182/24,172.16.91.183/24
pod-devops             Pod          kube-system   172.16.91.181/24
```

## 关于

本工程可以称为CNI插件的插件, 因为ta的工作时机就是在`kubelet`在创建pause容器完成, 调用CNI插件为其申请IP时实现功能的. 

本质上是一类`ipam`插件, 但同时ta又需要拥有`kube client`的权限, 因为ta需要读取`Pod`/`Deployment`/`DaemonSet`资源文件中的相关注解, 所以又不同于普通的`ipam`.

灵感来源于灵雀云的[kube-ovn](https://github.com/alauda/kube-ovn)项目, 借鉴了很多.

------

此插件需要对集群中的`CNI`插件做修改, 添加调用过程. 

目前只支持我自己编写的CNI插件[cni-terway](https://github.com/generals-space/cni-terway), 之后会考虑fork&modify一下flannel.
