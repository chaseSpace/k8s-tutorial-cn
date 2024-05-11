# Kubernetes 维护指导

如果你在阅读本文时发现了任何错误，请在Github上提交ISSUE（或PR），我将由衷地表示感谢。

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

### 1.2 维护节点

在对节点执行维护（例如内核升级、硬件维护等）之前，我们需要先从集群中排空（drain）这个节点。

在生产环境中清空某个节点是一项需要谨慎操作的任务，需要确保在清空节点之前，所有在该节点上运行的Pod都被重新调度到其他节点。
在开始之前，请通知集群的其他维护和使用人员即将此项任务，确保节点清空不会影响生产负载。

本节假定要清空的是普通节点（而非控制平面节点）。使用`kubectl drain`从节点安全地驱逐所有 Pod到其他节点。
安全的驱逐过程允许Pod的容器体面地终止。

**预留充足的节点资源**  
在生产环境中，你需要检查集群中剩余的节点资源能否足够接收被驱逐的Pod，如果剩余资源不足，可能会导致被驱逐的Pod无法被正常调度。
即使这样，也不会影响节点的清空操作。

**忽略某些Pod**  
在清空节点期间，如果创建了新的能够容忍`node.kubernetes.io/unschedulable`污点的 Pod，那么这些 Pod 仍然可能会被调度到你已经清空的节点上。
除了 DaemonSet 之外，请避免容忍此污点。另外，如果某个用户直接为 Pod 设置了`nodeName`字段，那Pod也会绑定到这个节点上，你需要妥善处理之后再进行清空操作。

下面通过实际环境进行演示：

- 当前集群环境中包含`k8s-master`和`k8s-node1`两个节点，在`k8s-node1`
  上运行了daemonset（calico-node和kube-proxy）、deployment、bare-pod（裸Pod）、stateful四种类型的Pod，尽可能模拟生产环境
- 现在准备排空节点`k8s-node1`，实际操作情况如下

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
k8s-master   Ready    control-plane   15d   v1.27.0
k8s-node1    Ready    <none>          15d   v1.27.0

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
k8s-master   Ready                      control-plane   16d   v1.27.0
k8s-node1    Ready,SchedulingDisabled   <none>          16d   v1.27.0

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
k8s-master   Ready    control-plane   16d   v1.27.0

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

### 1.4 节点健康

节点状态是集群管理员的重要参考，通过`kubectl describe node <节点名称>`命令可以查看节点状态，其中包含节点的健康信息。

一个节点的状态包含以下信息:

- 地址（Addresses）
- 状况（Condition）
- 容量与可分配（Capacity&Allocatable）
- 信息（Info）

**地址（Addresses）**  
包含以下字段：

- HostName：由节点的内核报告。可以通过 kubelet 的 `--hostname-override` 参数覆盖。
- ExternalIP：通常是节点的可外部路由（从集群外可访问）的 IP 地址。
- InternalIP：通常是节点的仅可在集群内部路由的 IP 地址。

**状况（Conditions）**  
描述了所有 `Running` 节点的CPU、内存以及磁盘等资源是否处于压力的状况。

**容量与可分配（Capacity&Allocatable）**  
描述了节点的CPU和内存的资源总量以及可分配的资源大小，其中包含可以调度到节点上的 Pod 的个数上限。

**信息（Info）**  
节点的一般信息，如内核版本、Kubernetes 版本（`kubelet` 和 `kube-proxy` 版本）、 容器运行时详细信息，以及节点使用的操作系统。
kubelet 从节点收集这些信息并将其发布到 Kubernetes API。

#### 1.4.1 节点心跳

节点发送的心跳帮助集群确定每个节点的可用性，并在检测到故障时采取一定的行动。

节点心跳有两种：

- 更新节点的`.status`（`kk get node <node-name>`）
- `kube-node-lease`名字空间中的`Lease`（租约）对象。 每个节点都有一个关联的`Lease`
  对象。可通过`kk get lease -nkube-node-lease`查看。

`Lease` 是比节点的`.status`更轻量级的资源，使用`Lease`来表达心跳在大型集群中可以减少这些更新对性能的影响。
kubelet **同时进行**这两种心跳机制。具体细节如下:

- 对于第一种，kubelet会在节点状态发生变化或距离上一次上报时间超过`--node-status-update-frequency`
  参数设置的时间（默认5分钟）时，才会更新节点的`.status`字段
- kubelet会每隔10s发送请求给API服务器更新自己的`Lease`对象

这两种心跳是协同工作的，当API服务器超过40s没有收到任何来自节点的心跳时，则将节点的`.status`更新为`Unknown`。

