# Kubernetes 集群维护

在当今云原生时代，Kubernetes 已经成为部署、管理和扩展容器化应用程序的事实标准。随着企业对微服务和可伸缩架构的迅速采用，对
Kubernetes 集群的健壮性和可靠性的需求也日益增加。在这个背景下，对 Kubernetes 集群进行有效的维护变得至关重要，以确保业务连续性、性能优化和安全性。本文将深入探讨
Kubernetes 集群维护的关键方面，涵盖从备份和恢复、节点管理到安全性和性能优化等多个关键主题。


> 如果你在阅读本文时发现了任何错误，请在Github上提交ISSUE或PR，我将由衷地表示感谢。

为了方便阅读，请点击网页右侧的 ![toc.jpg](img/toc.jpg) 按钮在右侧展开目录以了解全文大纲。

## 1. 节点管理

在此章节中，本文将以 Kubernetes 集群中的节点管理为主题进行深入探讨。节点管理是保持集群健壮性和性能优越的关键要素之一。节点作为集群的基本构建单元，其合理配置和高效管理对于确保应用程序的可靠性至关重要。

### 1.1 增加节点

新节点需要满足以下要求：

- 确保即将加入集群的节点至少满足最低硬件配置要求
- 测试与master节点的网络通信正常

新节点需要安装与master节点一致的k8s版本（三大组件），通过`kk get nodes`可以查看现有集群节点的kubelet版本信息。
然后需要对新节点完成各项基础配置，如修改hosts、关闭swap和selinux等。

接下来需要安装容器运行时（Docker或Containerd），请与现有节点保持一致。

然后可以使用kubeadm工具在master节点获取加入集群的命令（包含token和hash），并在新节点上执行。获取加入命令的步骤如下：

```shell
$ kubeadm token create --print-join-command
kubeadm join <master-ip>:6443 --token <token-value> --discovery-token-ca-cert-hash sha256:<hash-value>
```

到这里就基本完成新节点的加入工作了。接下来，根据集群环境信息，你可能还需要为新节点添加特定的标签或污点以满足后续Pod的调度需求。

> 完整的操作步骤建议参考 [使用kubeadm搭建k8s多节点集群](./install_by_kubeadm/install.md) 。

### 1.2 删除节点

在对节点执行维护（例如内核升级、硬件维护等）之前，我们需要先从集群中删除这个节点。

在生产环境中删除节点是一项需要谨慎操作的任务，需要确保在删除节点之前，所有在该节点上运行的Pod都被重新调度到其他节点。
在开始之前，请通知集群的其他维护和使用人员即将此项任务，确保节点删除不会影响生产负载。

本节假定要删除的是普通节点（而非控制平面节点）。使用`kubectl drain`从节点安全地驱逐所有 Pod（这个步骤叫做清空节点）到其他节点。
安全的驱逐过程允许Pod的容器体面地终止。

**预留充足的节点资源**  
在生产环境中，你需要检查集群中剩余的节点资源能否足够接收被驱逐的Pod，如果剩余资源不足，可能会导致被驱逐的Pod无法被正常调度。
即使这样，也不会影响节点的清空操作。

**忽略某些Pod**  
在清空节点期间，如果创建了新的能够容忍`node.kubernetes.io/unschedulable`污点的 Pod，那么这些 Pod 仍然可能会被调度到你已经清空的节点上。
除了 DaemonSet 之外，请避免容忍此污点。另外，如果某个用户直接为 Pod 设置了`nodeName`字段，那Pod也会绑定到这个节点上，你需要妥善处理之后再进行清空操作。

下面使用实际环境进行演示：

- 当前集群环境中包含`k8s-master`和`k8s-node1`两个节点，在`k8s-node1`
  上运行了daemonset（calico-node和kube-proxy都是）、deployment、single-pod、stateful四种类型的Pod，尽可能模拟生产环境
- 现在准备从集群中删除节点`k8s-node1`，下面是实际操作情况

