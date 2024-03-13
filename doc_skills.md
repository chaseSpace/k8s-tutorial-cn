# K8s小技巧汇总

## 获取当前版本支持的API列表

使用如下命令：

```shell
# 等价于 kubectl get --raw /apis，但后者获取的是JSON内容
# - 添加 -o wide 可返回API的verbs
# - 结果不含API的子资源，如 pods/log
$ kubectl api-resources

NAME                              SHORTNAMES   APIVERSION                             NAMESPACED   KIND
bindings                                       v1                                     true         Binding
componentstatuses                 cs           v1                                     false        ComponentStatus
configmaps                        cm           v1                                     true         ConfigMap
endpoints                         ep           v1                                     true         Endpoints
...
rolebindings                                   rbac.authorization.k8s.io/v1           true         RoleBinding
roles                                          rbac.authorization.k8s.io/v1           true         Role
priorityclasses                   pc           scheduling.k8s.io/v1                   false        PriorityClass
csidrivers                                     storage.k8s.io/v1                      false        CSIDriver
csinodes                                       storage.k8s.io/v1                      false        CSINode
csistoragecapacities                           storage.k8s.io/v1                      true         CSIStorageCapacity
storageclasses                    sc           storage.k8s.io/v1                      false        StorageClass
volumeattachments                              storage.k8s.io/v1                      false        VolumeAttachment
```

返回结果中包含某些API资源的缩写。还有一些命令：

```shell
kubectl api-versions
```

查看某个API的子资源以及详情：

```shell
kubectl get --raw="/api/v1" # 注意/api/v1指的是v1
kubectl get --raw="/apis/storage.k8s.io/v1" # 除了v1组，查看其他API都需要加上`/apis`的前缀
```

## 查看/解释某个资源支持的所有配置字段

```shell
$ kubectl explain pods                
KIND:     Pod
VERSION:  v1

DESCRIPTION:
     Pod is a collection of containers that can run on a host. This resource is
     created by clients and scheduled onto hosts.

FIELDS:
   apiVersion   <string>
     APIVersion defines the versioned schema of this representation of an
     object. Servers should convert recognized schemas to the latest internal
     value, and may reject unrecognized values. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources

   kind <string>
     Kind is a string value representing the REST resource this object
     represents. Servers may infer this from the endpoint the client submits
     requests to. Cannot be updated. In CamelCase. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds

   metadata     <Object>
     Standard object's metadata. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata

   spec <Object>
     Specification of the desired behavior of the pod. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status

   status       <Object>
     Most recently observed status of the pod. This data may not be up to date.
     Populated by the system. Read-only. More info:
     https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
```

其他：

```shell
kubectl explain pods.spec.containers
kubectl explain pods --recursive
```

## 使用nsenter在宿主机上调试容器

很多容器为了轻量化移除了很多基础命令，比如`ps`，`top`，`netstat`等。此时可以在宿主机上使用`nsenter`工具来调试容器。
通过nsenter可以使用宿主机上已安装的工具来调试容器，比如查看指定容器的防火墙规则，进程/CPU信息等。
Centos 7.9 默认安装了`nsenter`工具，如果没有安装，可以使用`yum install -y util-linux`命令自行安装，其他发行版请自行查找安装方式。

- [nsenter介绍](https://www.cnblogs.com/liugp/p/16344594.html)

调试步骤：

```shell
# 1. 获取容器ID
# - 下面的pod内含5个容器，根据名字找到任意一个容器的Container ID（除了init容器）
# - 因为Pod内的容器共享各命名空间，所以任一容器ID均可
$ kk describe po istio-client-test |grep erd -B 1
  istio-init:
    Container ID:  containerd://1c35ebc2cbf03f2d7069594cae68db9698d666174141c80bfcd5fca15bb600cb
--
  istio-client-test:
    Container ID:   containerd://e305d7f3fa5f9699e04586e87600adc40d2948271f5719af301a010034c2ca46
--
  iptables:
    Container ID:  containerd://ad016ed6d79010854d12aaeb27b352d38d3459811e2039c6dcc472b268c1dbe0
--
  tcpdump:
    Container ID:  containerd://5f993b223b5159db7efddf289db9f255c90833771ce3b47036a1c042b858fd38
--
  istio-proxy:
    Container ID:  containerd://e1752cafdd3de5ce83debb377d62b4ab8314f896fedb738917397699dd412cb5

# 2. 进入容器所运行的宿主机上查看容器在宿主机上的进程ID，如下是42816
$ crictl inspect e305d7f3fa5f9699e04586e87600adc40d2948271f5719af301a010034c2ca46|grep pid
    "pid": 42816,
            "pid": 1
            "type": "pid"

# 3. 使用nsenter命令查看容器内的iptables nat规则（此容器安装了envoy代理，所以会有包含15001端口的规则）
$ nsenter -t 42816 -n iptables -t nat -S                           
-P PREROUTING ACCEPT
-P INPUT ACCEPT
-P OUTPUT ACCEPT
-P POSTROUTING ACCEPT
-N ISTIO_INBOUND
-N ISTIO_IN_REDIRECT
-N ISTIO_OUTPUT
-N ISTIO_REDIRECT
-A PREROUTING -p tcp -j ISTIO_INBOUND
-A OUTPUT -p tcp -j ISTIO_OUTPUT
-A ISTIO_INBOUND -p tcp -m tcp --dport 15008 -j RETURN
-A ISTIO_INBOUND -p tcp -m tcp --dport 15090 -j RETURN
-A ISTIO_INBOUND -p tcp -m tcp --dport 15021 -j RETURN
-A ISTIO_INBOUND -p tcp -m tcp --dport 15020 -j RETURN
-A ISTIO_INBOUND -p tcp -j ISTIO_IN_REDIRECT
-A ISTIO_IN_REDIRECT -p tcp -j REDIRECT --to-ports 15006
-A ISTIO_OUTPUT -s 127.0.0.6/32 -o lo -j RETURN
-A ISTIO_OUTPUT ! -d 127.0.0.1/32 -o lo -p tcp -m tcp ! --dport 15008 -m owner --uid-owner 1337 -j ISTIO_IN_REDIRECT
-A ISTIO_OUTPUT -o lo -m owner ! --uid-owner 1337 -j RETURN
-A ISTIO_OUTPUT -m owner --uid-owner 1337 -j RETURN
-A ISTIO_OUTPUT ! -d 127.0.0.1/32 -o lo -p tcp -m tcp ! --dport 15008 -m owner --gid-owner 1337 -j ISTIO_IN_REDIRECT
-A ISTIO_OUTPUT -o lo -m owner ! --gid-owner 1337 -j RETURN
-A ISTIO_OUTPUT -m owner --gid-owner 1337 -j RETURN
-A ISTIO_OUTPUT -d 127.0.0.1/32 -j RETURN
-A ISTIO_OUTPUT -j ISTIO_REDIRECT
-A ISTIO_REDIRECT -p tcp -j REDIRECT --to-ports 15001
```