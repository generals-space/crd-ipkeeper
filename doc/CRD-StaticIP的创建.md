# CRD-StaticIP的创建

首先创建这些基础文件, 文件内容除了具体的结构体成员, group, version这些字段可以修改外, 其他的像`import`的内容, `+genclient`和`+k8s`这种编译标记, 都保持不变.

```
mkdir -p pkg/apis/ipkeeper/v1
touch pkg/apis/ipkeeper/v1/doc.go
touch pkg/apis/ipkeeper/v1/register.go
touch pkg/apis/ipkeeper/v1/types.go
```

在未进行下一步时, `register.go`中的`addKnownTypes()`, `&StaticIP{}`和`&StaticIPList{}`结构会出现红色下划线, 显示如下

```
cannot use &(StaticIPList literal) (value of type *StaticIPList) as runtime.Object value in argument to scheme.AddKnownTypes: missing method DeepCopyObject
```

接下来使用`code-generator`项目生成代码, 需要`code-generator`和`apimachinery`两个项目在`GOPATH`目录下.

```bash
mkdir -p $GOPATH/src/k8s.io
cd $GOPATH/src/k8s.io/

git clone https://github.com/kubernetes/code-generator.git
cd code-generator
git checkout -b v0.17.0 v0.17.0
go mod vendor
cd ..

git clone https://github.com/kubernetes/apimachinery.git
cd apimachinery
git checkout -b v0.17.0 v0.17.0
go mod vendor
cd ..
```

注意: `apimachinery`必须要在GOPATH目录下, 否则在生成过程中可能出现如下报错.

```
Generating deepcopy funcs
F0421 10:09:13.896524   30295 deepcopy.go:885] Hit an unsupported type invalid type for invalid type, from github.com/generals-space/crd-ipkeeper/pkg/apis/ipkeeper/v1.StaticIP
```

------

我们的项目也要在`GOPATH`目录下, 这里用软链接完成.

```console
$ mkdir -p $GOPATH/github.com/generals-space
$ ln -s crd-ipkeeper $GOPATH/github.com/generals-space
$ $GOPATH/src/k8s.io/code-generator/generate-groups.sh all github.com/generals-space/crd-ipkeeper/pkg/client github.com/generals-space/crd-ipkeeper/pkg/apis ipkeeper:v1
Generating deepcopy funcs
Generating clientset for ipkeeper:v1 at github.com/generals-space/crd-ipkeeper/pkg/client/clientset
Generating listers for ipkeeper:v1 at github.com/generals-space/crd-ipkeeper/pkg/client/listers
Generating informers for ipkeeper:v1 at github.com/generals-space/crd-ipkeeper/pkg/client/informers
```

生成完成后, 对`StaticIP{}`及`StaticIPList{}`的成员进行修改就不再需要重新生成了.
