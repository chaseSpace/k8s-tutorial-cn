# Kubernetes 小技巧汇总

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

还可以直接进入该容器的命名空间调试，免去输入`nsenter`命令前缀：

```shell
$ nsenter -t 42816 -n
$ iptables -t nat -S
...

# 退出该容器的命名空间
$ exit
```

## 调试Pod

首先，我们在定义负载（无论是Deployment还是裸Pod）时，没有必要在Pod内添加仅用于调试的容器，这样会无端消耗节点的CPU/内存资源。
K8s提供了`kubectl debug`命令，可以快速创建一个临时容器以进入Pod的命名空间，并执行调试命令，下面是操作步骤：

以 [pod.yaml](pod.yaml) 为例。

```shell
$ kk apply -f pod.yaml

# 源容器不支持iptables命令
$ kk exec -it go-http -- iptables -L
error: Internal error occurred: error executing command in container: failed to exec in container: failed to start exec "4eed822c2c4f81e34710bfde16f47063d641acbe352811c15b045583e404a217": OCI runtime exec failed: exec failed: unable to start container process: exec: "iptables": executable file not found in $PATH: unknown

# 使用专用容器进行调试, --target命令表示共享目标容器的进程空间（就能看到目标容器内运行的进程，可选参数）
# --profile=netadmin表示赋予网络管理权限给临时容器（该容器需要），否则无法启动。其他容器一般不需要设置此参数
$ kk debug go-http -it --image vimagick/iptables --profile=netadmin --target=go-http -- sh  
Targeting container "go-http". If you don't see processes from this container it may be because the container runtime doesn't support this feature.
Defaulting debug container name to debugger-vw2jz.
If you don't see a command prompt, try pressing enter.
/ # ps
PID   USER     TIME  COMMAND
    1 root      0:00 /main  # <------- 目标容器运行的主进程
   19 root      0:00 sh
   24 root      0:00 ps

# Pod内的网络空间是多容器共享的，所以可直接查看它们共享的iptables规则
/ # iptables -t nat -S
-P PREROUTING ACCEPT
-P INPUT ACCEPT
-P OUTPUT ACCEPT
...

# 退出临时容器
# exit
```

但如果目标容器已经崩溃，上面的方法就失效了。需要采用Pod副本的方式来调试，步骤如下：

```shell
# 修改pod.yaml如下
spec:
  containers:
    - name: go-http
      image: leigg/hellok8s:v1
      command: ["xxx"]  # 添加此行，xxx是一个无效的程序，所以Pod无法正常启动
      
$ kk apply -f pod.yaml

$ kk get po go-http        
NAME      READY   STATUS              RESTARTS      AGE
go-http   1/2     RunContainerError   2 (25h ago)   22s

# exec命令无法进入容器shell
$ kk exec -it go-http -- sh 
error: unable to upgrade connection: container not found ("go-http")

# 第一种方式也失败
$ kk debug go-http -it --image vimagick/iptables --profile=netadmin --target=go-http -- sh
Targeting container "go-http". If you don't see processes from this container it may be because the container runtime doesn't support this feature.
Defaulting debug container name to debugger-djmsn.
Warning: container debugger-djmsn: failed to generate container "5c8a0a7cbea5a941e5c31bdb8f5646f9b9c44eac5c60d577e059308ad43e61d2" spec: invalid target container: 
  container "efe86231f87668855b285305fe7b628fa9401e79c3272cac4673bfed97498f50" is not running - in state CONTAINER_EXITED
  
# 现在使用创建Pod副本的方式
# - 此时就可以进入目标容器shell，然后可以检查你的程序无法启动的原因（你可以将启动日志输出到某个文件以便定位）
$ kk debug go-http -it --image vimagick/iptables --profile=netadmin --share-processes --copy-to=debugger -- sh
Defaulting debug container name to debugger-45wx8.
If you don't see a command prompt, try pressing enter.
/ # ps
PID   USER     TIME  COMMAND
    1 65535     0:00 /pause
   26 root      0:00 sh
   31 1337      0:00 /usr/local/bin/pilot-agent proxy sidecar --domain default.svc.cluster.local --proxyLogLevel=warning --proxyComponentLogLevel=mis
   45 1337      0:00 /usr/local/bin/envoy -c etc/istio/proxy/envoy-rev.json --drain-time-s 45 --drain-strategy immediate --local-address-ip-version v
   63 root      0:00 ps

# 调试结束记得删除Pod副本
$ kk delete po debugger
```

其他还有一种使用方式，根据情况使用：

```shell
# 在创建Pod副本时，使用包含调试命令的镜像替换原来的镜像
kubectl debug myapp --copy-to=myapp-debug --set-image=*=busybox

# 直接使用一个临时容器连接到无法运行Pod的节点上进行调试（一般是推测因为节点原因导致Pod无法运行，才会使用此法）
# - 此时节点的文件系统将挂在到容器的 host/ 目录
# - 此容器与目标节点共享IPC空间、NET空间、PID空间，但没有特权
kubectl debug node/mynode -it --image=ubuntu
```