> 默认情况下，节点控制器在将节点标记为 `Unknown` 后等待 5
> 分钟提交第一个 [由API发起的驱逐](https://kubernetes.io/zh-cn/docs/concepts/scheduling-eviction/api-eviction/) 请求。
> 节点控制器默认每 5 秒检查一次节点状态，可以使用 kube-controller-manager 组件上的 `--node-monitor-period` 参数来配置周期。

#### 1.4.2 节点资源容量跟踪

K8s调度器会保证节点上有足够的资源供其上的所有 Pod 使用。 它会检查节点上所有容器的**请求的总和**不会超过节点的容量。
总的请求包括由 kubelet 启动的所有容器，但**不包括**由容器运行时直接启动的容器， 也不包括不受 kubelet 控制的其他进程。

如果要为非Pod进程预留资源，参考[为系统守护进程预留资源](https://kubernetes.io/zh-cn/docs/tasks/administer-cluster/reserve-compute-resources/#system-reserved) 。

### 1.5 日志查看

节点分为主节点和普通节点。主节点运行多个K8s组件，如API Server、Controller Manager、Scheduler等。
普通节点则运行两个K8s组件：kubelet和kube-proxy。

其中只有kubelet是节点级的进程，其进程通过`systemctl`管理，通过`journalctl -u kubelet`可查看日志。常用命令如下：

```shell
systemctl status kubelet

journalctl -u kubelet -f --lines=10

journalctl -u kubelet -f --lines=100 |grep -i error

journalctl -u kubelet --since today --no-pager
```

其他组件都是以Pod形式运行。常用的查询命令如下：

```shell
# --tail N 查看最新N条
# --since=5m 查看5m前开始的日志
# --all-containers=true 查看Pod内所有容器的日志，否则只会输出第一个容器的日志
kubectl logs -n kube-system $POD_NAME --since=5m --all-containers=true
```

如果API Server不可用（API Server也挂了），还可以通过节点本地的`crictl`工具来查询Pod日志：

```shell
# 列出节点运行的所有容器
crictl ps
# 查看POD日志
crictl logs $CONTAINER_ID
```

#### 1.5.1 使用kubetail查看Pod日志

kubetail是一个开源的由Shell脚本写成的工具，可以用于方便地查看多个Pod的聚合日志。下面是简单的安装步骤：

```shell
wget https://raw.gitmirror.com/johanhaleby/kubetail/master/kubetail
chmod +x kubetail                     
mv kubetail /usr/local/bin

# 设置别名
echo alias tt="kubetail" >> ~/.zshrc
source ~/.zshrc

$ tt -v                                              
1.6.19-SNAPSHOT
```

使用步骤：

```shell
# 查看当前空间下所有pod日志
tt

tt pod1
tt pod1,pod2

tt pod1 -c container1
tt pod1 -c container1 -c container2

tt -n ns1 pod1

tt '(service|consumer|thing)' -e regex
tt 'pod_name_contains_me' -e substring

tt '(service|consumer|thing)' --regex # 等价于 -e regex

# label筛选pod
tt -l some-label=xxx

# 起始时间为1min前
tt pod1 -s 1m

# 最近100行（但笔者发现有bug，最多只能查看最近3行，这个问题比较严重！暂时可以用-s替代）
tt pod --tail 100
```

这里使用代码[main_log.go](main_log.go)进行测试：

```shell
docker build . -t leigg/hellok8s:log_test
docker push leigg/hellok8s:log_test
kk apply -f deployment_logtest.yaml

$ kk get deploy hellok8s-logtest              
NAME               READY   UP-TO-DATE   AVAILABLE   AGE
hellok8s-logtest   2/2     2            2           32s

# 查看一个pod的日志
$ tt hellok8s-logtest-7f658bb745-9522d         
Will tail 1 logs...
hellok8s-logtest-7f658bb745-9522d
[hellok8s-logtest-7f658bb745-9522d] 2023/12/14 10:42:39 log test 36
[hellok8s-logtest-7f658bb745-9522d] 2023/12/14 10:42:42 log test 37
[hellok8s-logtest-7f658bb745-9522d] 2023/12/14 10:42:45 log test 38
[hellok8s-logtest-7f658bb745-9522d] 2023/12/14 10:42:48 log test 39

# 查看deployment下所有pod日志（不同pod的日志使用颜色区分）
$ tt -l app=hellok8s                              
Will tail 2 logs...
hellok8s-logtest-7f658bb745-9522d
hellok8s-logtest-7f658bb745-vvxhk
[hellok8s-logtest-7f658bb745-9522d] 2023/12/14 10:43:39 log test 56
[hellok8s-logtest-7f658bb745-9522d] 2023/12/14 10:43:42 log test 57
[hellok8s-logtest-7f658bb745-9522d] 2023/12/14 10:43:45 log test 58
[hellok8s-logtest-7f658bb745-vvxhk] 2023/12/14 10:43:39 log test 56
[hellok8s-logtest-7f658bb745-vvxhk] 2023/12/14 10:43:42 log test 57
[hellok8s-logtest-7f658bb745-vvxhk] 2023/12/14 10:43:45 log test 58
```

#### 1.5.2 使用Kail查看Pod日志

Kail具有kubetail相似的功能，但还有一些它没有的功能，下面是安装步骤：

```shell
wget https://hub.gitmirror.com/?q=https://github.com/boz/kail/releases/download/v0.17.1/kail_v0.17.1_linux_amd64.tar.gz -O kail.gz
tar zxf kail.gz
chmod +x kail         
mv kail /usr/local/bin

$ kail version
v0.17.1 (24da853)
```

使用示例：

```shell
# 不带参数时输出当前空间下所有pod日志
kail

kail -l some_label=xxx

kail -p pod1

# 指定deployment
kail -d hellok8s-logtest

kail --svc xxx

# 指定statefulSet
kail --sts xxx

# 指定node（上的所有pod）
kail --node node1

# 指定ingress（下的后端svc的所有pod）
kail --ing xxx

# 指定job
kail -j xxx
```

kail不支持查看最近N行的功能，也不支持颜色打印（无法区分不同Pod，有点伤！）。

#### 1.5.3 使用stern查看Pod日志

stern是笔者推荐的Pod日志查看工具，它具有多个必备的实用功能，包括颜色打印、正则匹配和查看最近N行日志。安装步骤：

```shell
wget https://hub.gitmirror.com/?q=https://github.com/stern/stern/releases/download/v1.27.0/stern_1.27.0_linux_amd64.tar.gz -O stern.gz
tar zxf stern.gz
chmod +x stern
mv stern /usr/local/bin 

# 设置别名
echo alias sn="stern" > ~/.zshrc
source ~/.zshrc

$ sn -v       
version: 1.27.0
commit: 67c7c9b5eff869662033015d0af7b96c25272d1b
built at: 2023-11-15T02:11:41Z
```

使用示例:

```shell
# 不带参数则打印help
sn

# 默认使用正则匹配pod，匹配 *test*，且输出全部日志
sn test

# 最近10行
sn test --tail 10

# -i使用正则匹配指定字符串的日志行，不必再使用grep
sn test -i 11:35

# 指定node
sn test --node k8s-node1

# 每一行日志带上节点时间戳
sn test -t

# label过滤，-A全部空间
sn -l app=hellok8s -A

# 包含斜杠，则是 resource/name 的完全匹配方式（不是正则）
# resource支持 pod/rc/rs/svc/daemonset/deployment/sts/job
sn deploy/hellok8s-logtest
```

支持`--output {format}`指定格式打印：

```shell
# 支持三种格式
# 默认: podName, containerName, msg
# raw：msg
# json：将msg、node、namespace、podName、containerName序列化为JSON打印
sn logtest -o json

sn logtest -o json |jq
```

如果Pod日志原本就是JSON，可以使用以下命令格式化打印（以[main_log_json.go](main_log_json.go)为例）：

```shell
$ sn logtest -o raw |xargs -0 -d'\n' -l jq -s
{
  "time": "2023-12-14 12:48:04",
  "number": 34,
  "field1": "abcdefghijklmn",
  "field2": "0123456789",
  "field3": "Golang",
  "field4": "Kubernetes"
}
{
  ...
}
```

此外，还支持`--since`
、定制颜色、指定容器、指定context。它最特色的功能是支持[模板化输出](https://github.com/stern/stern?tab=readme-ov-file#templates)
，这个就需要读者自行去了解了。

## 2. 镜像管理

镜像的存放位置在 Kubernetes 集群中取决于所采用的容器运行时。K8s与容器运行时之间的通信遵循 CRI（Container
Runtime Interface，容器运行时接口）标准。CRI 定义了容器运行时与 Kubernetes kubelet 之间的接口，使得 kubelet
能够通过这个接口来管理容器的生命周期。

为了与容器运行时进行交互和管理容器，Kubernetes 使用了一个命令行工具，即`crictl`。这个工具提供了与容器运行时通信的功能，允许
Kubernetes 组件也就是kubelet与底层容器运行时进行交互。`crictl`只能管理当前节点上的Pod/容器/镜像，不能远程管理。

安装集群的时候我们会按如下方式配置好`crictl`：

```shell
# 若是docker作为容器运行时
crictl config runtime-endpoint unix:///var/run/cri-dockerd.sock

# 若是containerd作为容器运行时
crictl config runtime-endpoint unix:///var/run/containerd/containerd.sock
```

我们可以使用`crictl`来直接管理集群中的Pod及其中的容器（包括创建/启动/停止/删除），但通常只会在kubelet无法正常控制Pod时才会这样做。


> 比如当API Server无法连接的时候，kubectl就会失去对集群资源（包括Pod）的控制能力。

你可以使用`crictl -h`获取此工具的帮助信息。

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
registry.aliyuncs.com/google_containers/kube-apiserver            v1.27.0             6f707f569b572       33.4MB
registry.aliyuncs.com/google_containers/kube-controller-manager   v1.27.0             95fe52ed44570       31MB
registry.aliyuncs.com/google_containers/kube-proxy                v1.27.0             5f82fc39fa816       23.9MB
registry.aliyuncs.com/google_containers/kube-scheduler            v1.27.0             f73f1b39c3fe8       18.2MB
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

## 未完待续