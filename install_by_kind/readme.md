## 使用kind安装配置集群

kind主要用于在本地机器上快速启动一个Kubernetes集群，由K8s官方开发设计，用于日常开发和测试（勿用于生产环境）。

本文参照[kind官方文档](https://kind.sigs.k8s.io/docs/user/quick-start/)，介绍如何使用kind安装配置Kubernetes集群。
笔者使用的机器是MacBookPro M1，所以演示的一些命令为macOS平台下的指令。

kind内部使用kubeadm来启动一个多节点集群，它使用自实现的`kindnetd`
作为容器网络接口（CNI）实现。更多设计细节参考[kind设计文档](https://kind.sigs.k8s.io/docs/design/initial)。

### 1. 安装kind

支持多种方式安装，笔者是macOS，所以使用Homebrew安装：

```shell
brew install kind
```

其他系统参考[二进制安装kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installing-from-release-binaries)。

### 2. 创建一个集群

kind使用一个构建好的节点镜像以容器的形式来启动一个集群（一个K8s单节点集群运行在一个容器中），镜像中包含了Kubernetes的关键组件，比如kubelet等。

> 节点镜像托管在[DockerHub](https://hub.docker.com/r/kindest/node/)
> ，查看它的[更多介绍](https://kind.sigs.k8s.io/docs/design/node-image)。

创建命令如下：

```shell
kind create cluster --image=kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72
```

如果不使用`--image`flag，则使用当前kind版本默认的节点镜像。但为了确定安装行为，请使用`--image`flag。
在[kind版本发布页](https://github.com/kubernetes-sigs/kind/releases)查找当前kind版本的预构建的不同K8s版本的节点镜像tag。
如果没有你想要的K8s版本，参考[这个页面](https://kind.sigs.k8s.io/docs/user/quick-start/#building-images)自行构建节点镜像。

创建后使用下面的命令简单管理集群：

```shell
kind get clusters
kind delete cluster -n <name>
```

当然，以上只是以最简化的集群配置方式启动。kind支持通过yaml文件来详细配置集群的启动参数，[kind-config.yaml](kind-config.yaml)
是一个包含注释的1主2Worker集群的完整kind配置示例。使用配置文件启动集群：

```shell
# 注意内存占用，主节点占用约500MB，Worker节点占用约200MB，总共占用约1GB。确保你的宿主机内存充足
kind create cluster --config=kind-config.yaml --retain --wait=1m
```

其中的`--retain`flag表示在kind命令执行结束后保留节点容器，否则集群会自动删除。保留的目的是方便在启动集群失败时进入容器查看错误日志：

```shell
journalctl -xeu kubectl
```

`--wait=1m`是等待控制节点Ready的最长等待时间。下文将使用这个集群进行演示。

#### 1.1 安装kubectl

kind创建集群后，我们需要在本机上安装kubectl来连接并管理集群。
如果你的机器已存在kubectl但版本与安装的k8s版本不同，可通过以下方式卸载：

```shell
which kubectl
sudo rm <path>
```

安装kubectl v1.27.3版本：

```shell
curl -LO "https://dl.k8s.io/release/v1.27.3/bin/darwin/arm64/kubectl"
sudo mv kubectl /usr/local/bin
sudo chmod +x /usr/local/bin/kubectl
kubectl version
```

该命令安装arm64架构的kubectl，其他架构请参考[kubectl安装文档](https://kubernetes.io/docs/tasks/tools)。

#### 1.2 连接集群

kind在创建集群后会自动在本机的`$HOME/.kube/config`处配置好kubeconfig文件。此时我们已经可以使用kubectl来连接并管理集群了。

```shell
kubectl cluster-info

# 查看宿主机上的节点容器列表
docker ps |grep test-1.27

# 查看使用的容器运行时（containerd）
docker exec -it test-1.27-control-plane crictl info|grep runtimeType
```

不过有个问题得注意一下，containerd默认的镜像仓库地址是`docker.io`，后续使用K8s拉取远端镜像会特别慢，
你可以参考 [containerd.config.toml](../install_by_kubeadm/containerd.config.toml)
来修改每个节点容器中的containerd配置文件（位于`/etc/containerd/config.toml`，搜索关键字`registry.mirrors`），
修改后需要重启containerd（`service containerd restart`）。

当然还有另一种办法，那就是直接使用宿主机的docker镜像，后面的3.1节会介绍如何操作。

#### 1.4 创建并管理多个集群

前面已经介绍了可以通过`kind get clusters`看到当前kind安装的K8s集群列表。这就告诉了我们kind可以同时安装多个集群。
安装多集群后，可以切换context来连接不同的集群。

```shell
# 安装两个集群
$ kind create cluster
$ kind create cluster --name kind-2

# 查看集群列表
$ kind get clusters
kind
kind-2

# 切换context（默认是kind-kind）
$ kubectl cluster-info --context kind-kind
$ kubectl cluster-info --context kind-kind-2
```

### 3. 部署应用

#### 3.1 添加镜像到节点容器

如果我们想要将宿主机上已存在的docker镜像直接导入节点容器（免去重复拉取），参照下面的命令：

```shell
# 需要先将镜像拉取到宿主机
docker pull busybox

# 再进行load操作
kind load docker-image busybox -n test-1.27 

# 也可以从tar包导入
kind load image-archive /my-image-archive.tar

# 导入镜像到特定节点容器（默认所有）
kind load docker-image <image1> --nodes <node-name>
```

Load之后，查看节点容器中的镜像：

```shell
$ docker exec -it test-1.27-control-plane crictl images
IMAGE                                      TAG                  IMAGE ID            SIZE
docker.io/kindest/kindnetd                 v20230511-dc714da8   b18bf71b941ba       25.3MB
docker.io/kindest/local-path-helper        v20230510-486859a6   d022557af8b63       2.92MB
docker.io/kindest/local-path-provisioner   v20230511-dc714da8   eec7db0a07d0d       17.3MB
docker.io/library/busybox                  latest               23466caa55cb7       4.27MB
registry.k8s.io/coredns/coredns            v1.10.1              97e04611ad434       14.6MB
registry.k8s.io/etcd                       3.5.7-0              24bc64e911039       80.7MB
registry.k8s.io/kube-apiserver             v1.27.3              634c53edb5c14       79.8MB
registry.k8s.io/kube-controller-manager    v1.27.3              aea4f169db16d       71.5MB
registry.k8s.io/kube-proxy                 v1.27.3              278dd40f83dfb       68.1MB
registry.k8s.io/kube-scheduler             v1.27.3              6234a065dec4c       57.6MB
registry.k8s.io/pause                      3.7                  e5a475a038057       268kB
```

#### 3.2 部署Pod

注意，我们前面已经在宿主机上安装了kubectl，所以现在可以直接在宿主机上管理集群，而不需要进入节点容器。

下面以清单 [pod_busybox.yaml](../pod_busybox.yaml) 为例进行部署演示。

```shell
$ kubectl apply -f pod_busybox.yaml

# 清单中使用的镜像是上一节中导入的镜像，所以Pod应该很快Running
$ kubectl get po                               
NAME      READY   STATUS    RESTARTS   AGE
busybox   1/1     Running   0          36s
```

再部署一个可通过宿主机访问的应用 [deployment_python_http_svc_nodeport.yaml](../deployment_python_http_svc_nodeport.yaml)：

```shell
$ docker pull python:3.9-alpine
$ kind load docker-image python:3.9-alpine -n test-1.27

$ kubectl apply -f deployment_python_http_svc_nodeport.yaml

$ kubectl get po
NAME                                READY   STATUS        RESTARTS      AGE
busybox                             1/1     Running       1 (15m ago)   19m
python-http-serv-6b874b4bdf-wtfhl   1/1     Running       0             5s

# 访问宿主机80端口（映射到控制平面节点的30080端口），可以看到一个HTML输出，其中包含Pod内容器的根目录下的文件列表
# 推荐使用浏览器访问
$ curl http://localhost/
```

## 结尾

笔者在查看kind官方文档的时候，发现kind缺少一个可能比较关键的功能，那就是在kind配置文件限制节点容器的CPU/Memory额度。遗憾的是，
笔者看到了kind仓库中的这个[ISSUE #1422](https://github.com/kubernetes-sigs/kind/issues/1422)
，也就是说kind截止目前（2024-1-2）也没有支持这个功能。

---

以上就是使用kind在MacOS上安装一个多节点集群的过程，其他操作系统的安装过程也是大差不差，具体可以看kind官文。
如果你有遇到问题请提出ISSUE，但也希望你能够先看一下官方kind文档。

## 参考

- [kind官方文档](https://kind.sigs.k8s.io/docs/user/quick-start/)