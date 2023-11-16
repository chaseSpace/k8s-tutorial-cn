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
        * [5.4 安装第三方网络插件](#54-安装第三方网络插件)
        * [5.5 在普通节点执行kubectl](#55-在普通节点执行kubectl)
        * [5.6 删除集群](#56-删除集群)
    * [6. 验证集群](#6-验证集群)
    * [7. 故障解决](#7-故障解决)
        * [7.1 解决calico镜像下载较慢的问题](#71-解决calico镜像下载较慢的问题)
        * [7.2 解决calico密钥过期问题](#72-解决calico密钥过期问题)
        * [7.3 解决k8s证书过期问题](#73-解决k8s证书过期问题)

<!-- TOC -->

为了提高命令行使用效率，建议先 [安装ohmyzsh](../doc_install_ohmyzsh.md)。

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

在实战中，master节点配置通常是**中高配置**（如4c8g，8c16g），虽然k8s不会调度pod到master上运行，但由于Master是整个 Kubernetes
集群的核心部分，负责协调、管理和调度所有工作负载。并且Master节点上运行着各种关键组件（如
etcd、kube-apiserver、kube-controller-manager 和 kube-scheduler），这些组件都需要处理大量的网络流量和计算任务。

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
systemctl enable containerd # 开机启动

systemctl daemon-reload
systemctl restart containerd
systemctl status containerd
```

## 3. 安装三大件

即 kubeadm、kubelet 和 kubectl

在centos上安装：

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

# centos 安装各组件
sudo yum install -y wget lsof net-tools \
    kubelet-1.25.14 kubeadm-1.25.14 kubectl-1.25.14 --disableexcludes=kubernetes

# 开机启动，且立即启动
sudo systemctl enable --now kubelet

# 检查版本
kubeadm version
kubelet --version
kubectl version

# 配置容器运行时，以便后续通过crictl管理 集群内的容器和镜像
crictl config runtime-endpoint unix:///var/run/containerd/containerd.sock
```

在ubuntu上安装：

```shell
apt-get update && apt-get install -y apt-transport-https

curl https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | apt-key add - 

# 设置阿里源
cat <<EOF > /etc/apt/sources.list.d/kubernetes.list
deb https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main
EOF

apt-get update
apt-get install -y kubelet=1.25.14-00 kubeadm=1.25.14-00 kubectl=1.25.14-00
# 查看软件仓库包含哪些版本 apt-cache madison kubelet
# 删除 apt-get remove  -y kubelet kubeadm kubectl
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
# 建议主机ip与教程一致
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
# --apiserver-advertise-address 指定 Kubernetes API Server 的宣告地址，可以不设置让其自动检测
# 其他节点和客户端将使用此地址连接到 API Server
# --image-repository 指定了 Docker 镜像的仓库地址，用于下载 Kubernetes 组件所需的容器镜像。在这里，使用了阿里云容器镜像地址，可以加速镜像的下载。
#    注意：即使提取拉取了镜像，这里也要指定相同的仓库，否则还是会拉取官方镜像
# --service-cidr 指定 Kubernetes 集群中 Service 的 IP 地址范围，Service IP 地址将分配给 Kubernetes Service，以允许它们在集群内通信
# --pod-network-cidr 指定 Kubernetes 集群中 Pod 网络的 IP 地址范围。Pod IP 地址将分配给容器化的应用程序 Pod，以便它们可以相互通信。
$ kubeadm init \
--image-repository registry.aliyuncs.com/google_containers \
--kubernetes-version v1.25.14 \
--service-cidr=20.1.0.0/16 \
--pod-network-cidr=20.2.0.0/16

[init] Using Kubernetes version: v1.25.14
[preflight] Running pre-flight checks
[preflight] Pulling images required for setting up a Kubernetes cluster
[preflight] This might take a minute or two, depending on the speed of your internet connection
[preflight] You can also perform this action in beforehand using 'kubeadm config images pull'
... 省略
```

[k8s-cluster-init.log](k8s-cluster-init.log) 是一个k8s集群初始化日志实例。

注意暂存日志输出的最后部分：

```shell
...
kubeadm join 10.0.2.2:6443 --token 4iwa6j.ejrsfqm26jpcshz2 \
	--discovery-token-ca-cert-hash sha256:f8fa90012cd3bcb34f3198b5b6184dc45104534f998ee601ed97c39f2efa8b05
```

这是一条普通节点加入集群的命令。其中包含一个临时token和证书hash，如果忘记或想要查询，分别可以通过以下方式查看：

```shell
# 1. 查看token
$ kubeadm token list
TOKEN                     TTL         EXPIRES                USAGES                   DESCRIPTION                                                EXTRA GROUPS
4iwa6j.ejrsfqm26jpcshz2   23h         2023-11-13T08:32:40Z   authentication,signing   The default bootstrap token generated by 'kubeadm init'.   system:bootstrappers:kubeadm:default-node-token

# 2. 查看cert-hash
$ openssl x509 -in /etc/kubernetes/pki/ca.crt -pubkey -noout |
pipe> openssl pkey -pubin -outform DER |
pipe pipe> openssl dgst -sha256
(stdin)= f8fa90012cd3bcb34f3198b5b6184dc45104534f998ee601ed97c39f2efa8b05
```

token默认有效期24h。在过期后还想要加入集群的话，我们需要手动创建一个新token：

```shell
# cert-hash一般不会改变
$ kubeadm token create --print-join-command                       
kubeadm join 10.0.2.2:6443 --token eczspu.kjrxrem8xv5x7oqm --discovery-token-ca-cert-hash sha256:f8fa90012cd3bcb34f3198b5b6184dc45104534f998ee601ed97c39f2efa8b05
$ kubeadm token list                       
TOKEN                     TTL         EXPIRES                USAGES                   DESCRIPTION                                                EXTRA GROUPS
4iwa6j.ejrsfqm26jpcshz2   23h         2023-11-14T08:25:28Z   authentication,signing   <none>                                                     system:bootstrappers:kubeadm:default-node-token
eczspu.kjrxrem8xv5x7oqm   23h         2023-11-14T08:32:40Z   authentication,signing   The default bootstrap token generated by 'kubeadm init'.   system:bootstrappers:kubeadm:default-node-token 
```

### 5.2 准备用户的 k8s 配置文件

以便用户可以使用 kubectl 工具与 Kubernetes 集群进行通信，下面的操作只需要在**master节点**执行一次。

若是root用户，执行：

```shell
echo 'export KUBECONFIG=/etc/kubernetes/admin.conf' >> /etc/profile
source /etc/profile
```

不是root用户，执行：

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
# 注意使用初始化集群时输出的命令，确认token和cert-hash正确
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

然后在master上查看节点状态：

```shell
[root@k8s-master ~]# kubectl get nodes
NAME         STATUS     ROLES           AGE     VERSION
k8s-master   NotReady   control-plane   3m48s   v1.25.14
k8s-node1    NotReady   <none>          6s      v1.25.14
```

下节解决节点状态是`NotReady`的问题。

### 5.4 安装第三方网络插件

Kubernetes 需要网络插件(Container Network Interface: CNI)来提供集群内部和集群外部的网络通信。以下是一些常用的 k8s 网络插件：

- Flannel：Flannel 是最常用的 k8s 网络插件之一，它使用了虚拟网络技术来实现容器之间的通信，支持多种网络后端，如 VXLAN、UDP 和
  Host-GW。
- Calico：Calico 是一种基于 BGP 的网络插件，它使用路由表来路由容器之间的流量，支持多种网络拓扑结构，并提供了安全性和网络策略功能。
- Canal：Canal 是一个组合了 Flannel 和 Calico 的网络插件，它使用 Flannel 来提供容器之间的通信，同时使用 Calico
  来提供网络策略和安全性功能。
- Weave Net：Weave Net 是一种轻量级的网络插件，它使用虚拟网络技术来为容器提供 IP 地址，并支持多种网络后端，如 VXLAN、UDP 和
  TCP/IP，同时还提供了网络策略和安全性功能。
- Cilium：Cilium 是一种基于 eBPF (Extended Berkeley Packet Filter) 技术的网络插件，它使用 Linux
  内核的动态插件来提供网络功能，如路由、负载均衡、安全性和网络策略等。
- Contiv：Contiv 是一种基于 SDN 技术的网络插件，它提供了多种网络功能，如虚拟网络、网络隔离、负载均衡和安全策略等。
- Antrea：Antrea 是一种基于 OVS (Open vSwitch) 技术的网络插件，它提供了容器之间的通信、网络策略和安全性等功能，还支持多种网络拓扑结构。

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
ctr image pull docker.io/calico/cni:v3.26.1 && \
ctr image pull docker.io/calico/node:v3.26.1 && \
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

安装calicoctl（也可暂时不用安装），方便观察calico的各种信息和状态：

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

### 5.5 在普通节点执行kubectl

默认情况下，我们只能在master上运行kubectl命令，如果在普通节点执行会得到以下错误提示：

```shell
[root@k8s-node1 ~]# kubectl get nodes
The connection to the server localhost:8080 was refused - did you specify the right host or port?
```

kubectl命令默认连接本地的8080端口，需要修改配置文件，指向master的6443端口。当然，除了连接地址外还需要证书完成认证。
这里可以直接将master节点的配置文件拷贝到普通节点：

```shell
#  拷贝配置文件到node1，输入节点密码
[root@k8s-master ~]# scp /etc/kubernetes/admin.conf root@k8s-node1:/etc/kubernetes/

# 在节点配置环境变量后即可使用
[root@k8s-node1 ~]# echo 'export KUBECONFIG=/etc/kubernetes/admin.conf' >> /etc/profile
[root@k8s-node1 ~]# source /etc/profile
[root@k8s-node1 ~]# kubectl get nodes
NAME         STATUS   ROLES           AGE   VERSION
k8s-master   Ready    control-plane   17h   v1.25.14
k8s-node1    Ready    <none>          16h   v1.25.14
```

但是，在实际环境中，我们通常不需要做这个操作。因为普通节点相对master节点只是一种临时资源，可能会以后某个时间点退出集群。
而且`/etc/kubernetes/admin.conf`是一个包含证书密钥的敏感文件，不应该存在于普通节点上。

### 5.6 删除集群

后面如果想要彻底删除集群，在所有节点执行:

```shell
kubeadm reset # 重置集群  -f 强制执行

rm -rf /var/lib/kubelet/* # 删除核心组件目录
rm -rf /etc/kubernetes/* # 删除集群配置 
rm -rf /etc/cni/net.d/* # 删除容器网络配置
rm -rf /var/log/pods && rm -rf /var/log/containers # 删除pod和容器日志
service kubelet restart
# 镜像一般保留，查看当前节点已下载的镜像命令如下
crictl images
# 快速删除节点上的全部镜像
# rm -rf /var/lib/containerd/*
# 然后可能需要重启节点才能再次加入集群
reboot
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

上面通过NodePort类型的Service来暴露了Pod，它将容器80端口映射到所有节点的一个随机端口（这里是30447）。
然后我们可以通过访问节点端口来测试在所有集群机器上的pod连通性：

```shell
# 在master上执行
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

至此，使用kubeadm搭建集群结束。但是还有一些进阶话题需要讨论，比如k8s镜像清理、日志存储等，参考下一篇文档。

## 7. 故障解决

### 7.1 解决calico镜像下载较慢的问题

镜像下载慢会导致节点一直停留在`NotReady`状态，可以通过手动拉取的方式解决：

```shell
$ cat calico.yaml|grep image:
          image: docker.io/calico/cni:v3.26.1
          image: docker.io/calico/cni:v3.26.1
          image: docker.io/calico/node:v3.26.1
          image: docker.io/calico/node:v3.26.1
          image: docker.io/calico/kube-controllers:v3.26.1
# 一个个手动拉取上面的三个镜像（需要在所有节点执行）
$ ctr image pull docker.io/calico/cni:v3.26.1
$ ctr image pull docker.io/calico/node:v3.26.1
$ ctr image pull docker.io/calico/kube-controllers:v3.26.1
```

因为之前安装containerd时在其配置文件中添加了国内源，所以这里直接使用ctr手动拉取的速度很快。

在后续测试过程中，你也可以使用这个方式来解决国外镜像下载慢的问题。对于生产环境，通常是使用本地镜像仓库，一般不会有这个问题。

> k8s默认使用`crictl pull <image-name>`命令拉取镜像，但crictl读取不到containerd设置的国内源，所以才会慢。

### 7.2 解决calico密钥过期问题

每个新创建的Pod都需要calico分配IP，如果calico无法分配IP，就会导致Pod启动异常:

```shell
$ kk describe po hellok8s-go-http-999f66c56-4k72x
...
Events:
  Type     Reason                  Age   From               Message
  ----     ------                  ----  ----               -------
  Warning  FailedCreatePodSandBox  3d6h  kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "603ebf87036af6c05e6bf26c82403b404cc9763b5d20ab89cd08286969899348": plugin type="calico" failed (add): error getting ClusterInformation: connection is unauthorized: Unauthorized
```

这里的`connection is unauthorized: Unauthorized`其实是calico的日志，根本原因是calico用来查询集群信息的ServiceAccount
Token过期了。
calico使用的token存储在`/etc/cni/net.d/calico-kubeconfig`，通过cat可以查看。这个token的有效期只有24h，
但不知为何calico没有自动续期导致Pod无法正常创建和删除（对应分配和释放IP操作）。

一个快速解决的办法是删除`calico-node`Pod，这样它在重建`calico-node`Pod后会生成新的token：

```shell
$ kk delete po -l k8s-app=calico-node -A                             
pod "calico-node-v94sd" deleted
pod "calico-node-xzxbd" deleted
```

再次观察Pod状态就会正常了。

### 7.3 解决k8s证书过期问题

默认情况下kubernetes集群各个组件的证书有效期是一年，这可以通过以下命令查看：

```shell
$ kubeadm certs check-expiration                                   
[check-expiration] Reading configuration from the cluster...
[check-expiration] FYI: You can look at this config file with 'kubectl -n kube-system get cm kubeadm-config -o yaml'

CERTIFICATE                EXPIRES                  RESIDUAL TIME   CERTIFICATE AUTHORITY   EXTERNALLY MANAGED
admin.conf                 Nov 15, 2024 08:33 UTC   317d            ca                      no      
apiserver                  Nov 15, 2024 08:33 UTC   317d            ca                      no      
apiserver-etcd-client      Nov 15, 2024 08:33 UTC   317d            etcd-ca                 no      
apiserver-kubelet-client   Nov 15, 2024 08:33 UTC   317d            ca                      no      
controller-manager.conf    Nov 15, 2024 08:33 UTC   317d            ca                      no      
etcd-healthcheck-client    Nov 15, 2024 08:33 UTC   317d            etcd-ca                 no      
etcd-peer                  Nov 15, 2024 08:33 UTC   317d            etcd-ca                 no      
etcd-server                Nov 15, 2024 08:33 UTC   317d            etcd-ca                 no      
front-proxy-client         Nov 15, 2024 08:33 UTC   317d            front-proxy-ca          no      
scheduler.conf             Nov 15, 2024 08:33 UTC   317d            ca                      no      

CERTIFICATE AUTHORITY   EXPIRES                  RESIDUAL TIME   EXTERNALLY MANAGED
ca                      Nov 13, 2033 08:33 UTC   9y              no      
etcd-ca                 Nov 13, 2033 08:33 UTC   9y              no      
front-proxy-ca          Nov 13, 2033 08:33 UTC   9y              no  
```

当证书过期后，执行kubectl命令会得到证书过期的提示，导致无法管理集群。通过以下命令进行证书更新：

```shell
# 首先备份旧证书
$ cp -r /etc/kubernetes/ /tmp/backup/
$ ls /tmp/backup      
admin.conf  controller-manager.conf  kubelet.conf  manifests  pki  scheduler.conf

# 对单个组件证书续期（一年）
$ kubeadm certs renew apiserver
[renew] Reading configuration from the cluster...
[renew] FYI: You can look at this config file with 'kubectl -n kube-system get cm kubeadm-config -o yaml'

certificate for serving the Kubernetes API renewed
$ kubeadm certs check-expiration |grep apiserver 
apiserver                  Jan 01, 2025 16:17 UTC   364d            ca                      no      
apiserver-etcd-client      Nov 15, 2024 08:33 UTC   317d            etcd-ca                 no      
apiserver-kubelet-client   Nov 15, 2024 08:33 UTC   317d            ca                      no  

# 或者直接对全部组件证书续期
$ kubeadm certs renew all                        
[renew] Reading configuration from the cluster...
[renew] FYI: You can look at this config file with 'kubectl -n kube-system get cm kubeadm-config -o yaml'

certificate embedded in the kubeconfig file for the admin to use and for kubeadm itself renewed
certificate for serving the Kubernetes API renewed
certificate the apiserver uses to access etcd renewed
certificate for the API server to connect to kubelet renewed
certificate embedded in the kubeconfig file for the controller manager to use renewed
certificate for liveness probes to healthcheck etcd renewed
certificate for etcd nodes to communicate with each other renewed
certificate for serving etcd renewed
certificate for the front proxy client renewed
certificate embedded in the kubeconfig file for the scheduler manager to use renewed

Done renewing certificates. You must restart the kube-apiserver, kube-controller-manager, kube-scheduler and etcd, so that they can use the new certificates.

# 重启kubelet
systemctl restart kubelet

# 如果不是root用户
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
```

## 参考

- [掘金-冰_点-Kubernetes 之7大CNI 网络插件用法和对比](https://juejin.cn/post/7236182358817800251)