## `kube-ovn v0.1.0`

v0.1.0只实现了Pod资源的静态IP, 还没有实现 deployment, daemonset 中的静态资源池.

具体见[CHANGELOG.md]()

整个工程分为3个可执行文件: 

1. controller: 通过调用ovs工具, 构建集群网络, 创建iptables规则, dns表, 设置路由等.
2. cni: 实现了`cmdAdd()/cmdDel()`接口的cni插件, 每次`kubelet`在创建完`pause`容器后就会调用此cni插件. 在`ipam`期间, 通过`unix socket`调用`dameon`服务, 为当前Pod的IP申请IP.
3. daemon: 可以说是一个`ipam`插件, 接收到来自`cni`的请求后, 会通过`kube client`向`apiserver`查询当前Pod中的注解是否存在`ovn.kubernetes.io/ip_address`字段, 如果是则返回其中指定的IP.

