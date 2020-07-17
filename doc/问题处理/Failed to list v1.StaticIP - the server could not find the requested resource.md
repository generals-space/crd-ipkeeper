# Failed to list v1.StaticIP: the server could not find the requested resource

把CRD类型的名称从`StaticIPs`改成`StaticIP`后(要把`client`目录删掉重新生成代码), 重新构建工程并部署, 但是一直报如下错误.

```
E0502 01:25:48.931070  119221 reflector.go:156] pkg/mod/k8s.io/client-go@v0.17.0/tools/cache/reflector.go:108: Failed to list *v1.StaticIP: the server could not find the requested resource (get staticips.ipkeeper.generals.space)
E0502 01:25:49.933466  119221 reflector.go:156] pkg/mod/k8s.io/client-go@v0.17.0/tools/cache/reflector.go:108: Failed to list *v1.StaticIP: the server could not find the requested resource (get staticips.ipkeeper.generals.space)
```

我已经确认yaml文件中的crd名称和rbac中的名称一致了, 一直没想到问题出在哪里. 而且报错信息直接就在`reflector.go`中, 没有具体的定位...

后来想到之前的`staticips`的crd对象可能还没有删除, 查看了一下, 果然.

```console
$ k get crd
NAME                                  CREATED AT
podgroups.testgroup.k8s.io            2020-04-20T08:30:44Z
staticips.ipkeeper.generals.space     2020-05-02T01:25:20Z
staticipses.ipkeeper.generals.space   2020-05-02T01:18:39Z
```

没想到删除`staticipses.ipkeeper.generals.space`竟然真的解决了...