```shell
# 在master查看当前集群中运行的所有Pod信息（-A等价于--all-namespaces）
$ kk get po -o wide -A    
NAMESPACE     NAME                                           READY   STATUS    RESTARTS       AGE     IP             NODE         NOMINATED NODE   READINESS GATES
default       deployment-hellok8s-go-http-55cfd74847-gq5rd   1/1     Running   0              17s     20.2.36.70     k8s-node1    <none>           <none>
default       deployment-hellok8s-go-http-55cfd74847-m9zk6   1/1     Running   0              17s     20.2.235.236   k8s-master   <none>           <none>
default       go-http                                        1/1     Running   0              12s     20.2.36.72     k8s-node1    <none>           <none>
default       stateful-nginx-0                               1/1     Running   0              6s      20.2.36.74     k8s-node1    <none>           <none>
default       stateful-nginx-1                               1/1     Running   0              5s      20.2.235.237   k8s-master   <none>           <none>
kube-system   calico-kube-controllers-74cfc9ffcc-th8t5       1/1     Running   0              15m     20.2.235.229   k8s-master   <none>           <none>
kube-system   calico-node-pmt5d                              1/1     Running   0              39m     192.168.31.2   k8s-master   <none>           <none>
kube-system   calico-node-zdnxl                              1/1     Running   0              39m     192.168.31.3   k8s-node1    <none>           <none>
kube-system   coredns-c676cc86f-9rqm8                        1/1     Running   0              2m12s   20.2.235.234   k8s-master   <none>           <none>
kube-system   coredns-c676cc86f-l6rgv                        1/1     Running   0              2m12s   20.2.36.65     k8s-node1    <none>           <none>
kube-system   etcd-k8s-master                                1/1     Running   2 (3d2h ago)   16d     192.168.31.2   k8s-master   <none>           <none>
kube-system   kube-apiserver-k8s-master                      1/1     Running   3 (3d2h ago)   16d     192.168.31.2   k8s-master   <none>           <none>
kube-system   kube-controller-manager-k8s-master             1/1     Running   4 (3d2h ago)   16d     192.168.31.2   k8s-master   <none>           <none>
kube-system   kube-proxy-t9bt2                               1/1     Running   2 (12d ago)    16d     192.168.31.3   k8s-node1    <none>           <none>
kube-system   kube-proxy-zpzhn                               1/1     Running   2 (3d2h ago)   16d     192.168.31.2   k8s-master   <none>           <none>
kube-system   kube-scheduler-k8s-master                      1/1     Running   4 (3d2h ago)   16d     192.168.31.2   k8s-master   <none>           <none>


# 查看节点信息，确定待删除节点名称
$ kk get nodes     
NAME         STATUS   ROLES           AGE   VERSION
k8s-master   Ready    control-plane   15d   v1.25.14
k8s-node1    Ready    <none>          15d   v1.25.14

# 在master节点上执行清空pod操作（drain）
# - 根据输出得知:该节点已进入 cordoned（隔离）状态，不再接受新Pod调度
# - error提示无法清空节点上的Pod，因为存在一个无控制器的Pod名为default/go-http
# - pending表示节点的清空状态被挂起（等待手动处理）
$ kk drain --ignore-daemonsets k8s-node1
node/k8s-node1 cordoned
error: unable to drain node "k8s-node1" due to error:cannot delete Pods declare no controller (use --force to override): default/go-http, continuing command...
There are pending nodes to be drained:
 k8s-node1
cannot delete Pods declare no controller (use --force to override): default/go-http

# 观察k8s-node1状态已经发生变化
$ kk get nodes                                  
NAME         STATUS                     ROLES           AGE   VERSION
k8s-master   Ready                      control-plane   16d   v1.25.14
k8s-node1    Ready,SchedulingDisabled   <none>          16d   v1.25.14

# 因为default/go-http是不重要的Pod，所以可以执行强制清空
# - 根据输出得知：除了daemonset之外的pod都驱逐完成了（被驱逐的Pod在其他节点创建可能需要点时间）
# - 最后一条表示节点已清空，现在可以对节点进行维护工作（包括物理重启、升级版本等）
$ kk drain --ignore-daemonsets k8s-node1 --force
node/k8s-node1 already cordoned
Warning: ignoring DaemonSet-managed Pods: kube-system/calico-node-zdnxl, kube-system/kube-proxy-t9bt2; deleting Pods that declare no controller: default/go-http
evicting pod kube-system/coredns-c676cc86f-l6rgv
evicting pod default/deployment-hellok8s-go-http-55cfd74847-gq5rd
evicting pod default/go-http
evicting pod default/stateful-nginx-0
pod/deployment-hellok8s-go-http-55cfd74847-gq5rd evicted
pod/go-http evicted
pod/stateful-nginx-0 evicted
pod/coredns-c676cc86f-l6rgv evicted
node/k8s-node1 drained

# 此时观察集群中的Pod基本都运行在仅剩的master节点上，并且deployment和stateful pod的数量仍然符合预期
# calico-node和kube-proxy都是daemonset pod，可以忽略。
$ kk get po -A -o wide
NAMESPACE     NAME                                           READY   STATUS    RESTARTS       AGE   IP             NODE         NOMINATED NODE   READINESS GATES
default       deployment-hellok8s-go-http-55cfd74847-m9zk6   1/1     Running   0              19m   20.2.235.236   k8s-master   <none>           <none>
default       deployment-hellok8s-go-http-55cfd74847-w97gl   1/1     Running   0              16m   20.2.235.238   k8s-master   <none>           <none>
default       stateful-nginx-0                               1/1     Running   0              16m   20.2.235.240   k8s-master   <none>           <none>
default       stateful-nginx-1                               1/1     Running   0              18m   20.2.235.237   k8s-master   <none>           <none>
kube-system   calico-kube-controllers-74cfc9ffcc-th8t5       1/1     Running   0              34m   20.2.235.229   k8s-master   <none>           <none>
kube-system   calico-node-pmt5d                              1/1     Running   0              58m   192.168.31.2   k8s-master   <none>           <none>
kube-system   calico-node-zdnxl                              1/1     Running   0              58m   192.168.31.3   k8s-node1    <none>           <none>
kube-system   coredns-c676cc86f-7hgbd                        1/1     Running   0              16m   20.2.235.239   k8s-master   <none>           <none>
kube-system   coredns-c676cc86f-9rqm8                        1/1     Running   0              20m   20.2.235.234   k8s-master   <none>           <none>
kube-system   etcd-k8s-master                                1/1     Running   2 (3d3h ago)   16d   192.168.31.2   k8s-master   <none>           <none>
kube-system   kube-apiserver-k8s-master                      1/1     Running   3 (3d3h ago)   16d   192.168.31.2   k8s-master   <none>           <none>
kube-system   kube-controller-manager-k8s-master             1/1     Running   4 (3d3h ago)   16d   192.168.31.2   k8s-master   <none>           <none>
kube-system   kube-proxy-t9bt2                               1/1     Running   2 (12d ago)    16d   192.168.31.3   k8s-node1    <none>           <none>
kube-system   kube-proxy-zpzhn                               1/1     Running   2 (3d3h ago)   16d   192.168.31.2   k8s-master   <none>           <none>
kube-system   kube-scheduler-k8s-master                      1/1     Running   4 (3d3h ago)   16d   192.168.31.2   k8s-master   <none>           <none>
```

