# 使用kubeadm搭建k8s多节点集群

**目录**
<!-- TOC -->
* [使用kubeadm搭建k8s多节点集群](#使用kubeadm搭建k8s多节点集群)
  * [1. 准备资源](#1-准备资源)
  * [2. 安装容器运行时](#2-安装容器运行时)
    * [2.1 Linux支持的CRI的端点](#21-linux支持的cri的端点)
    * [2.2 安装Containerd](#22-安装containerd)
  * [3. 安装三大件](#3-安装三大件)
  * [4. 为kubelet和runtime配置相同的cgroup driver](#4-为kubelet和runtime配置相同的cgroup-driver)
  * [5. 使用kubeadmin创建集群](#5-使用kubeadmin创建集群)
    * [5.1 在master上初始化集群](#51-在master上初始化集群)
    * [5.2 准备用户的 k8s 配置文件](#52-准备用户的-k8s-配置文件)
    * [5.3 其他节点加入集群](#53-其他节点加入集群)
    * [5.4 集群就绪验证](#54-集群就绪验证)
    * [5.5 安装第三方网络插件](#55-安装第三方网络插件)
  * [6. 验证集群](#6-验证集群)
<!-- TOC -->

## 1. 准备资源

```
10.0.2.2 k8s-master
10.0.2.3 k8s-node1
```
两台机，最低配置如下：
- cpu: 2c+
- mem: 2g+
- disk: 20g+
- network: 同属一个子网

> 在实战中，master节点配置通常是较低配，不需要较多cpu核心和内存，因为k8s不会调度pod到master上运行。
> 它的角色非常重要，在master上运行pod可能导致节点资源被耗尽进而导致集群不可用。但如果将node节点从集群中全部删除，
> 则pod会自动调度到master上。
>
> master的主要任务是作为管理者的角色来调度集群内的各项资源到其他工作节点上。

## 2. 安装容器运行时

k8s使用 Container Runtime Interface（CRI）来连接你选择的runtime。

### 2.1 Linux支持的CRI的端点

| Runtime                           | Path to Unix domain socket                 |
|-----------------------------------|--------------------------------------------|
| containerd                        | unix:///var/run/containerd/containerd.sock |
| CRI-O                             | unix:///var/run/crio/crio.sock             |
| Docker Engine (using cri-dockerd) | unix:///var/run/cri-dockerd.sock           |

### 2.2 安装Containerd

kubernetes 1.24.x及以后版本默认CRI为containerd，cri称之为容器运行时插件。其中ctr是containerd自带的CLI命令行工具，
crictl是k8s中CRI（容器运行时接口）的客户端，k8s使用该客户端和containerd进行交互。

在所有机器上运行：

```shell
# - 安装依赖
yum install -y yum-utils device-mapper-persistent-data lvm2
# - 设置源
yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
yum-config-manager --add-repo http://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
yum install containerd -y

containerd  --version
# - 创建或修改配置
vi /etc/containerd/config.toml  # 直接使用此文档同级位置的 containerd.config.toml 覆盖即可

systemctl daemon-reload
systemctl enable containerd # 开机启动
systemctl restart containerd
systemctl status containerd
```

## 3. 安装三大件

即 kubeadm、kubelet 和 kubectl

```shell
# 设置阿里云为源
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=http://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
gpgkey=http://mirrors.aliyun.com/kubernetes/yum/doc/yum-key.gpg
       http://mirrors.aliyun.com/kubernetes/yum/doc/rpm-package-key.gpg
EOF

# ubuntu
apt-get update && apt-get install -y apt-transport-https

curl https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | apt-key add - 

cat <<EOF > /etc/apt/sources.list.d/kubernetes.list
deb https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main
EOF

apt-get update
# 2023-10-15 已经出了1.28
apt-get install -y kubelet=1.25.14-00 kubeadm=1.25.14-00 kubectl=1.25.14-00
# 查看软件仓库包含哪些版本 apt-cache madison kubelet
# 删除 apt-get remove  -y kubelet kubeadm kubectl

# 检查版本
kubelet --version
kubeadm version -o json
kubectl version -o json

# centos 安装各组件
sudo yum install -y kubelet-1.25.14 kubeadm-1.25.14 kubectl-1.25.14 --disableexcludes=kubernetes

# 开机启动，且立即启动
sudo systemctl enable --now kubelet

# 配置容器运行时，以便后续通过crictl管理 集群内的容器和镜像
crictl config runtime-endpoint unix:///var/run/containerd/containerd.sock

# 准备工具
sudo yum install wget -y
```

## 4. 为kubelet和runtime配置相同的cgroup driver

Container runtimes推荐使用`systemd`作为kubeadm的driver，而不是kubelet默认的`cgroupfs`driver。

从k8s v1.22起，kubeadm默认使用`systemd`作为cgroupDriver。

https://kubernetes.io/docs/tasks/administer-cluster/kubeadm/configure-cgroup-driver/

所以使用高于v1.22的版本，这步就不用配置。

## 5. 使用kubeadmin创建集群

下面的命令需要在所有机器上执行。

设置hosts

```shell
cat <<EOF >> /etc/hosts
10.0.2.2 k8s-master
10.0.2.3 k8s-node1
EOF
```

设置每台机器的hostname

```shell
# 在master节点执行
hostnamectl set-hostname k8s-master

# 在node1节点执行
hostnamectl set-hostname k8s-node1
```

logout后再登录可见。


```shell
# 关闭swap：
swapoff -a # 临时关闭
sed -ri 's/.*swap.*/#&/' /etc/fstab  #永久关闭

# 关闭selinux
sudo setenforce 0
sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config

# 关闭防火墙
iptables -F
iptables -X
systemctl stop firewalld.service
systemctl disable firewalld
```

设置sysctl

```shell
cat > /etc/sysctl.conf << EOF
vm.swappiness=0
vm.overcommit_memory=1
vm.panic_on_oom=0
fs.inotify.max_user_watches=89100
EOF
sysctl -p # 生效

cat <<EOF | tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF

cat <<EOF | tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.conf.default.rp_filter=1
net.ipv4.conf.all.rp_filter=1
EOF

sysctl --system # 生效

# 加载内核模块
modprobe br_netfilter  # 网络桥接模块
modprobe overlay # 联合文件系统模块
lsmod | grep -e br_netfilter -e overlay
```

### 5.1 在master上初始化集群

```shell
# 提前拉取需要的image
kubeadm config images pull --image-repository registry.aliyuncs.com/google_containers

# 查看拉取的镜像
$ crictl images                                                            
IMAGE                                                             TAG                 IMAGE ID            SIZE
registry.aliyuncs.com/google_containers/coredns                   v1.9.3              5185b96f0becf       14.8MB
registry.aliyuncs.com/google_containers/etcd                      3.5.6-0             fce326961ae2d       103MB
registry.aliyuncs.com/google_containers/kube-apiserver            v1.25.14            48f6f02f2e904       35.1MB
registry.aliyuncs.com/google_containers/kube-controller-manager   v1.25.14            2fdc9124e4ab3       31.9MB
registry.aliyuncs.com/google_containers/kube-proxy                v1.25.14            b2d7e01cd611a       20.5MB
registry.aliyuncs.com/google_containers/kube-scheduler            v1.25.14            62a4b43588914       16.2MB
registry.aliyuncs.com/google_containers/pause                     3.8                 4873874c08efc       311kB
registry.cn-hangzhou.aliyuncs.com/google_containers/pause         3.6                 6270bb605e12e       302kB

# 初始化集群
# --apiserver-advertise-address 指定 Kubernetes API Server 的宣告地址。这是 Master 节点上的 Kubernetes API Server 的网络地址，
# 其他节点和客户端将使用此地址连接到 API Server
# --image-repository 指定了 Docker 镜像的仓库地址，用于下载 Kubernetes 组件所需的容器镜像。在这里，使用了阿里云容器镜像地址，可以加速镜像的下载。
#    注意：即使提取拉取了镜像，这里也要指定相同的仓库，否则还是会拉取官方镜像
# --service-cidr 指定 Kubernetes 集群中 Service 的 IP 地址范围，Service IP 地址将分配给 Kubernetes Service，以允许它们在集群内通信
# --pod-network-cidr 指定 Kubernetes 集群中 Pod 网络的 IP 地址范围。Pod IP 地址将分配给容器化的应用程序 Pod，以便它们可以相互通信。
$ kubeadm init \
--apiserver-advertise-address=10.0.2.2 \
--image-repository registry.aliyuncs.com/google_containers \
--kubernetes-version v1.25.14 \
--service-cidr=20.1.0.0/16 \
--pod-network-cidr=20.2.0.0/16

[init] Using Kubernetes version: v1.25.14
[preflight] Running pre-flight checks
[preflight] Pulling images required for setting up a Kubernetes cluster
[preflight] This might take a minute or two, depending on the speed of your internet connection
[preflight] You can also perform this action in beforehand using 'kubeadm config images pull'
... 日志较长，建立复制保存这段日志，留作以后维护查看组件配置信息使用




# 配置文件生效
# 一台机器只需执行一次
echo export KUBECONFIG=/etc/kubernetes/admin.conf >> /etc/profile
source /etc/profile
```

[k8s-cluster-init.log](k8s-cluster-init.log) 是一个k8s集群初始化日志实例。

后面如果想要删除集群，在所有节点执行:

```shell
rm -rf /var/lib/kubelet # 删除核心组件目录
rm -rf /etc/kubernetes # 删除集群配置 
rm -rf /etc/cni/net.d # 删除容器网络配置
rm -rf /var/log/pods && rm -rf /var/log/containers # 删除pod和容器日志
kubeadm reset -f
# 镜像我们仍然保留，/var/lib/containerd
# crictl images 
reboot
# 然后可能需要重启master，否则其他节点无法加入新集群
```

k8s组件的日志文件位置（当集群故障时查看）：
```shell
$ ls /var/log/containers/
etcd-k8s-master_kube-system_etcd-64d58a06aaf9417d406fd335f26eec0f8c51ed9d10e3713c3b553977e4bc6b6e.log@                                      
kube-apiserver-k8s-master_kube-system_kube-apiserver-f773c1b3959f7c9a1a25618a5dee2a36752e4f0b8a618902e9eedcdfba075cb5.log@                  
kube-controller-manager-k8s-master_kube-system_kube-controller-manager-7ea76156361ce5a837fd7ac9e56afee04904f24c2e95f15950d2ac6347061370.log@
kube-proxy-l9s4z_kube-system_kube-proxy-a25215f976bfab8762c877bd5ce90fdfe7f53c1c197887badb0ece5b0e13b683.log@                               
kube-scheduler-k8s-master_kube-system_kube-scheduler-55ce404921d20a4a05c7dea80987969f3a141eeaf6d22d234df11cc32365e120.log@ 
```

### 5.2 准备用户的 k8s 配置文件

**若是root用户，请忽略这一步**。

以便用户可以使用 kubectl 工具与 Kubernetes 集群进行通信。

```shell
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
```

查看节点状态：

```shell
[root@k8s-master calico]# kubectl get nodes
NAME         STATUS   ROLES           AGE   VERSION
k8s-master   NotReady   control-plane   7m14s   v1.25.14

[root@k8s-master calico]# kubectl cluster-info

Kubernetes control plane is running at https://10.0.2.2:6443
CoreDNS is running at https://10.0.2.2:6443/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy

To further debug and diagnose cluster problems, use 'kubectl cluster-info dump'.

```

这里由于还未安装pod网络插件，所以是NotReady，后面步骤解决。

### 5.3 其他节点加入集群

```shell
# 在node1上执行
# 注意使用初始化集群时输出的命令，确认token和sha正确
# 若忘记这个参数，在master执行：kubeadm token create --print-join-command 获取
$ kubeadm join 10.0.2.2:6443 --token ihde1u.chb9igowre1btgpt --discovery-token-ca-cert-hash sha256:fcbe96b444325ab7c854feeae7014097b6840329a608415b08c3af8e8e513573
[preflight] Running pre-flight checks
[preflight] Reading configuration from the cluster...
[preflight] FYI: You can look at this config file with 'kubectl -n kube-system get cm kubeadm-config -o yaml'
[kubelet-start] Writing kubelet configuration to file "/var/lib/kubelet/config.yaml"
[kubelet-start] Writing kubelet environment file with flags to file "/var/lib/kubelet/kubeadm-flags.env"
[kubelet-start] Starting the kubelet
[kubelet-start] Waiting for the kubelet to perform the TLS Bootstrap...

This node has joined the cluster:
* Certificate signing request was sent to apiserver and a response was received.
* The Kubelet was informed of the new secure connection details.

Run 'kubectl get nodes' on the control-plane to see this node join the cluster.
```

### 5.4 集群就绪验证

```shell
[root@k8s-master ~]# kubectl get nodes
NAME         STATUS     ROLES           AGE     VERSION
k8s-master   NotReady   control-plane   3m48s   v1.25.14
k8s-node1    NotReady   <none>          6s      v1.25.14
```

下面解决状态是NotReady的问题。

### 5.5 安装第三方网络插件

Kubernetes 需要网络插件(Container Network Interface: CNI)来提供集群内部和集群外部的网络通信。以下是一些常用的 k8s 网络插件：

```
Flannel：Flannel 是最常用的 k8s 网络插件之一，它使用了虚拟网络技术来实现容器之间的通信，支持多种网络后端，如 VXLAN、UDP 和 Host-GW。
Calico：Calico 是一种基于 BGP 的网络插件，它使用路由表来路由容器之间的流量，支持多种网络拓扑结构，并提供了安全性和网络策略功能。
Canal：Canal 是一个组合了 Flannel 和 Calico 的网络插件，它使用 Flannel 来提供容器之间的通信，同时使用 Calico 来提供网络策略和安全性功能。
Weave Net：Weave Net 是一种轻量级的网络插件，它使用虚拟网络技术来为容器提供 IP 地址，并支持多种网络后端，如 VXLAN、UDP 和 TCP/IP，同时还提供了网络策略和安全性功能。
Cilium：Cilium 是一种基于 eBPF (Extended Berkeley Packet Filter) 技术的网络插件，它使用 Linux 内核的动态插件来提供网络功能，如路由、负载均衡、安全性和网络策略等。
Contiv：Contiv 是一种基于 SDN 技术的网络插件，它提供了多种网络功能，如虚拟网络、网络隔离、负载均衡和安全策略等。
Antrea：Antrea 是一种基于 OVS (Open vSwitch) 技术的网络插件，它提供了容器之间的通信、网络策略和安全性等功能，还支持多种网络拓扑结构。

作者：冰_点
链接：https://juejin.cn/post/7236182358817800251
来源：稀土掘金
```

这里选择calico，安装步骤如下：

```shell
mkdir -p ~/k8s/calico && cd ~/k8s/calico

# 注意calico版本需要匹配k8s版本，否则无法应用
wget --no-check-certificate  https://raw.gitmirror.com/projectcalico/calico/v3.26.1/manifests/calico.yaml

# 修改calico.yaml，在 CALICO_IPV4POOL_CIDR 的位置，修改value为pod网段：20.2.0.0/16 (默认：192.168.0.0/16)

# 应用配置文件
# - 这将自动在Kubernetes集群中创建所有必需的资源，包括DaemonSet、Deployment和Service等
kubectl apply -f calico.yaml

# 观察calico 的几个 pod是否 running，这可能需要几分钟
[root@k8s-master calico]# kubectl get pods -n kube-system --watch
NAME                                       READY   STATUS              RESTARTS      AGE
calico-kube-controllers-74cfc9ffcc-85ng7   0/1     Pending             0             17s
calico-node-bsqtv                          0/1     Init:ErrImagePull   0             17s
calico-node-xjwt8                          0/1     Init:ErrImagePull   0             17s
...

# 观察到calico镜像拉取失败，查看pod日志
kubectl describe pod -n kube-system calico-node-bsqtv
# 从输出中可观察到是拉取 docker.io/calico/cni:v3.26.1 镜像失败，改为手动拉取（在所有节点都执行）
ctr image pull docker.io/calico/cni:v3.26.1
ctr image pull docker.io/calico/node:v3.26.1
ctr image pull docker.io/calico/kube-controllers:v3.26.1

# 检查
$ ctr image ls

# 删除calico pod，让其重启
kk delete pod -l k8s-app=calico-node -n kube-system
kk delete pod -l k8s-app=calico-kube-controllers -n kube-system

# 观察pod状态
kk get pods -A --watch

# ok后，重启一下网络（笔者出现集群正常后，无法连通外网，重启后可以）
service network restart

# 当需要重置网络时，在master节点删除calico全部资源，再重新配置
kubectl delete -f calico.yaml && rm -rf /etc/cni/net.d
service kubelet restart
# 当需要重置网络时，在其他节点：
rm -rf /etc/cni/net.d && service kubelet restart
```

安装calicoctl，方便观察calico的各种信息和状态：

```shell
# 第1种安装方式（推荐）
curl -o /usr/local/bin/calicoctl -O -L  "https://hub.gitmirror.com/https://github.com/projectcalico/calico/releases/download/v3.26.1/calicoctl-linux-amd64" 
chmod +x /usr/local/bin/calicoctl
# calicoctl 常用命令
calicoctl node status
calicoctl get nodes

# 第2种安装方式
curl -o /usr/local/bin/kubectl-calico -O -L  "https://hub.gitmirror.com/https://github.com/projectcalico/calico/releases/download/v3.26.1/calicoctl-linux-amd64" 
chmod +x /usr/local/bin/kubectl-calico
kubectl calico -h

# 检查Calico的状态
[root@k8s-master calico]# kubectl calico node status
Calico process is running.

IPv4 BGP status
+--------------+-------------------+-------+----------+-------------+
| PEER ADDRESS |     PEER TYPE     | STATE |  SINCE   |    INFO     |
+--------------+-------------------+-------+----------+-------------+
| 10.0.2.3     | node-to-node mesh | up    | 13:26:06 | Established |
+--------------+-------------------+-------+----------+-------------+

IPv6 BGP status
No IPv6 peers found.

# 列出Kubernetes集群中所有节点的状态，包括它们的名称、IP地址和状态等
[root@k8s-master calico]# kubectl calico get nodes
NAME         
k8s-master   
k8s-node1  
```

现在再次查看集群状态，一切OK。

```shell
[root@k8s-master ~]# kubectl get nodes
NAME         STATUS   ROLES           AGE   VERSION
k8s-master   Ready    control-plane   64m   v1.25.14
k8s-node1    Ready    <none>          61m   v1.25.14
```

## 6. 验证集群

这一节通过在集群中快速部署nginx服务来验证集群是否正常工作。

在master上执行下面的命令：

```shell
# 创建pod
kubectl create deployment nginx --image=nginx

#  添加nginx service，设置映射端口
# 如果是临时测试：kubectl port-forward deployment nginx 3000:3000
kubectl expose deployment nginx --port=80 --type=NodePort

# 查看pod，svc状态
$ kubectl get pod,svc
NAME                        READY   STATUS    RESTARTS   AGE
pod/nginx-76d6c9b8c-g5lrr   1/1     Running   0          7m12s

NAME                 TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)        AGE
service/kubernetes   ClusterIP   20.1.0.1      <none>        443/TCP        94m
service/nginx        NodePort    20.1.255.52   <none>        80:30447/TCP   7m
```

测试在所有集群机器上的pod连通性（在master上执行）：

```shell
$ curl http://10.0.2.2:30447
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
...


$ curl http://10.0.2.3:30447
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
...
```

删除部署

```shell
kubectl delete deployment nginx 
kubectl delete svc nginx
```

是的，nginx服务在所有集群上的暴露端口都是30447。

至此，使用Kubeadmin搭建结束。但是还有一些进阶话题需要讨论，比如k8s镜像清理、日志存储等，参考下一篇文档。