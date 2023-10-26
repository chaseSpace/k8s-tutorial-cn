## 使用minikube安装k8s单节点集群

<!-- TOC -->
  * [使用minikube安装k8s单节点集群](#使用minikube安装k8s单节点集群)
    * [0. 安装最新docker](#0-安装最新docker)
    * [1.安装启动minikube](#1安装启动minikube)
    * [2.安装kubectl](#2安装kubectl)
    * [3.minikube的镜像管理](#3minikube的镜像管理)
<!-- TOC -->

**环境准备**：

```
- 一台机
    - OS: Centos v7.9
    - Mem: 2c2g+
    - Disk: 20g+
```

**参考资料：**

- [Docker教程](https://yeasy.gitbook.io/docker_practice/)
- [国内Centos机器安装clash代理](../../pure_doc/use_clash_linux.md)

### 0. 安装最新docker

- [Centos升级/安装docker](https://www.cnblogs.com/wdliu/p/10194332.html)
    - 注意换国内源

### 1.安装启动minikube

安装

```shell
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 
sudo install minikube-linux-amd64 /usr/local/bin/minikube
```

启动（minikube要求较新的docker版本）

```shell
# --force允许在root执行
# --image-mirror-country='cn' 是加速minikube自身资源的下载安装
minikube start --force --image-mirror-country=cn
```
如果多次在`Pulling base image...`未知出错，需要清理缓存：`minikube delete --all --purge`，再重新下载。


其他启动参数：
```
--driver=*** 从1.5.0版本开始，Minikube缺省使用系统优选的驱动来创建Kubernetes本地环境，比如您已经安装过Docker环境，minikube 将使用 docker 驱动
--cpus=2: 为minikube虚拟机分配CPU核数
--memory=2048mb: 为minikube虚拟机分配内存数
--registry-mirror=*** 为了提升拉取Docker Hub镜像的稳定性，可以为 Docker daemon 配置镜像加速，参考阿里云镜像服务
--kubernetes-version=***: minikube 虚拟机将使用的 kubernetes 版本
```

查看启动状态：

```shell
$ minikube status
minikube
type: Control Plane
host: Running
kubelet: Running
apiserver: Running
kubeconfig: Configured
```

minikube命令速查：

```shell
minikube 命令速查

minikube stop 不会删除任何数据，只是停止 VM 和 k8s 集群。

minikube delete 删除所有 minikube 启动后的数据。

minikube ip 查看集群和 docker enginer 运行的 IP 地址。

minikube pause 暂停当前的资源和 k8s 集群

minikube status 查看当前集群状态
```

### 2.安装kubectl

由于minikube下载kubectl命令太慢，所以笔者自行下载kubectl。

>如备有代理，可参考前面**参考资料**中的文档连接代理后再直接下载kubectl

先导入源

```shell
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64/
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://mirrors.aliyun.com/kubernetes/yum/doc/yum-key.gpg https://mirrors.aliyun.com/kubernetes/yum/doc/rpm-package-key.gpg
EOF
```

再配置和安装最新版本的k8s组件

```shell
setenforce 0
yum install -y kubelet kubeadm kubectl
systemctl enable kubelet && systemctl start kubelet
```

> 安装指定版本  
> `yum install -y kubelet-<version> kubectl-<version> kubeadm-<version>`

### 3.minikube的镜像管理
当我们启动pod时，引用的镜像会从远程拉取到本地，存入minikube自身的本地镜像库中管理，而不是由docker管理。
```shell
# alias m='minikube'
root@VM-0-13-centos ~/install_k8s » m image -h
管理 images

Available Commands:
  build         在 minikube 中构建一个容器镜像
  load          将镜像加载到 minikube 中
  ls            列出镜像
  pull          拉取镜像
  push          推送镜像
  rm            移除一个或多个镜像
  save          从 minikube 中保存一个镜像
  tag           为镜像打标签

Use "minikube <command> --help" for more information about a given command.
root@VM-0-13-centos ~/install_k8s » m image ls
registry.k8s.io/pause:3.9
registry.k8s.io/kube-scheduler:v1.27.3
registry.k8s.io/kube-proxy:v1.27.3
registry.k8s.io/kube-controller-manager:v1.27.3
registry.k8s.io/kube-apiserver:v1.27.3
registry.k8s.io/etcd:3.5.7-0
registry.k8s.io/coredns/coredns:v1.10.1
gcr.io/k8s-minikube/storage-provisioner:v5
docker.io/leigg/hellok8s:v2   <----------------
docker.io/leigg/hellok8s:v1   <----------------
```
也就是说，`docker rmi`删除的镜像是不会影响minikube的镜像库的。即使通过`m image rm`删除了本地的一个minikube管理的镜像，
再启动deployment，也可以启动的，因为minikube会去远程镜像库Pull，除非远程仓库也删除了这个镜像。
重新启动后，可通过`m image ls`再次看到被删除的镜像又出现了。