**完全删除节点**  
如果只是维护节点，比如对节点本身资源（CPU、内存或磁盘等）进行扩容，那就没有必要完全删除节点。在节点维护完成后，你可以参考下面的
**恢复调度** 来快速恢复节点到Ready状态。

删除节点使用下面的命令：

```shell
# 完全删除节点
$ kubectl delete node k8s-node1
node "k8s-node1" deleted
$ kubectl get nodes                                            
NAME         STATUS   ROLES           AGE   VERSION
k8s-master   Ready    control-plane   16d   v1.25.14

# 在k8s-node1上执行，重置节点以便下次加入集群, 添加-f忽略询问
[root@k8s-node1 ~]# kubeadm reset
[preflight] Running pre-flight checks
W1105 22:35:06.562494   23928 removeetcdmember.go:85] [reset] No kubeadm config, using etcd pod spec to get data directory
[reset] No etcd config found. Assuming external etcd
[reset] Please, manually reset etcd to prevent further issues
[reset] Stopping the kubelet service
[reset] Unmounting mounted directories in "/var/lib/kubelet"
[reset] Deleting contents of directories: [/etc/kubernetes/manifests /etc/kubernetes/pki]
[reset] Deleting files: [/etc/kubernetes/admin.conf /etc/kubernetes/kubelet.conf /etc/kubernetes/bootstrap-kubelet.conf /etc/kubernetes/controller-manager.conf /etc/kubernetes/scheduler.conf]
[reset] Deleting contents of stateful directories: [/var/lib/kubelet]

The reset process does not clean CNI configuration. To do so, you must remove /etc/cni/net.d

The reset process does not reset or clean up iptables rules or IPVS tables.
If you wish to reset iptables, you must do so manually by using the "iptables" command.

If your cluster was setup to utilize IPVS, run ipvsadm --clear (or similar)
to reset your system's IPVS tables.

The reset process does not clean your kubeconfig files and you must remove them manually.
Please, check the contents of the $HOME/.kube/config file.
```

