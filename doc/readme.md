一些想法

对于固定IP的实现, 我曾经有如下几个想法.

## Patch

我的第一个想法是, 既然在创建Pod时无法指定IP, 那是否可以尝试在更新Pod的保持已经获得的IP不变? 

由CRD捕获更新操作, 不移除原有的pause容器, 而是将业务container移除并新建一个更新镜像的container, 并且仍然使用原来的pause容器网络. 

我尝试过使用`kubectl patch`命令修改一个正在运行的Pod的`image`字段, 以模拟业务更新操作. 能够看到的确是只更换了业务container, 而pause没有变, 因而IP也没有变.

这可能需要借助Pod的Patch接口, 不能确定是否能实现.

## 修改kubernetes源码

修改kubernetes源码是肯定可行的, 需要在`PodSpec`结构体中添加一个`IP`字段, 唯品会就是这么做的.

但是这需要对kubernetes进行深度定制, 工作量非常大. 而且为了要跟上官方的更新, 需要付出极大的精力, 代价很大.

## 自定义IPKPod

`Pod`本身是kuber的底层对象, `controller-manager`的组件都是在Pod层面之上进行操作的, 常规CRD也一般也很少直接操作`Pod`.

有考虑过自定义`IPKPod`类型的资源, 同时把`kubelet`中所有与`Pod`相关的操作(比如oom检测, 存活检测, 挂载configmap, volume, 以及资源限制等等)全都拷贝出来, 否则实现的功能定然不会全面.

...但这就相当于重写了`kubelet`组件了, 至少重写了大部分功能.

能不能越过`kubelet`直接调用`kubelet`下的`docker`接口创建`IPKPod`, 但是仍然注册为普通的Pod对象, 并且仍由kubelet管理?

但是CRD只能拥有`clientset`的权限, 权限声明最多细致到`Pod`级别, 应该没有办法权限直接操作`container`.

## CNI

kubelet在创建pause容器完成后, 调用`CNI`的`ipam`插件获取IP, 同时在`Pod`删除时也会释放其拥有的IP. 除非同时修改CNI, 否则在Pod的移除与创建过程中无法保证地址不变.

但问题是具体的实现策略无法确定, 我不知道怎么申请IP并告知`kubelet`.

## 进入Pod手动修改IP

使用exec进入pod后可以使用`ip`命令对容器IP进行修改且目标IP有效(需要容器本身拥有`NET_ADMIN`的权限), 且的确可以生效.

但是在使用`kubectl get pod -o wide`时, 打印出来的还是容器最初获得的IP. 

所以修改容器IP只能在`pause`容器通过`CNI`接口申请IP时完成, 否则查询有缺陷(尝试过使用`kubectl edit`修改目标Pod的status字段, 无效). 