如果你在没有清空节点的情况下执行了删除节点操作，操作会立即成功，节点立即从集群中消失，并且节点上运行的Pod也会在短暂时间后被kubelet组件删除（通过`crictl pods`
命令查看节点上运行的Pod列表）。
如果节点上的Pod没有被任何控制器（如Deployment）管理，那Pod将从集群中消失；如果是控制器管理的Pod，那将会被调度到其他节点，如果没有节点可以调度（如资源不足或无法容忍污点），则调度失败。

> 如果不小心执行了`kubectl delete node`命令，你只能在节点上执行`kubeadm reset`
> 命令来重置节点，然后重新加入集群。

**使用drain选项**  
`kubectl drain -h`查看排空节点时支持的选项，这里摘取部分选项进行说明：

- `--delete-emptydir-data=false`：驱逐Pod后删除Pod使用的空目录卷
- `--disable-eviction=false`: 强制使用删除方式进行驱逐。这将绕过对 PodDisruptionBudgets 的检查，请谨慎使用
- `--grace-period=-1`：指定 Pod 删除前的宽限期（覆盖Pod自己的参数），负数表示使用 Pod 中指定的默认值
- `--pod-selector=''`：通过标签筛选要驱逐的Pod，比如`--pod-selector=app=web`

**恢复调度**  
当`kubectl drain`命令完成后，此时节点在集群中进入了不可调度的状态，但还没有退出集群，通过`kubectl get nodes`
依然可以看到该节点。此时通过`kubectl uncordon <node name>`命令可以快速将节点恢复到**可调度状态**。但已经驱逐的Pod不会自动归位，
手动删除带有控制器管理的Pod可以触发对Pod的重新调度（此时节点相当于刚加入集群，资源充足，所以重新调度的Pod有很大概率会调度到该节点上）。

**并行清空多个节点**  
虽然 `kubectl drain` 命令一次只能发送给一个节点，但你可以开启多个终端同时执行 `kubectl drain` 命令，这样就可以并行清空多个节点。

**当应用受到PodDisruptionBudget（PDB）保护**  
PDB用于保障应用**在大部分时候**都能够拥有最低可用副本数量，而大部分时候包含了`kubectl drain`发起的驱逐要求这类情况。
当节点排空操作可能导致受PDB保护的应用违反PDB配置中的`minAvailable`或`maxUnavailable`字段要求时，API Server会阻止排空操作。

### 1.3 清理节点

**删除节点**（不是清空）后，你可能还需要删除一些节点上的容器缓存文件（**操作前请确认节点名称**）：

```shell
# 在已删除的节点上执行
rm -rf /var/lib/kubelet/* # 删除核心组件目录
rm -rf /etc/kubernetes/* # 删除集群配置 
rm -rf /etc/cni/net.d/* # 删除容器网络配置
rm -rf /var/log/pods && rm -rf /var/log/containers # 删除pod和容器日志
systemctl stop kubelet # 停止核心组件

# 清空所有iptables规则（k8s使用iptables来实现节点到Pod的网络通信）
sudo iptables -F
sudo iptables -t nat -F
sudo iptables -t mangle -F
sudo iptables -t raw -F

# 可能需要重启来彻底恢复某些系统组件的状态
reboot
```

如果你为集群节点配置了监控，你可能需要从监控系统中删除该节点的配置。

## 2. 镜像管理

镜像存放位置取决于集群采用的容器运行时`crictl`，它是一个用于与容器运行时 (CRI，Container Runtime Interface)
通信的命令行工具。CRI 是 Kubernetes 使用的标准，它定义了容器运行时和 Kubernetes kubelet 之间的接口，以便 kubelet
能够管理容器的生命周期。

首先配置`crictl`：

```shell
# 若是docker作为容器运行时
crictl config runtime-endpoint unix:///var/run/cri-dockerd.sock

# 若是containerd作为容器运行时
crictl config runtime-endpoint unix:///var/run/containerd/containerd.sock
```

### 2.1 精简镜像

精简镜像的策略有很多，比如使用Dockerfile的多阶段构建、使用精简镜像如busybox或alpine等。[Dockerfile](Dockerfile)
是一个使用精简镜像的Go应用的Dockerfile示例，使用它构建的镜像大小约为7MB。

### 2.2 查看镜像列表

```shell
# 在具体节点上执行
$ crictl images                                                            
IMAGE                                                             TAG                 IMAGE ID            SIZE
docker.io/calico/cni                                              v3.26.1             9dee260ef7f59       93.4MB
docker.io/calico/kube-controllers                                 v3.26.1             1919f2787fa70       32.8MB
docker.io/calico/node                                             v3.26.1             8065b798a4d67       86.6MB
registry.aliyuncs.com/google_containers/coredns                   v1.9.3              5185b96f0becf       14.8MB
registry.aliyuncs.com/google_containers/etcd                      3.5.6-0             fce326961ae2d       103MB
registry.aliyuncs.com/google_containers/kube-apiserver            v1.25.14            48f6f02f2e904       35.1MB
registry.aliyuncs.com/google_containers/kube-controller-manager   v1.25.14            2fdc9124e4ab3       31.9MB
registry.aliyuncs.com/google_containers/kube-proxy                v1.25.14            b2d7e01cd611a       20.5MB
registry.aliyuncs.com/google_containers/kube-scheduler            v1.25.14            62a4b43588914       16.2MB
registry.aliyuncs.com/google_containers/pause                     3.8                 4873874c08efc       311kB
registry.cn-hangzhou.aliyuncs.com/google_containers/pause         3.6                 6270bb605e12e       302kB
```

### 2.3 清理镜像

```shell
# 删除单个镜像
crictl rmi <image-id>
# 删除所有未被任何容器使用的镜像（用于释放磁盘空间）
crictl rmi --prune
```

在较老的k8s版本中，仍然使用docker作为默认的镜像管理工具，docker清理资源的命令如下：

```shell
# 查看可清理的资源（如镜像，容器，卷，cache）
$ docker system df
TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
Images          1         0         4.904MB   4.904MB (100%)
Containers      0         0         0B        0B
Local Volumes   0         0         0B        0B
Build Cache     58        0         622.9MB   622.9MB

# 此命令可以用于清理磁盘，删除关闭的容器、无用的镜像和网络
# 添加 -f 禁用询问
docker system prune
```

### 2.4 镜像更新

在更新应用Pod时，通常是先构建并推送新版本的镜像到仓库中，然后再使用`kubectl set image`命令更新Pod使用的镜像。
如果新的镜像存在问题，则使用`kubectl rollout undo`命令进行回滚（到上个镜像）。

> 建议在更新容器镜像时使用清晰的语义化的版本标签，例如`v1.1.2-fixPayModule`，以便在任何时候都能够通过镜像版本清晰的了解更新内容。
> 此外，还可以携带日期信息，例如`v1.1.2-20231010-fixPayModule`。如果仅仅使用`v1.1.2`
> 这样简洁的版本号，则无法知道更新内容，你必须在其他位置记录版本号对应的更新内容。

通常我们会将新镜像的构建、推送以及更新操作整合到持续集成/持续部署（CI/CD）流水线中，以自动化更新过程。

### 2.5 镜像存储

在实际生产环境中，我们通常会搭建本地镜像仓库，用于存储和分发镜像。这样做可能出于镜像私有、拉取速度等原因。
搭建本地镜像仓库的方式有很多，例如Harbor、Docker Registry等。Harbor是VMWare开源的企业级Docker镜像仓库管理项目，
也是目前主流的企业镜像仓库解决方案；而Docker Registry更适用于个人或小团队。

## 3. 使用Velero备份和恢复集群

介绍使用 [Velero](https://github.com/vmware-tanzu/velero/) 来完成（定期）备份集群和恢复集群。

使用前在其Github页面根据你的K8s版本选择Velero相应版本。

- [Velero工作原理](https://velero.io/docs/v1.12/how-velero-works/)

Velero支持按需备份、定时备份、恢复备份、设置备份过期等功能。
每个Velero操作如按需备份，计划备份，恢复都是一个自定义资源，使用Kubernetes定义 **自定义资源定义**（CRD）并存储在
etcd。Velero还包括处理自定义资源以执行备份、恢复和所有相关操作的控制器。

你可以备份或还原群集中的所有对象，也可以按类型、命名空间和/或标签筛选对象。

### 3.1 备份

当执行`velero backup create test-backup`开始备份时，内部工作流如下：

- Velero客户端调用Kubernetes API服务器以创建Backup对象
- BackupController会注意到新的Backup对象并执行验证
- BackupController开始备份过程。它通过查询API服务器的资源来收集要备份的数据
- BackupController调用对象存储服务（例如AWS S3）以上传备份文件

### 3.2 恢复

- **namespace重新映射**：恢复操作允许您从以前创建的备份中恢复所有对象和持久卷。您也可以只还原
  对象和持久卷的过滤子集。Velero支持多个命名空间重新映射-例如，在单个恢复中，命名空间“abc”中的对象可以在命名空间“def”下重新创建，命名空间“123”中的对象可以在“456”下重新创建。
- **恢复的默认名称**：默认名称为<BACKUP NAME>-<TIMESTAMP>，其中<TIMESTAMP>
  格式为YYYYMMDDHhmmss。也可以指定自定义名称。恢复的对象还包括一个带有键velero.io/restore-name和值的标签<RESTORE NAME>；
- **还原**：默认情况下，备份存储位置以读写模式创建。但是，在还原过程中，您可以将备份存储位置配置为只读模式，这将禁用该存储位置的备份创建和删除。这有助于确保在还原方案中不会无意中创建或删除备份；
- **恢复钩子**：您可以选择指定 在恢复期间或恢复资源之后执行的恢复钩子。例如，您可能需要在数据库应用程序容器启动之前执行自定义数据库还原操作。

当执行`velero restore create`开始恢复时，内部工作流如下：

- Velero客户端调用Kubernetes API服务器以创建一个 还原对象；
- RestoreController会通知新的Restore对象并执行验证；
- RestoreController从对象存储服务获取备份信息。然后，它会对备份的资源运行一些预处理，以确保这些资源在新的集群上可以正常工作。例如使用
  备份的API版本，以验证恢复资源是否可在目标群集上工作；
- RestoreController启动还原过程，一次还原一个符合条件的资源；

**非破坏性恢复**  
默认情况下，Velero执行非破坏性恢复，这意味着它不会删除目标群集上的任何数据。如果备份中的资源已存在于目标群集中，Velero将跳过该资源。您可以将Velero配置为使用更新策略，而不是使用
--existing-resource-policy还原标志。
当此标志设置为update时，Velero将尝试更新目标群集中的现有资源，以匹配备份中的资源。

### 3.2 关于备份的API版本

Velero使用Kubernetes API服务器的首选版本为每个组/资源备份资源。恢复资源时，目标群集中必须存在相同的API组/版本，恢复才能成功。

例如，如果要备份的群集在things API组中有一个Gizmos资源，组/版本为things/v1 alpha 1、things/v1 beta1和things/v1，
并且服务器的首选组/版本为things/v1，则将从things/v1 API端点备份所有Gizmos。恢复此群集中的备份时，目标群集必须具有things/v1端点，
以便恢复Gizmo。注意，things/v1不需要是目标集群中的首选版本，它只需要存在。

### 3.3 备份设置为过期

创建备份时，可以通过添加标志--ttl来指定TTL（生存时间）<DURATION>。如果Velero发现现有备份资源已过期，则会删除：

- 备份资源
- 云对象存储中的备份文件
- 所有PersistentVolume快照
- 所有关联的恢复

TTL标志允许用户使用以小时、分钟和秒为单位的值指定备份保留期，格式为--ttl 24h0m0s。如果未指定，则将应用默认TTL值30天。

过期策略不会立即应用，默认情况下，当gc控制器每小时运行一次协调循环时，会应用过期策略。
如果需要，您可以使用--garbage-collection-frequency标志调整协调循环的频率<DURATION>。

如果备份无法删除，则会将标签velero.io/gc-failure=<Reason>添加到备份自定义资源。您可以使用此标签筛选和选择未能删除的备份。
可能的原因有：

- 找不到备份存储位置
- BSL无法获取：无法从API服务器检索备份存储位置，原因不是找不到
- BSL只读：备份存储位置为只读

### 3.4 对象存储使用方法

Velero可以将对象存储视为事实的来源。它会不断检查是否始终存在正确的备份资源。如果存储桶中有正确格式化的备份文件，但Kubernetes
API中没有对应的备份资源，Velero会将对象存储中的信息重新存储到Kubernetes。

这个特性适用于恢复功能在群集迁移方案中工作，其中原始备份对象不存在于新群集中。

同样，如果一个已完成的备份对象存在于Kubernetes中，但不在对象存储中，它将从Kubernetes中删除，因为对象存储中的备份不存在。
对象存储同步不会删除失败或部分失败的备份。

### 3.5 安装使用

```shell
wget --no-check-certificate https://hub.gitmirror.com/https://github.com/vmware-tanzu/velero/releases/download/v1.12.0/velero-v1.12.0-linux-amd64.tar.gz
tar -xvf velero-v1.12.0-linux-amd64.tar.gz
mv velero-v1.12.0-linux-amd64/velero /usr/local/bin/

$ velero version                                                                                                                     
Client:
	Version: v1.12.0
	Git commit: 7112c62e493b0f7570f0e7cd2088f8cad968db99
<error getting server version: no matches for kind "ServerStatusRequest" in version "velero.io/v1">

```

Velero支持各种存储提供商，用于不同的备份和快照操作。Velero有一个插件系统，它允许任何人在不修改Velero代码库的情况下为其他备份和卷存储平台添加兼容性。

- [提供插件的存储提供商](https://velero.io/docs/v1.12/supported-providers/)

但现在不使用云提供商的存储资源，而是使用本地存储进行测试。

TODO