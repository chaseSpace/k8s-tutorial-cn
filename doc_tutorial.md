# Kubernetes 使用教程

为了方便阅读，建议点击网页右上角的 ![toc.jpg](img/toc.jpg) 按钮在右侧展开目录。

**环境准备**：

```
10.0.2.2 k8s-master  
10.0.2.3 k8s-node1
```

可参考下面的教程来搭建Kubernetes集群：

- [使用minikube安装k8s单节点集群](install_by_minikube/install.md)
- [使用kubeadm搭建k8s多节点集群](install_by_kubeadm/install.md)

一些提高效率的设置：

1. [安装ohmyzsh](doc_install_ohmyzsh.md)
2. 设置kubectl的alias为`kk`，下文会用到。

## 1. 简介

Kubernetes项目是Google公司在2014年启动（内部项目最初叫做Borg）。它建立在Google公司超过10多年的运维经验之上，Google所有的应用都运行在容器上。
Kubernetes是目前最受欢迎的开源容器编排平台。

Kubernetes可以实现容器集群的自动化部署、自动扩缩容、维护等功能。它拥有自动包装、自我修复、横向缩放、服务发现、负载均衡、
自动部署、升级回滚、存储编排等特性。

### 1.1 设计架构

K8s集群节点拥有Master和Node两种角色，Master管理Node，而Node管理容器。

Master主要负责整个集群的管控，包含监控、编排、调度集群中的各个工作节点。通常Master会占用一台独立的服务器，基于高可用可能会占用多台。

Node则是集群中的承载实际工作任务的节点，直接负责对容器的控制，可以无限扩展。

K8s架构图如下：
<div align="center">
<img src="img/k8s-arch.webp" width = "1300" height = "550" alt=""/>
</div>

### 1.2 Master

Master由四个部分组成：

1. **API Server进程**  
   核心组件之一，为集群中各类资源提供增删改查的HTTP REST接口，即操作任何资源都要经过API Server。与其通信有三种方式：

- 最原始的通过REST API访问；
- 通过官方提供的Client来访问，本质上也是REST API调用；
- 通过kubectl客户端访问，其将命令转换为REST API调用，是最主要的访问方式。

2. **etcd**  
   K8s使用etcd作为内部数据库，用于保存集群配置以及所有对象的状态信息。只有API Server进程能直接读写etcd。


3. **调度器（Scheduler）**  
   它是Pod资源的调度器，用于监听刚创建还未分配Node的Pod，为其分配相应Node。
   调度时会考虑资源需求、硬件/软件/指定限制条件以及内部负载情况等因素，所以可能会调度失败。
   调度器也是操作API Server进程的各项接口来完成调度的。比如Watch接口监听新建的Pod，并搜索所有满足Pod需求的Node列表，
   再执行Pod调度逻辑，调度成功后将Pod绑定到目标Node上。


4. **控制器管理器（kube-controller-manager）**  
   集群中的大部分功能是由控制器执行的。理论上，以下每种控制器都是一个单独的进程，为了降低复杂度，它们都被编译、合并到单个文件中，
   并在单个进程中运行。

- Node控制器：负责在Node故障时响应
- Replication控制器：负责对系统重每个ReplicationController对象维护预期数量的Pod
- Endpoint控制器：负责生成和维护所有Endpoint对象的控制器。Endpoint控制器用于监听Service和对应Pod副本的变化
- ServiceAccount及Token控制器：为新的命名空间创建默认账户和API访问令牌。

kube-controller-manager所执行的各项操作也是基于API Server进程的。

### 1.3 Node

Node由三部分组成：kubelet、kube-proxy和容器运行时（如docker/containerd）。

1. **kubelet**  
   它是每个Node上都运行的主要代理进程。kubelet以PodSpec为单位来运行任务，后者是一种Pod的yaml或json对象。
   kubelet会运行由各种方式提供的一系列PodSpec，并确保这些PodSpec描述的容器健康运行。

不是k8s创建的容器不属于kubelet管理范围，kubelet也会及时将Pod内容器状态报告给API Server，并定期执行PodSpec描述的容器健康检查。
同时kubelet也负责存储卷等资源的管理。

kubelet会定期调用Master节点上的API Server的REST API以报告自身状态，然后由API Server存储到etcd中。

2. **kube-proxy**  
   用于管理Service的网络访问入口，包括从集群内的其他Pod到Service的访问以及集群外访问Service。

3. **容器运行时**  
   负责直接管理容器生命周期的软件。k8s支持包含docker、containerd在内的任何基于k8s cri（容器运行时接口）实现的runtime。

### 1.4 k8s的核心对象

为了完成对大规模容器集群的高效率、全功能性的任务编排，k8s设计了一系列额外的抽象层，这些抽象层对应的实例由用户通过Yaml或Json文件进行描述，
然后由k8s的API Server负责解析、存储和维护。

k8s的对象模型图如下：

<div align="center">
<img src="img/k8s-object-model.jpg" width = "1200" height = "600" alt=""/>
</div>

1. **Pod**  
   Pod是k8s调度的基本单元，它封装了一个或多个容器。Pod中的容器会作为一个整体被k8s调度到一个Node上运行。

Pod一般代表单个app，由一个或多个关系紧密的容器组成。这些容器拥有共同的生命周期，作为一个整体被编排到Node上。并且它们
共享存储卷、网络和计算资源。k8s以Pod为最小单位进行调度等操作。

2. **控制器**

一般来说，用户不会直接创建Pod，而是创建控制器来管理Pod，因为控制器能够更细粒度的控制Pod的运行方式，比如副本数量、部署位置等。
控制器包含下面几种：

- **Replication控制器**（以及ReplicaSet控制器）：负责保证Pod副本数量符合预期（涉及对Pod的启动、停止等操作）；
- **Deployment控制器**：是高于Replication控制器的对象，也是最常用的控制器，用于管理Pod的发布、更新、回滚等；
- **StatefulSet控制器**：与Deployment同级，提供排序和唯一性保证的特殊Pod控制器。用于管理有状态服务，比如数据库等。
- **DaemonSet控制器**：与Deployment同级，用于在集群中的每个Node上运行单个Pod，多用于日志收集和转发、监控等功能的服务。并且它可以绕过常规Pod无法调度到Master运行的限制；
- **Job控制器**：与Deployment同级，用于管理一次性任务，比如批处理任务；
- **CronJob控制器**：与Deployment同级，在Job控制器基础上增加了时间调度，用于执行定时任务。

3. **Service、Ingress和Storage**

**Service**是对一组Pod的抽象，它定义了Pod的逻辑集合以及访问该集合的策略。前面的Deployment等控制器只定义了Pod运行数量和生命周期，
并没有定义如何访问这些Pod，由于Pod重启后IP会发生变化，没有固定IP和端口提供服务。  
Service对象就是为了解决这个问题。Service可以自动跟踪并绑定后端控制器管理的多个Pod，即使发生重启、扩容等事件也能自动处理，
同时提供统一IP供前端访问，所以通过Service就可以获得服务发现的能力，部署微服务时就无需单独部署注册中心组件。

**Ingress**不是一种服务类型，而是一个路由规则集合，通过Ingress规则定义的规则，可以将多个Service组合成一个虚拟服务（如前端页面+后端API）。
它可实现业务网关的作用，类似Nginx的用法，可以实现负载均衡、SSL卸载、流量转发、流量控制等功能。

**Storage**是Pod中用于存储的抽象，它定义了Pod的存储卷，包括本地存储和网络存储；它的生命周期独立于Pod之外，可进行单独控制。

4. **资源划分**

- 命名空间（Namespace）：k8s通过namespace对同一台物理机上的k8s资源进行逻辑隔离。
-

标签（Labels）：是一种语义化标记，可以附加到Pod、Node等对象之上，然后更高级的对象可以基于标签对它们进行筛选和调用，例如Service可以将请求只路由到指定标签的Pod，或者Deployment可以将Pod只调度到指定标签的Node。

- 注解（Annotations）：也是键值对数据，但更灵活，它的value允许包含结构化数据。一般用于元数据配置，不用于筛选。例如Ingress中通过注解为nginx控制器配置
  **禁用ssl重定向**。

## 2. 创建程序和使用docker管理镜像

### 2.1 安装docker

如果安装的k8s版本不使用docker作为容器运行时，那只需要在master节点（或专门的镜像部署节点）安装docker。
我们需要docker来构建和推送镜像。
```shell
yum install -y yum-utils device-mapper-persistent-data lvm2
yum-config-manager --add-repo http://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
# 列出可用版本
#yum list docker-ce --showduplicates | sort -r
# 选择版本安装
yum -y install docker-ce-18.03.1.ce

docker version

# 设置源
echo '{
    "registry-mirrors": [
        "https://registry.docker-cn.com"
    ]
}' > /etc/docker/daemon.json

# 重启docker
systemctl restart docker
# 开机启动
systemctl enable docker

# 查看源是否设置成功
$ docker info |grep Mirrors -A 3
Registry Mirrors:
 https://registry.docker-cn.com/

```

另外，可能需要纠正主机时间和时区：
```shell
# 先设置时区
echo "ZONE=Asia/Shanghai" >> /etc/sysconfig/clock
ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

# 若时间不准，则同步时间（容器会使用节点的时间）
yum -y install ntpdate
ntpdate -u  pool.ntp.org

$ date # 检查时间
```

### 2.2 构建和运行镜像

1. 编写一个简单的[main.go](main.go)
2. 编写[Dockerfile](Dockerfile)

打包镜像（替换leigg为你的docker账户名）

```shell
docker build . -t leigg/hellok8s:v1
```

这里有个小技巧，（修改代码后）重新构建镜像若使用同样的镜像名会导致旧的镜像的名称和tag变成`<none>`，可通过下面的命令来一键删除：

```shell
docker image prune -f
# docker system prune # 删除
```

测试运行：

```shell
docker run --rm -p 3000:3000 leigg/hellok8s:v1
```

运行ok则按ctrl+c退出。

### 2.3. 推送到docker仓库

k8s部署服务时会从远端拉取本地不存在的镜像，但由于这个k8s版本是使用containerd不是docker作为容器运行时，
所以读取不到docker构建的本地镜像，另外即使当前节点有本地镜像，其他节点不存在也会从远端拉取，所以每次修改代码后，
都需要推送新的镜像到远端，再更新部署。

先登录docker hub：

```shell
$ docker login  # 然后输入自己的docker账户和密码，没有先去官网注册
```

推送镜像到远程hub

```shell
docker push leigg/hellok8s:v1
```
>如果是生产部署，则不会使用docker官方仓库，而是使用harbor等项目搭建本地仓库，以保证稳定拉取镜像。
## 3. 使用Pod

Pod 是 Kubernetes 最小的可部署单元，**通常包含一个或多个容器**。
它们可以容纳紧密耦合的容器，例如运行在同一主机上的应用程序和其辅助进程。但是，在生产环境中，通常使用其他资源来更好地管理和扩展服务。

Pod是 Kubernetes 中创建和管理的、最小的可部署的计算单元。

### 3.1 创建nginx pod

```yaml
# nginx.yaml
apiVersion: v1
kind: Pod  # 资源类型=pod
metadata:
  name: nginx-pod  # 需要唯一
spec:
  containers: # pod内的容器组
    - name: nginx-container
      image: nginx  # 镜像默认来源 DockerHub
```

### 3.2 创建pod

运行第一条k8s命令创建pod：

```shell
kubectl apply -f nginx.yaml
```

### 3.3 查看nginx-pod状态

```shell
kubectl get po nginx-pod
```

查看全部pods：`kubectl get pods`

### 3.4 与pod交互

添加端口转发，然后就可以在宿主机访问nginx-pod

```shell
# 宿主机4000映射到pod的80端口
# 这条命令是阻塞的，仅用来调试pod服务是否正常运行
kubectl port-forward nginx-pod 4000:80

# 打开另一个控制台
curl http://127.0.0.1:4000
```

其他命令：

```shell
kubectl delete pod nginx-pod # 删除pod
kubectl delete -f nginx.yaml  # 删除配置文件内的全部资源
 
kubectl exec -it nginx-pod -- /bin/bash   # 进入pod shell

# 支持 --tail LINES_NUM
kubectl logs -f nginx-pod  # 查看日志（stdout/stderr）
```

### 3.5 Pod 与 Container 的不同

在刚刚创建的资源里，在最内层是我们的服务 nginx，运行在 container 容器当中， container (容器) 的**本质是进程**，而 pod
是管理这一组进程的资源。

所以 pod 可以管理多个 container，在某些场景例如服务之间需要文件交换(日志收集)，本地网络通信需求(使用 localhost 或者 Socket
文件进行本地通信)，
在这些场景中使用 pod 管理多个 container 就非常的推荐。而这，也是 k8s 如何处理服务之间复杂关系的第一个例子。

**Pod定义**  
Pod 是 Kubernetes 最小的可部署/调度单元，通常包含一个或多个容器。它们可以容纳紧密耦合的容器，例如运行在同一主机上的应用程序和其辅助进程。但是，在生产环境中，通常使用其他资源来更好地管理和扩展服务。

### 3.6 创建go程序的pod

定义[pod.yaml](./pod.yaml)，这里面使用了之前已经推送的镜像`leigg/hellok8s:v1`

启动pod：

```shell
$ kk apply -f pod.yaml
# 几秒后
$ kk get pods
NAME      READY   STATUS    RESTARTS   AGE
go-http   1/1     Running   0          17s
```

临时开启端口转发（在master节点）：

```shell
# 绑定pod端口3000到 master节点的3000端口
kubectl port-forward go-http 3000:3000
```

现在pod提供的http服务可以在master节点上可用。

打开另一个会话测试：

```shell
$ curl http://localhost:3000
[v1] Hello, Kubernetes!#
```

### 3.7 pod有哪些状态

- Pending（挂起）： Pod 正在调度中（包含镜像拉取、容器创建和启动）。
- ContainerCreating（容器创建中）： Pod 已经被调度，但其中的容器尚未完全创建和启动。
- Running（运行中）： Pod 中的容器已经在运行。
- Completed（已成功）： 所有容器都成功终止，任务或工作完成，特指那些一次性或批处理任务而不是常驻容器。
- Failed（已失败）： 至少一个容器以非零退出码终止。
- Unknown（未知）： 无法获取 Pod 的状态，通常是宿主机通信问题导致。

**关于Pod的重启策略**  
即`restartPolicy`字段，可选值为Always、OnFailure和Never。此策略对Pod内所有容器有效，
由Pod所在Node上的kubelet执行判断和重启。由kubelet重启的已退出容器将会以递增延迟的方式（10s，20s，40s...）
尝试重启，上限5min。成功运行10min后这个时间会重置。**一旦Pod绑定到某个节点上，除非节点自身问题或手动调整，
否则不会再调度到其他节点**。

**Pod的销毁过程**  
当Pod需要销毁时，kubelet会先向API Server发送删除请求，然后等待Pod中所有容器停止，包含以下过程:

1. 用户发送Pod删除命令
2. API Server更新Pod：开始销毁，并设定宽限时间（默认30s，可通过--grace-period=n指定，为0时需要追加--force），超时强制Kill
3. 同时触发：
    - Pod 标记为 Terminating
    - kubelet监听到 Terminating 状态，开始终止Pod
    - Endpoint控制器监控到Pod即将删除，将移除所有Service对象中与此Pod关联的Endpoint对象
4. 如Pod定义了prepStop回调，则会在Pod中执行，并再次执行步骤2，且增加宽限时间2s
5. Pod进程收到SIGTERM信号
6. 到达宽限时间还在运行，kubelet发送SIGKILL信号，设置宽限时间0s，直接删除Pod

## 4. 使用Deployment

通常，Pod不会被（通过pod.yaml）直接创建和管理，而是由更高级别的控制器，如Deployment，来创建和管理。
这是因为Deployment提供了更强大的应用程序管理功能。

- **应用管理**：Deployment是Kubernetes中的一个控制器，用于管理应用程序的部署和更新。它允许你定义应用程序的期望状态，然后确保集群中的副本数符合这个状态。

- **自愈能力**：Deployment可以自动修复故障，如果Pod失败，它将启动新的Pod来替代。这有助于确保应用程序的高可用性。

- **滚动更新**：Deployment支持滚动更新，允许你逐步将新版本的应用程序部署到集群中，而不会导致中断。

- **副本管理**：Deployment负责管理Pod的副本，可以指定应用程序需要的副本数量，Deployment将根据需求来自动调整。

- **声明性配置**：Deployment的配置是声明性的，你只需定义所需的状态，而不是详细指定如何实现它。Kubernetes会根据你的声明来管理应用程序的状态。


### 4.1 部署deployment
先创建一个[deployment文件](./deployment.yaml)， 用来编排多个pod。

```shell
$ kk apply -f deployment.yaml
deployment.apps/hellok8s-go-http created

# 查看启动的pod
$ kk get deployments                
NAME               READY   UP-TO-DATE   AVAILABLE   AGE
hellok8s-go-http   2/2     2            2           3m
```

还可以查看pod运行的node：

```shell
# 这里的IP是pod ip，属于部署k8s集群时规划的pod网段
# NODE就是集群中的node名称
$ kk get pod -o wide
NAME                                READY   STATUS    RESTARTS   AGE   IP           NODE        NOMINATED NODE   READINESS GATES
hellok8s-go-http-55cfd74847-5jw7f   1/1     Running   0          68s   20.2.36.75   k8s-node1   <none>           <none>
hellok8s-go-http-55cfd74847-zlf49   1/1     Running   0          68s   20.2.36.74   k8s-node1   <none>           <none>
```

**删除pod会自动重启一个，确保可用的pod数量与`deployment.yaml`中的`replicas`字段保持一致，不再演示**。

### 4.2 修改deployment

通过vi修改内容中的replicas=3，再次部署，开始之前，我们使用下面的命令来观察pod数量变化

```shell
$ kubectl get pods --watch
NAME                                   READY   STATUS    RESTARTS   AGE
hellok8s-go-http-58cb496c84-cft9j   1/1     Running   0          4m7s


# 在另一个CLI执行 kk apply ...

hellok8s-go-http-58cb496c84-sdrt2   0/1     Pending   0          0s
hellok8s-go-http-58cb496c84-sdrt2   0/1     Pending   0          0s
hellok8s-go-http-58cb496c84-pjkp9   0/1     Pending   0          0s
hellok8s-go-http-58cb496c84-pjkp9   0/1     Pending   0          0s
hellok8s-go-http-58cb496c84-sdrt2   0/1     ContainerCreating   0          0s
hellok8s-go-http-58cb496c84-pjkp9   0/1     ContainerCreating   0          0s
hellok8s-go-http-58cb496c84-pjkp9   1/1     Running             0          1s
hellok8s-go-http-58cb496c84-sdrt2   1/1     Running             0          1s
```

### 4.3 更新deployment

这一步通过修改main.go来模拟实际项目中的服务更新，修改后的文件是[main2.go](./main2.go)。

重新构建镜像：

```shell
docker build . -t leigg/hellok8s:v2
```

再次push镜像到仓库：

```shell
docker push leigg/hellok8s:v2
```

然后更新deployment：

```shell
$ kubectl set image deployment/hellok8s-go-http hellok8s=leigg/hellok8s:v2

$ 查看更新过程
$ kubectl rollout status deployment/hellok8s-go-http
Waiting for deployment "hellok8s-go-http" rollout to finish: 2 out of 3 new replicas have been updated...
Waiting for deployment "hellok8s-go-http" rollout to finish: 2 out of 3 new replicas have been updated...
Waiting for deployment "hellok8s-go-http" rollout to finish: 2 out of 3 new replicas have been updated...
Waiting for deployment "hellok8s-go-http" rollout to finish: 1 old replicas are pending termination...
Waiting for deployment "hellok8s-go-http" rollout to finish: 1 old replicas are pending termination...
deployment "hellok8s-go-http" successfully rolled  # OK

# 也可以直接查看pod信息，会观察到pod正在更新（这是一个启动新pod，删除旧pod的过程，最终会维持到所配置的replicas数量）
$ kk get pods
NAMESPACE     NAME                                       READY   STATUS              RESTARTS      AGE
default       go-http                                    1/1     Running             0             14m
default       hellok8s-go-http-55cfd74847-5jw7f          1/1     Terminating         0             27m
default       hellok8s-go-http-55cfd74847-z29dl          1/1     Running             0             23m
default       hellok8s-go-http-55cfd74847-zlf49          1/1     Running             0             27m
default       hellok8s-go-http-668c7f75bd-m56pm          0/1     ContainerCreating   0             0s
default       hellok8s-go-http-668c7f75bd-qlrk5          1/1     Running             0             14s

# 绑定其中一个pod来测试
$ kk port-forward hellok8s-go-http-668c7f75bd-m56pm 3000:3000
Forwarding from 127.0.0.1:3000 -> 3000
Forwarding from [::1]:3000 -> 3000
```

在另一个会话窗口执行

```shell
$ curl http://localhost:3000
[v2] Hello, Kubernetes!
```

这里演示的更新是容器更新，修改deployment.yaml的其他配置也属于更新。

### 4.4 回滚部署

如果新的镜像无法正常启动，则旧的pod不会被删除，但需要回滚，使deployment回到正常状态。

按照下面的步骤进行：

1. 修改main.go，将最后监听端口那行先注释，添加一行：panic("something went wrong")
2. 构建镜像: docker build . -t leigg/hellok8s:v2_problem
3. push镜像：docker push leigg/hellok8s:v2_problem
4. 更新deployment使用的镜像：kubectl set image deployment/hellok8s-go-http hellok8s=leigg/hellok8s:v2_problem
5. 观察：kubectl rollout status deployment/hellok8s-go-http （会停滞，按 Ctrl-C 停止观察）
6. 观察pod：kubectl get pods

```shell
$ kk get pods
NAME                                READY   STATUS             RESTARTS     AGE
go-http                             1/1     Running            0            36m
hellok8s-go-http-55cfd74847-fv2kp   1/1     Running            0            17m
hellok8s-go-http-55cfd74847-l78pb   1/1     Running            0            17m
hellok8s-go-http-55cfd74847-qtb59   1/1     Running            0            17m
hellok8s-go-http-7c9d684dd-msj2c    0/1     CrashLoopBackOff   1 (4s ago)   6s

# CrashLoopBackOff状态表示重启次数过多，过一会儿再试，这表示pod内的容器无法正常启动，或者启动就立即退出了

# 查看每个副本集每次更新的pod情况（包含副本数量、上线时间、使用的镜像tag）
# DESIRED-预期数量，CURRENT-当前数量，READY-可用数量
# -l 进行标签筛选
$ kubectl get rs -l app=hellok8s -o wide
NAME                          DESIRED   CURRENT   READY   AGE   CONTAINERS   IMAGES                      SELECTOR
hellok8s-go-http-55cfd74847   0         0         0       76s   hellok8s     leigg/hellok8s:v1           app=hellok8s,pod-template-hash=55cfd74847
hellok8s-go-http-668c7f75bd   3         3         3       55s   hellok8s     leigg/hellok8s:v2           app=hellok8s,pod-template-hash=668c7f75bd
hellok8s-go-http-7c9d684dd    1         1         0       11s   hellok8s     leigg/hellok8s:v2_problem   app=hellok8s,pod-template-hash=7c9d684dd
```

现在进行回滚：

```shell
# 先查看deployment更新记录
$ kk rollout history deployment/hellok8s-go-http               
deployment.apps/hellok8s-go-http 
REVISION  CHANGE-CAUSE
1         <none>
2         <none>
3         <none>

# 现在回到revision 2，可以先查看它具体信息（主要看用的哪个镜像tag）
$ kk rollout history deployment/hellok8s-go-http --revision=2
deployment.apps/hellok8s-go-http with revision #2
Pod Template:
  Labels:	app=hellok8s
	pod-template-hash=668c7f75bd
  Containers:
   hellok8s:
    Image:	leigg/hellok8s:v2
    Port:	<none>
    Host Port:	<none>
    Environment:	<none>
    Mounts:	<none>
  Volumes:	<none>

# 确认后，回滚（到上个版本）
$ kubectl rollout undo deployment/hellok8s-go-http  #到指定版本 --to-revision=2          
deployment.apps/hellok8s-go-http rolled back

# 检查副本集状态（所处的版本）
$ kk get rs -l app=hellok8s -o wide                                
hellok8s-go-http-55cfd74847   0         0         0       9m31s   hellok8s     leigg/hellok8s:v1           app=hellok8s,pod-template-hash=55cfd74847
hellok8s-go-http-668c7f75bd   3         3         3       9m10s   hellok8s     leigg/hellok8s:v2           app=hellok8s,pod-template-hash=668c7f75bd
hellok8s-go-http-7c9d684dd    0         0         0       8m26s   hellok8s     leigg/hellok8s:v2_problem   app=hellok8s,pod-template-hash=7c9d684dd

# 恢复正常
$ kk get deployments hellok8s-go-http
NAME               READY   UP-TO-DATE   AVAILABLE   AGE
hellok8s-go-http   3/3     3            3           7m42s
```

### 4.5 滚动更新（Rolling Update）

k8s 1.15版本起支持滚动更新，即先创建新的pod，创建成功后再删除旧的pod，确保更新过程无感知，大大降低对业务影响。

在 deployment 的资源定义中, spec.strategy.type 有两种选择:

- RollingUpdate: 逐渐增加新版本的 pod，逐渐减少旧版本的 pod。（常用）
- Recreate: 在新版本的 pod 增加前，先将所有旧版本 pod 删除（针对那些不能多进程部署的服务）

另外，还可以通过以下字段来控制升级 pod 的速率：

- maxSurge: 最大峰值，用来指定可以创建的超出期望 Pod 个数的 Pod 数量。
- maxUnavailable: 最大不可用，用来指定更新过程中不可用的 Pod 的个数上限。

如果不设置，deployment会有默认的配置：

```shell
$ kk describe -f deployment.yaml
Name:                   hellok8s-go-http
Namespace:              default
CreationTimestamp:      Sun, 13 Aug 2023 21:09:33 +0800
Labels:                 <none>
Annotations:            deployment.kubernetes.io/revision: 1
Selector:               app=aaa,app1=hellok8s
Replicas:               3 desired | 3 updated | 3 total | 3 available | 0 unavailable
StrategyType:           RollingUpdate
MinReadySeconds:        0
RollingUpdateStrategy:  25% max unavailable, 25% max surge # <------ 看这
省略。。。
```

为了明确地指定deployment的更新方式，我们需要在yaml中配置：

```shell
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hellok8s-go-http
spec:
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
  replicas: 3
省略其他熟悉的配置项。。。
```

这样，我们通过`k apply`命令时会以滚动更新方式进行。
> 从`maxSurge: 1`可以看出更新时最多会出现4个pod，从`maxUnavailable: 1`可以看出最少会有2个pod正常运行。

注意：无论是通过`kubectl set image ...`还是`kubectl rollout restart deployment xxx`方式更新deployment都会遵循配置进行滚动更新。

### 4.6 控制Pod水平伸缩

```shell
# 指定副本数量
$ kubectl scale deployment/hellok8s-go-http --replicas=10
deployment.apps/hellok8s-go-http scaled

# 观察到副本集版本并没有变化，而是数量发生变化
$ kubectl get rs -l app=hellok8s -o wide                 
NAME                          DESIRED   CURRENT   READY   AGE   CONTAINERS   IMAGES                      SELECTOR
hellok8s-go-http-55cfd74847   0         0         0       33m   hellok8s     leigg/hellok8s:v1           app=hellok8s,pod-template-hash=55cfd74847
hellok8s-go-http-668c7f75bd   10        10        10      33m   hellok8s     leigg/hellok8s:v2           app=hellok8s,pod-template-hash=668c7f75bd
hellok8s-go-http-7c9d684dd    0         0         0       32m   hellok8s     leigg/hellok8s:v2_problem   app=hellok8s,pod-template-hash=7c9d684dd
```

### 4.7 存活探针 (livenessProb)

存活探测器来确定什么时候要重启容器。 例如，存活探测器可以探测到应用死锁（应用程序在运行，但是无法继续执行后面的步骤）情况。
重启这种状态下的容器有助于提高应用的可用性，即使其中存在缺陷。

下面更新app代码为[main_liveness.go](./main_liveness.go)，并且构建新的镜像以及推送到远程仓库：

```shell
docker build . -t leigg/hellok8s:liveness
docker push leigg/hellok8s:liveness
```

然后在deployment.yaml内添加存活探针配置：

```shell
apiVersion: apps/v1
kind: Deployment
metadata:
  # deployment唯一名称
  name: hellok8s-go-http
spec:
  replicas: 2 # 副本数量
  selector:
    matchLabels:
      app: hellok8s # 管理template下所有 app=hellok8s的pod，（要求和template.metadata.labels完全一致！！！否则无法部署deployment）
  template: # template 定义一组pod
    metadata:
      labels:
        app: hellok8s
    spec:
      containers:
        - image: leigg/hellok8s:v1
          name: hellok8s
          # 存活探针
          livenessProbe:
            # http get 探测指定pod提供HTTP服务的路径和端口
            httpGet:
              path: /healthz
              port: 3000
            # 3s后开始探测
            initialDelaySeconds: 3
            # 每3s探测一次
            periodSeconds: 3
```

更新deployment：

```shell
kk apply -f deployment.yaml
kk set image deployment/hellok8s-go-http hellok8s=leigg/hellok8s:liveness
```

现在pod将在15s后一直重启：

```shell
$ kk get pods
NAME                                READY   STATUS    RESTARTS      AGE
hellok8s-go-http-7d948dfc79-jwjrm   1/1     Running   2 (10s ago)   58s
hellok8s-go-http-7d948dfc79-wpk2d   1/1     Running   2 (11s ago)   59s


#可以看到探针失败原因
$ kk describe pod hellok8s-go-http-7d948dfc79-wpk2d
...
Events:
  Type     Reason     Age                 From               Message
  ----     ------     ----                ----               -------
  Normal   Scheduled  113s                default-scheduler  Successfully assigned default/hellok8s-go-http-7d948dfc79-wpk2d to k8s-node1
  Normal   Pulled     41s (x4 over 113s)  kubelet            Container image "leigg/hellok8s:liveness" already present on machine
  Normal   Created    41s (x4 over 113s)  kubelet            Created container hellok8s
  Normal   Started    41s (x4 over 113s)  kubelet            Started container hellok8s
  Normal   Killing    41s (x3 over 89s)   kubelet            Container hellok8s failed liveness probe, will be restarted
  Warning  Unhealthy  23s (x10 over 95s)  kubelet            Liveness probe failed: HTTP probe failed with statuscode: 500
```

还有其他探测方式，比如TCP、gRPC、Shell命令。

[官方文档](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)

### 4.8 就绪探针 (readiness)

就绪探测器可以知道容器何时准备好接受请求流量，当一个 Pod 内的所有容器都就绪时，才能认为该 Pod 就绪。
这种信号的一个用途就是控制哪个 Pod 作为 Service 的后端。若 Pod 尚未就绪，会被从 Service 的负载均衡器中剔除。

如果一个Pod升级后不能就绪，就不应该允许流量进入该Pod，否则升级完成后导致所有服务不可用。

下面更新app代码为[main_readiness.go](./main_readiness.go)，并且构建新的镜像以及推送到远程仓库：

```shell
docker build . -t leigg/hellok8s:readiness
docker push leigg/hellok8s:readiness
```

然后修改配置文件为 [deployment_readiness.yaml](deployment_readiness.yaml)

更新deployment：

```shell
kk apply -f deployment.yaml
kk set image deployment/hellok8s-go-http hellok8s=leigg/hellok8s:readiness
```

现在可以发现两个 pod 一直处于没有 Ready 的状态当中，通过 describe
命令可以看到是因为 `Readiness probe failed: HTTP probe failed with statuscode: 500 `的原因。
又因为设置了最大不可用的服务数量为maxUnavailable=1，这样能保证剩下两个 v2 版本的 hellok8s 能继续提供服务。

```shell
$ kk get pods                                                       
NAME                                READY   STATUS    RESTARTS   AGE
hellok8s-go-http-764849969-9rtdw    1/1     Running   0          10m
hellok8s-go-http-764849969-qfqds    1/1     Running   0          10m
hellok8s-go-http-7b778ccdcd-c9kv4   0/1     Running   0          5s
hellok8s-go-http-7b778ccdcd-fn7p6   0/1     Running   0          5s

$ kk describe pod hellok8s-go-http-7b778ccdcd-c9kv4
...
Events:
  Type     Reason     Age                  From               Message
  ----     ------     ----                 ----               -------
  Normal   Scheduled  112s                 default-scheduler  Successfully assigned default/hellok8s-go-http-7b778ccdcd-c9kv4 to k8s-node1
  Normal   Pulled     111s                 kubelet            Container image "leigg/hellok8s:readiness" already present on machine
  Normal   Created    111s                 kubelet            Created container hellok8s
  Normal   Started    111s                 kubelet            Started container hellok8s
  Warning  Unhealthy  21s (x22 over 110s)  kubelet            Readiness probe failed: HTTP probe failed with statuscode: 500
```

### 4.9 更新的暂停与恢复

在更新时，有时候我们希望先更新1个Pod，通过监控各项指标日志来验证没问题后，再继续更新其他Pod。这个需求可以通过暂停和恢复Deployment来解决。

> 这也叫做金丝雀发布。

这里会用到的暂停和恢复命令如下：

```shell
kk rollout pause deploy {deploy-name}
kk rollout resume deploy {deploy-name}
```

测试步骤如下：

```shell
# 一次性执行两条命令
kk set image deploy hellok8s-go-http=leigg/hellok8s:v2
kk rollout pause deploy hellok8s-go-http

# 现在观察更新情况，会发现只有一个pod被更新
kk get pods

# 如果此刻想要回滚
kk rollout undo deployment hellok8s-go-http --to-revision=N

# 若要继续更新
kk rollout resume deploy hellok8s-go-http
```

## 5. 使用DaemonSet

DaemonSet是一种特殊的控制器，它会在每个node上**只会**运行一个pod，
因此常用来部署那些为节点本身提供服务或维护的Pod（如日志收集和转发、监控等）。

正因为它的特殊，所以DaemonSet的pod通常会在配置中直接指定映射到指定node端口，并且可以绕过污点限制从而可以被调度到Master上运行（需要在yaml中配置）。

DaemonSet的yaml文件示例 [daemonset.yaml](./example_deployment/daemonset.yaml)

测试步骤如下：

```shell
kubectl create -f fluentd-daemonset.yaml

# 会看到每个node上都运行一个pod，包含master
kubectl get pods -n kube-system -o wide

# 所有pod正常运行后，编辑pod配置(编辑语法同vi)
kubectl edit ds/fluentd-elasticsearch -n kube-system

# 观察pod数量和状态
kubectl get pods -n kube-system -o wide

# 观察控制器状态信息
kubectl get daemonset -n kube-system
```

对于Daemonset控制器管理的Pod的更新，都是先（手动或自动）删除再创建，不会进行滚动更新。

## 6. 使用Job和CronJob

Job和CronJob控制器与Deployment、Daemonset都是同级的控制器。它俩都是用来执行一次性任务的，区别在于Job是一次性的，而CronJob是周期性的。

本节笔者使用k8s官方提供的 [playground平台](https://labs.play-with-k8s.com) 来进行测试，简单几步就可以搭建起一个临时的多节点k8s集群，
这里也推荐使用，练习/演示必备。（当然读者也可以使用已经搭建好的集群进行测试）

### 6.1 使用Job

具体来说，Job控制器可以执行3种类型的任务。

- 一次性任务：启动一个Pod（除非启动失败）。一旦Pod成功终止，Job就算完成了。
- 串行式任务：连续、多次地执行某个任务，上一个任务完成后，立即执行下个任务，直到全部执行完。
- 并行式任务：可以通过spec.completions属性指定执行次数。

使用 [job.yaml](example_job/job.yaml) 测试**一次性任务**：

```shell
[node1 ~]$ kubectl apply -f job.yaml 
job.batch/pods-job created

[node1 ~]$ kubectl get job
NAME     COMPLETIONS  DURATION   AGE
pods-job   0/1           19s     19s

# DURATION 表示job启动到结束耗时
[node1 ~]$ kubectl get job
NAME     COMPLETIONS   DURATION   AGE
pods-job   1/1           36s     60s

# Completed 表示pod正常终止
[node1 ~]$ kubectl get pods
NAME                    READY   STATUS      RESTARTS   AGE
pods-simple-pod-kdjr6   0/1     Completed   0          4m41s

# 查看pod日志（标准输出和错误）
[node1 ~]$ kubectl logs pods-simple-pod-kdjr6
Start Job!
Job Done!

# 执行结束后，手动删除job，也可在yaml中配置自动删除
[node1 ~]$ kubectl delete job pods-job
job.batch "pods-job" deleted
```

配置文件中启动`completions`字段来设置任务需要执行的总次数（串行式任务），启动`parallelism`字段来设置任务并发数量（并行式任务）。

**处理异常情况**   
任务执行失败，可以通过`backoffLimit`字段设置失败重试次数，默认是6次。并且推荐设置`restartPolicy`为Never（而不是OnFailure），
这样可以保留启动失败的Pod，以便排查日志。

### 6.2 使用CronJob

它是基于Job的更高级的控制器，添加了时间管理功能。可以实现：

- 在未来某个指定时间运行一次Job
- 周期性运行Job

使用 [job.yaml](example_job/cronjob.yaml) 测试：

```shell
[node1 ~]$ kubectl apply -f cronjob.yaml 
job.batch/pods-cronjob created

[node1 ~]$ kubectl get cronjob
NAME           SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
pods-cronjob   */1 * * * *   False     1        28s             10s

# cronjob内部还是调用的job
[node1 ~]$ kubectl get job
NAME                    COMPLETIONS   DURATION   AGE
pods-cronjob-28305226   1/1           34s        2m54s
pods-cronjob-28305227   1/1           34s        114s
pods-cronjob-28305228   1/1           34s        54s

# 删除cronjob，会自动删除关联的job, pod
[node1 ~]$ kubectl delete cronjob pods-cronjob
cronjob.batch "pods-cronjob" deleted
[node1 ~]$ kubectl get job
No resources found in default namespace.
```

### 6.3 其他控制器

除了前面介绍的Deployment、DaemonSet、Job和CronJob控制器，其他还有：

- ReplicationController和ReplicaSetController
- StatefulController

**关于ReplicationController和ReplicaSetController**  
在早期的k8s版本中，ReplicationController是最早提供的控制器，后来ReplicaSetController出现并替代了前者，二者没有本质上的区别，
后者支持复合式的selector。在Deployment出现后，由于它们缺少其他后来新增控制器的更细粒度的生命周期管理功能，
导致ReplicationController和ReplicaSetController已经很少使用，但仍然保留下来。

在后来的版本中，一般都是创建Deployment控制器，由它自动托管ReplicaSetController，用户无需操心后者（但可以命令查看）。
ReplicaSetController也可通过模板创建，可自行查询。需要注意的是，手动创建的ReplicaSetController不能由Deployment控制器托管，
所以ReplicaSetController也不具有滚动更新、版本查看和回滚功能。

**StatefulController**  
这是一种提供排序和唯一性保证的特殊Pod控制器，将在后面的章节中进行介绍。

下一节，将介绍前面这些 Controller 控制的Pod集合如何有效且稳定的对外暴露服务。

## 7. 使用Service

先提出几个问题：

- 在前面的内容中，我们通过`port-forward`的临时方式来访问pod，需要指定某个pod名称，而如果pod发生扩容或重启，pod名称就会变化，
  那如何获取稳定的pod访问地址呢？
- deployment通常会包含多个pod，如何进行负载均衡？

`Service` 就是用来解决上述问题的。

kubernetes 提供了一种名叫 `Service` 的资源帮助解决这些问题，它为 pod 提供一个稳定的 Endpoint。`Service` 位于 pod 的前面，
负责接收请求并将它们传递给它后面的所有pod。一旦服务中的 Pod 集合发生更改，Endpoints 就会被更新，请求的重定向自然也会导向最新的
pod。

> `Service`为Pod提供了网络访问、负载均衡以及服务发现等功能。

### 7.1 不同类型的Service

Kubernetes提供了多种类型的Service，包括ClusterIP、NodePort、LoadBalancer和ExternalName，每种类型服务不同的需求和用例。
Service类型的选择取决于你的应用程序的具体要求以及你希望如何将其暴露到网络中。

- ClusterIP:
    - 原理：使用这种方式发布时，会为Service提供一个固定的集群内部虚拟IP，供集群内访问。
    - 场景：内部数据库服务、内部API服务等。
- ClusterIP（Headless版）:
    - 原理：这种方式不会分配ClusterIP，也不会通过Kube-proxy进行反向代理和负载均衡，而是通过DNS提供稳定的网络ID来访问，
      并且DNS会将无头Service的后端解析为Pod的后端IP列表，也仅供集群内访问
    - 场景：一般提供给StatefulSet使用。
- NodePort:
    - 原理：通过每个节点上的 IP 和静态端口发布服务。 这是一种基于ClusterIP的发布方式，因为它应用后首先会生成一个集群内部IP，
        然后再将其绑定到节点的IP和端口，这样就可以在集群外通过节点IP:端口的方式访问服务。
    - 场景：Web应用程序、REST API等。
- LoadBalancer:
    - 原理：这种方式又基于ClusterIP和NodePort两种方式，另外还会使用到外部由云厂商提供的负载均衡器。由后者向外发布Service。
      一般在使用云平台提供的Kubernetes集群时，会用到这种方式。
    - 场景：Web应用程序、公开的API服务等。
- ExternalName:
    - 原理：与上面提到的发布方式不太相同，这种方式是将外部服务引入集群内部，为集群内提供服务。
    - 场景：连接到外部数据库服务、外部认证服务等。

### 7.2 Service类型之ClusterIP

ClusterIP通过分配集群内部IP来在集群内暴露服务（集群外不能访问），这样就可以在集群内通过集群IP+端口访问到pod服务。

>这种方式适用于那些不需要对外暴露的服务，如节点守护agent等。

准备工作：

1. 修改main.go为 [main_hostname.go](main_hostname.go)
2. 重新构建和推送镜像

```shell
docker build . -t leigg/hellok8s:v3_hostname
docker push leigg/hellok8s:v3_hostname
```

3. 更新deployment使用的image

```shell
kk set image deployment/hellok8s-go-http hellok8s=leigg/hellok8s:v3_hostname

# 等待pod更新
kk get pods --watch
```

4. deployment更新成功后，编写 `Service` 配置文件 [service-clusterip.yaml](service-clusterip.yaml)
5. 应用`Service` 配置文件，并观察 `Endpoint`

```shell
kk apply -f service-clusterip.yaml

$ kk get svc
NAME                         TYPE        CLUSTER-IP    EXTERNAL-IP   PORT(S)    AGE
kubernetes                   ClusterIP   20.1.0.1      <none>        443/TCP    11h
service-hellok8s-clusterip   ClusterIP   20.1.120.16   <none>        3000/TCP   20s

$ kk get endpoints                  
NAME                         ENDPOINTS                         AGE
kubernetes                   10.0.2.2:6443                     6h54m
service-hellok8s-clusterip   20.2.36.72:3000,20.2.36.73:3000   6m38s
```
这里通过`kk get svc`获取到的就是集群内`default`空间下的service列表，我们发布的自然是第二个，它的ClusterIP是`20.1.120.16`，
这个IP是可以在节点直接访问的：
```shell
$ curl 20.1.120.16:3000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-dstff
# 多次访问，会观察到hostname变化，说明service进行了负载均衡
$ curl 20.1.120.16:3000 
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-wtdht
```

然后我们通过`kk get endpoints`获取到的是Service后端的逻辑Pod组的信息，`ENDPOINTS`列中包含的两个地址则是两个就绪的pod的访问地址（这个IP也是Pod网段，节点无法直接访问），
这些端点是和就绪的pod保持实时一致的（Service会实时跟踪），下面通过扩缩容来观察。

```shell
$ kk scale deployment/hellok8s-go-http --replicas=3                      
deployment.apps/hellok8s-go-http scaled

$ kk get endpoints                                      
NAME                         ENDPOINTS                                         AGE
kubernetes                   10.0.2.2:6443                                     7h3m
service-hellok8s-clusterip   20.2.36.72:3000,20.2.36.73:3000,20.2.36.74:3000   15m

$ kk scale deployment/hellok8s-go-http --replicas=2
deployment.apps/hellok8s-go-http scaled

# 注意pod ip可能发生变化
$ kk get endpoints                                      
NAME                         ENDPOINTS                         AGE
kubernetes                   10.0.2.2:6443                     7h5m
service-hellok8s-clusterip   20.2.36.72:3000,20.2.36.75:3000   17m
```

`ClusterIP`除了在节点上可直接访问，在集群内也是可以访问的。下面启动一个Nginx Pod来访问这个虚拟的ClusterIP （`20.1.120.16`）。

1. 定义 [pod_nginx.yaml](pod_nginx.yaml)，并应用它，不再演示。(
   提前在node上拉取镜像：`ctr images pull docker.io/library/nginx:latest`)
2. 进入nginx pod shell，尝试访问 `service-hellok8s-clusterip`提供的endpoint

```shell
$ kk get pods --watch
NAME                                READY   STATUS    RESTARTS   AGE
hellok8s-go-http-6bb87f8cb5-dstff   1/1     Running   0          27m
hellok8s-go-http-6bb87f8cb5-wtdht   1/1     Running   0          11m
nginx                               1/1     Running   0          11s

# 进入 nginx pod
$ kk exec -it nginx -- bash 
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.

# 访问 hellok8s 的 cluster ip
root@nginx:/# curl 20.1.120.16:3000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-dstff
root@nginx:/# curl 20.1.120.16:3000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-wtdht
```

**Service访问及负载均衡原理**  
如果还记得文章开头的架构图，就会发现每个节点都运行着一个kube-proxy组件，这个组件会跟踪Service和Pod的动态变化，并且最新
的Service和Pod的映射关系会被记录到iptables中，这样每个节点上的iptables规则都会被更新。而iptables使用NAT技术将虚拟IP的流量转发到Endpoint。

通过在master节点（其他节点也可）`iptables -L -v -n -t nat`可以查看其配置，这个结果会很长。这里贴出关键的两条链：
```shell
$ iptables -L -v -n -t nat
...
Chain KUBE-SERVICES (2 references)
 pkts bytes target     prot opt in     out     source               destination         
    0     0 KUBE-SVC-JD5MR3NA4I4DYORP  tcp  --  *      *       0.0.0.0/0            20.1.0.10            /* kube-system/kube-dns:metrics cluster IP */ tcp dpt:9153
    6   360 KUBE-SVC-BRULDGNIV2IQDBPU  tcp  --  *      *       0.0.0.0/0            20.1.120.16          /* default/service-hellok8s-clusterip cluster IP */ tcp dpt:3000
    0     0 KUBE-SVC-NPX46M4PTMTKRN6Y  tcp  --  *      *       0.0.0.0/0            20.1.0.1             /* default/kubernetes:https cluster IP */ tcp dpt:443
    0     0 KUBE-SVC-TCOU7JCQXEZGVUNU  udp  --  *      *       0.0.0.0/0            20.1.0.10            /* kube-system/kube-dns:dns cluster IP */ udp dpt:53
    0     0 KUBE-SVC-ERIFXISQEP7F7OF4  tcp  --  *      *       0.0.0.0/0            20.1.0.10            /* kube-system/kube-dns:dns-tcp cluster IP */ tcp dpt:53
 1079 64740 KUBE-NODEPORTS  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* kubernetes service nodeports; NOTE: this must be the last rule in this chain */ ADDRTYPE match dst-type LOCAL

Chain KUBE-SVC-BRULDGNIV2IQDBPU (1 references)
 pkts bytes target     prot opt in     out     source               destination         
    6   360 KUBE-MARK-MASQ  tcp  --  *      *      !20.2.0.0/16          20.1.120.16          /* default/service-hellok8s-clusterip cluster IP */ tcp dpt:3000
    2   120 KUBE-SEP-JCBKJJ6OJ3DPB6OD  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/service-hellok8s-clusterip -> 20.2.36.77:3000 */ statistic mode random probability 0.50000000000
    4   240 KUBE-SEP-YHSEP23J6IVZKCOG  all  --  *      *       0.0.0.0/0            0.0.0.0/0            /* default/service-hellok8s-clusterip -> 20.2.36.78:3000 */
...
```
这里有 `KUBE-SERVICES`和 `KUBE-SVC-BRULDGNIV2IQDBPU`两条链，前者引用了后者，在第一条链中，可以看到 **target**为`20.1.120.16`(ClusterIP)的流量将转发至3个目标 `KUBE-SVC-BRULDGNIV2IQDBPU`：
- 第一条规则会对除了 20.2.0.0/16 地址范围之外的且目标是3000端口的所有来源的tcp协议数据包执行MASQ动作，即NAT操作（把数据包的源IP转换为目标IP）
- 第二条规则将任意链内流量转发到目标`KUBE-SEP-JCBKJJ6OJ3DPB6OD`，尾部`probability`说明应用此规则的概率是0.5
- 第三条规则将任意链内流量转发到目标`KUBE-SEP-YHSEP23J6IVZKCOG`，概率也是0.5（1-0.5）
而这2和3两个规则中的目标其实就是指向两个后端Pod IP，可通过`iptables-save | grep KUBE-SEP-YHSEP23J6IVZKCOG`查看其中一个目标明细：
```shell
$ iptables-save | grep KUBE-SEP-YHSEP23J6IVZKCOG
:KUBE-SEP-YHSEP23J6IVZKCOG - [0:0]
-A KUBE-SEP-YHSEP23J6IVZKCOG -s 20.2.36.78/32 -m comment --comment "default/service-hellok8s-clusterip" -j KUBE-MARK-MASQ
-A KUBE-SEP-YHSEP23J6IVZKCOG -p tcp -m comment --comment "default/service-hellok8s-clusterip" -m tcp -j DNAT --to-destination 20.2.36.78:3000
-A KUBE-SVC-BRULDGNIV2IQDBPU -m comment --comment "default/service-hellok8s-clusterip -> 20.2.36.78:3000" -j KUBE-SEP-YHSEP23J6IVZKCOG

$ kk get pods -o wide
NAME                                READY   STATUS    RESTARTS   AGE   IP           NODE        NOMINATED NODE   READINESS GATES
hellok8s-go-http-6bb87f8cb5-dstff   1/1     Running   0          53m   20.2.36.77   k8s-node1   <none>           <none>
hellok8s-go-http-6bb87f8cb5-wtdht   1/1     Running   0          52m   20.2.36.78   k8s-node1   <none>           <none>
```
可以看到链`KUBE-SEP-YHSEP23J6IVZKCOG`的规则之一就是将转入的流量全部转发到目标`20.2.36.78:3000`，这个IP也是名字为`hellok8s-go-http-6bb87f8cb5-wtdht`的Pod的内部IP。

### 7.3 Service类型之NodePort

`ClusterIP`只能在集群内访问Pod服务，而`NodePort`则进一步将服务暴露到集群外部的节点的固定端口上。

比如K8s集群有2个节点：node1, node2，暴露后就可以通过 `node1-ip:port` 或 `node2-ip:port` 的方式来稳定访问Pod服务。

准备工作：

1. 删除已经创建的`ClusterIP`类型的Service，减少干扰（执行：`kk delete -f service-clusterip.yaml`）；
2. 定义 [service-nodeport.yaml](service-nodeport.yaml)，并应用；
3. 现在可以通过访问k8s集群中的任一节点ip+端口进行验证

```shell
# 同样会分配一个 cluster-ip
$ kk get svc service-hellok8s-nodeport                   
NAME                        TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)          AGE
service-hellok8s-nodeport   NodePort    20.1.252.217   <none>        3000:30000/TCP   79s

# 在节点10.0.2.2 上访问 本机端口 以及 节点 10.0.2.3:30000
# - 同样每个ip访问2次验证负载均衡功能
$ curl 10.0.2.2:30000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-hx7pv
$ curl 10.0.2.2:30000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-4bddw

$ curl 10.0.2.3:30000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-hx7pv
$ curl 10.0.2.3:30000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-4bddw
```

### 7.4 Service类型之LoadBalancer

`LoadBalancer` 是通过使用云提供商的负载均衡器（一般叫做SLB，Service LoadBalancer）的方式向外暴露服务。
负载均衡器可以将集群外的流量转发到集群内的Pod，
假如你在 AWS 的 EKS 集群上创建一个 Type 为 LoadBalancer 的 Service。它会自动创建一个 ELB (Elastic Load Balancer)
，并可以根据配置的 IP 池中自动分配一个独立的 IP 地址，可以供外部访问。

这一步无条件，不再演示，LoadBalancer架构图如下：

<div align="center">
<img src="img/k8s-loadbalancer.png" width = "600" height = "700" alt=""/>
</div>

`LoadBalancer`类型的Service有点像`ClusterIP`和`NodePort`的结合，它监听了节点的随机端口，并转发端口流量到后端Pod。

如果是使用公有云维护的K8s集群（不是自己搭建），那么通常也会使用它们提供的SLB服务（即会用到`LoadBalancer`）。若是自己搭建的集群，
那么一般也不会使用`LoadBalancer`，而是使用 DaemonSet+HostNetwork+nodeSelector 来向外暴露服务 （后续介绍）。

- [阿里云使用私网SLB教程](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/configure-an-ingress-controller-to-use-an-internal-facing-slb-instance?spm=a2c4g.11186623.0.0.5d1736e0l59zqg)

### 7.5 Service类型之ExternalName

`ExternalName`是k8s中一个特殊的service类型，它不需要指定selector去选择哪些pods实例提供服务，而是使用DNS
CNAME机制把自己CNAME到你指定的另外一个域名上，你可以提供集群内的名字，
比如`mysql.db.svc`这样的建立在db命名空间内的mysql服务，也可以指定`http://mysql.example.com`这样的外部真实域名。

比如可以定义一个 `Service` 指向 `ifconfig.me` （一个curl访问可获取自己公网IP的公共地址），然后可以在集群内的任何一个pod上访问这个service的名称，
请求将自动转发到`ifconfig.me`。

> 注意`ExternalName`这个类型也仅在集群内生效，在节点上是无法访问service名称的。

准备工作：

1. 定义 [service-externalname.yaml](service-externalname.yaml)，并应用
2. 定义 [pod_busybox.yaml](pod_busybox.yaml) 并应用，用来作为client访问定义好的service
2. 验证步骤如下：

```shell
# 进入busybox pod
$ kk exec -it busybox -- sh            

# 使用 service名称 作为dns地址 进行查找
/ $ nslookup cloud-mysql-svc
Server:		20.1.0.10
Address:	20.1.0.10:53

** server can't find cloud-mysql-svc.cluster.local: NXDOMAIN

** server can't find cloud-mysql-svc.svc.cluster.local: NXDOMAIN

** server can't find cloud-mysql-svc.cluster.local: NXDOMAIN

** server can't find cloud-mysql-svc.svc.cluster.local: NXDOMAIN

cloud-mysql-svc.default.svc.cluster.local	canonical name = mysql-s23423.db.tencent.com

cloud-mysql-svc.default.svc.cluster.local	canonical name = mysql-s23423.db.tencent.com

# 如结果所示，最终在 default.svc.cluster.local 这个域内找到了service配置的外部地址

# 也可使用完整域名查找
/ $ nslookup cloud-mysql-svc.default.svc.cluster.local
Server:		20.1.0.10
Address:	20.1.0.10:53

cloud-mysql-svc.default.svc.cluster.local	canonical name = mysql-s23423.db.tencent.com

cloud-mysql-svc.default.svc.cluster.local	canonical name = mysql-s23423.db.tencent.com
```

**用途说明**：`ExternalName`类Service一般用在集群内部需要调用外部服务的时候，比如云服务商部署的DB等服务。

> 注意：观察`cloud-mysql-svc.default.svc.cluster.local`这个域名组成：
> - `cloud-mysql-svc` 是服务名
> - `default` 是集群 namespace
> - `svc.cluster.local` 是service默认域  
    > 也就是说，修改集群 namespace 字段我们就可以实现跨集群 namespace 的Pod访问。

>
另外，很多时候，比如是自己部署的DB服务，只有IP而没有域名，ExternalName是无法实现这个需求的，需要使用 `无头Service`+`Endpoints`
来实现，请看后续。

## 8. 使用Ingress

`Ingress` 是一种用于管理和公开集群内服务的 API 对象。它充当了对集群中的服务进行外部公开和流量路由的入口点。
`Ingress` 允许你配置规则以指定服务之间的路径和主机名路由，从而可以根据 URL 路径和主机名将请求路由到不同的后端服务。

Ingress具有 TLS/SSL 支持：你可以为 Ingress 配置 TLS 证书，以加密传输到后端服务的流量下功能：

- **路由规则**：Ingress 允许你定义路由规则，使请求根据主机名和路径匹配路由到不同的后端服务。这使得可以在同一 IP
  地址和端口上公开多个服务。
- **Rewrite 规则**：Ingress 支持 URL 重写，允许你在路由过程中修改请求的 URL 路径；
- **TLS/SSL 支持**：你可以为 Ingress 配置 TLS 证书，以加密传输到后端服务的流量；
- **负载均衡**：Ingress 可以与云提供商的负载均衡器集成，以提供外部负载均衡和高可用性；
- **虚拟主机**：你可以配置多个主机名（虚拟主机）来公开不同的服务。这意味着你可以在同一 IP 地址上托管多个域名；
- **自定义错误页面**：你可以定义自定义错误页面，以提供用户友好的错误信息；
- **插件和控制器**：社区提供了多个 Ingress 控制器，如 Nginx Ingress Controller 和 Traefik，它们为 Ingress 提供了更多功能和灵活性。

`Ingress` 可以简单理解为后端集群服务的 网关（Gateway），它是所有流量的入口，经过配置的路由规则，将流量重定向到后端的服务。

> 相对于Ingress，service类型之一的NodePort转发流量的方式比较单一，仅支持节点的特定端口到特定service的流量转发，并且不支持编写路由规则、域名配置等重要功能。

### 8.1 关于Ingress控制器

它指的是Ingress的具体实现，像简介中说的路由、rewrite等功能都是k8s ingress定义的通用功能，但k8s并不负责实现这些功能。
它把具体实现交给第三方，以提供灵活性和可定制化。

常见的Ingress控制器实现有：Nginx Ingress、APISIX Ingress、BFE
Ingress等，[点击链接](https://kubernetes.io/zh-cn/docs/concepts/services-networking/ingress-controllers/) 查看更多。

### 8.2 安装Nginx Ingress控制器

传统架构中常用Nginx作为外部网关，所以这里也使用Nginx作为Ingress控制器来练习。

- [官方仓库](https://github.com/kubernetes/ingress-nginx)
- [官方安装指导](https://kubernetes.github.io/ingress-nginx/deploy/)

先通过官方仓库页面的版本支持表确认控制器与k8s匹配的版本信息，笔者使用的k8s版本是`1.25.14`，准备安装的Nginx
ingress控制器版本是`1.8.2`。

安装方式有Helm安装和手动安装，Helm是一个很好用的k8s包管理器（后续介绍），但这里先使用手动安装。

```shell
# 下载Nginx Ingress控制器安装文件
wget https://raw.gitmirror.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml

# 安装
kubectl apply -f deploy.yaml

# 等待控制器的pod运行正常（这里自动创建了一个新的namespace）
$ kubectl get pods --namespace=ingress-nginx --watch
NAME                                        READY   STATUS      RESTARTS   AGE
ingress-nginx-admission-create-kt8lm        0/1     Completed   0          2m36s
ingress-nginx-admission-patch-rslxl         0/1     Completed   2          2m36s
ingress-nginx-controller-6f4df7b5d6-lxfsr   1/1     Running     0          2m36s

# 注意前两个 Completed 的pod是一次性的，用于执行初始化工作，现在安装成功。

#查看安装的所有资源
$ kubectl get all -n ingress-nginx
NAME                                            READY   STATUS      RESTARTS   AGE
pod/ingress-nginx-admission-create-smxkz        0/1     Completed   0          16m
pod/ingress-nginx-admission-patch-7c86x         0/1     Completed   1          16m
pod/ingress-nginx-controller-6f4df7b5d6-pz8cp   1/1     Running     0          16m

NAME                                         TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)                      AGE
service/ingress-nginx-controller             LoadBalancer   20.1.115.216   <pending>     80:31888/TCP,443:30158/TCP   16m
service/ingress-nginx-controller-admission   ClusterIP      20.1.102.149   <none>        443/TCP                      16m

NAME                                       READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/ingress-nginx-controller   1/1     1            1           16m

NAME                                                  DESIRED   CURRENT   READY   AGE
replicaset.apps/ingress-nginx-controller-6f4df7b5d6   1         1         1       16m

NAME                                       COMPLETIONS   DURATION   AGE
job.batch/ingress-nginx-admission-create   1/1           5s         16m
job.batch/ingress-nginx-admission-patch    1/1           7s         16m
```

这里重点关注`service/ingress-nginx-controller`这一行，这是Nginx Ingress自动创建的`LoadBalancer`类型的service，
它负责实现转发节点流量到 pod `ingress-nginx-controller`，后者再转发流量到 `service-hellok8s-clusterip`，然后最终到达业务pod。

所以Nginx Ingress Controller启动后会默认监听节点的两个随机端口（这里是31888/30158），分别对应其Pod内的80/443，
后面讲如何修改为节点固定端口。

### 8.3 开始测试

准备工作：

1. 修改main.go为 [main_nginxingress.go](main_nginxingress.go)
2. 重新构建并推送镜像

```shell
docker build . -t leigg/hellok8s:v3_nginxingress
docker push leigg/hellok8s:v3_nginxingress
```

3. 更新deployment镜像：`kubectl set image deployment/hellok8s-go-http hellok8s=leigg/hellok8s:v3_nginxingress`，并等待更新完成
4. 恢复之前的ClusterIP类型的`Service`
4. 定义 Ingress yaml文件 [ingress-hellok8s.yaml](ingress-hellok8s.yaml)，并在其中定义路由规则，然后应用
5. 在集群节点上验证

```shell
$ kk get svc service-hellok8s-clusterip                       
NAME                         TYPE           CLUSTER-IP     EXTERNAL-IP                   PORT(S)          AGE
service-hellok8s-clusterip   ClusterIP      20.1.106.177   <none>                        3000/TCP         5s

# 这里的80端口并不是指节点端口，而是控制器pod内监听的端口
$ kk get ingress           
NAME               CLASS   HOSTS   ADDRESS   PORTS   AGE
hellok8s-ingress   nginx   *                 80      2m

# 现在可以直接访问节点的转发端口（访问集群任一节点均可，端口一致）
$ curl 10.0.2.3:31888/hello      
[v3] Hello, Kubernetes!, host:hellok8s-go-http-6df8b5c5d7-h76jl
$ curl 10.0.2.3:31888/ingress/123
[v3] Hello, Kubernetes!, path:/ingress/123
$ curl 10.0.2.3:31888/hello123   
/hello123 is not found, 404
```

若要更新路由规则，修改yaml文件后再次应用即可，通过`kk logs -f ingress-nginx-controller-xxx -n ingress-nginx`可以看到路由访问日志。

这里列出几个常见的配置示例：

- [证书配置：ingress-hellok8s-cert.yaml](ingress-hellok8s-cert.yaml)
- [默认后端：ingress-hellok8s-defaultbackend.yaml](ingress-hellok8s-defaultbackend.yaml)
- [正则匹配：ingress-hellok8s-regex.yaml](ingress-hellok8s-regex.yaml)

### 8.4 Ingress高可靠部署

一般通过多节点部署的方式来实现高可靠，同时Ingress作为业务的流量入口，也建议一个ingress服务独占一个节点的方式进行部署，
避免业务服务与ingress服务发生资源争夺。

> 也就是说，单独使用一台机器来部署ingress服务，这台机器可以是较低计算性能（如2c4g），但需要较高的上行带宽。

然后再根据业务流量规模（定期观察ingress节点的上行流量走势）进行ingress节点扩缩容。若前期规模不大，也可以ingress节点与业务节点混合部署的方式，
但要注意进行资源限制和隔离。

**下面给出常用指令，根据需要使用**。

Ingress控制器扩容：

```shell
kk -n kube-system scale --replicas=3 deployment/nginx-ingress-controller
```

指定节点部署ingress（通过打标签）:

```shell
$ kk label nodes k8s-node1 ingress="true"
$ kk get node k8s-node1 --show-labels
NAME        STATUS   ROLES    AGE     VERSION    LABELS
k8s-node1   Ready    <none>   2d22h   v1.25.14   beta.kubernetes.io/arch=amd64,beta.kubernetes.io/os=linux,ingress=true,kubernetes.io/arch=amd64,kubernetes.io/hostname=k8s-node1,kubernetes.io/os=linux

# 修改ingress部署文件，搜索Deployment，在其spec.template.spec.nodeSelector下面添加 ingress: "true"
$ vi deploy.yaml 
#apiVersion: apps/v1
#kind: Deployment
#...
#   nodeSelector:
#    kubernetes.io/os: linux
#    ingress: "true"  # <----- 添加这行
#...

$ kk apply -f deploy.yaml # 更新部署
```

>
注意：默认不能部署到master节点，存在污点问题，需要移除污点才可以。参考 [k8s-master增加和删除污点](https://www.cnblogs.com/zouhong/p/17351418.html)

### 8.5 Ingress部署方案推荐

1. **Deployment + `LoadBalancer` 模式的 Service**  
   介绍：如果要把ingress部署在公有云，那用这种方式比较合适。用Deployment部署ingress-controller，创建一个 type为 LoadBalancer
   的 service 关联这组 pod。大部分公有云，都会为 LoadBalancer 的 service 自动创建一个负载均衡器，通常还绑定了公网地址。
   只要把域名解析指向该地址，就实现了集群服务的对外暴露。

2. **DaemonSet + HostNetwork + nodeSelector**  
   介绍：用DaemonSet结合nodeselector来部署ingress-controller到特定的node上，然后使用HostNetwork直接把该pod与宿主机node的网络打通，直接使用宿主机的80/433端口就能访问服务。这时，ingress-controller所在的node机器就很类似传统架构的边缘节点，比如机房入口的nginx服务器。该方式整个请求链路最简单，性能相对NodePort模式更好。
   缺点是由于直接利用宿主机节点的网络和端口，一个node只能部署一个ingress-controller pod， 比较适合大并发的生产环境使用。

3. **Deployment + `NodePort`模式的Service**  
   介绍：同样用Deployment模式部署ingress-controller，并创建对应的service，但是type为NodePort。这样，Ingress就会暴露在集群节点ip的特定端口上。
   然后可以直接将Ingress节点IP填到域名CNAME记录中。

## 9. 使用Namespace

Namespace（命名空间）用来隔离集群内不同环境下的资源。仅同一namespace下的资源命名需要唯一，它的作用域仅针对带有名字空间的对象，例如
Deployment、Service 等。

前面的教程中，默认使用的 namespace 是 default。

创建多个namespace：

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: dev

---

apiVersion: v1
kind: Namespace
metadata:
  name: test
```

使用：

```shell
$ kubectl apply -f namespaces.yaml    
# namespace/dev created
# namespace/test created


$ kubectl get namespaces          
# NAME              STATUS   AGE
# default           Active   215d
# dev               Active   2m44s
# ingress-nginx     Active   110d
# kube-node-lease   Active   215d
# kube-public       Active   215d
# kube-system       Active   215d
# test              Active   2m44s

# 获取指定namespace下的资源
$ kubectl get pods -n dev
```

## 10. 使用ConfigMap

K8s 使用 ConfigMap 来将你的配置数据和应用程序代码分开，将非机密性的数据保存到键值对中。ConfigMap 在设计上不是用来保存大量数据的。
在 ConfigMap 中保存的数据不可超过 1 MiB。如果你需要保存超出此尺寸限制的数据，你可能考虑挂载存储卷。

下面使用ConfigMap来保存`default`空间下的数据库连接地址：

```yaml
# configmap-hellok8s.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: hellok8s-config
  namespace: default # 可省略
data:
  DB_URL: "http://DB_ADDRESS"
```

然后在 deployment
配置中添加读取env的配置，并且env从configmap中读取，具体看 [deployment-use-configmap.yaml](deployment-use-configmap.yaml)。

然后修改main.go为 [main_readenv.go](main_readenv.go)，接着重新构建并推送镜像：

```shell
docker build . -t leigg/hellok8s:v4
docker push leigg/hellok8s:v4

$ kk apply -f deployment.yaml
deployment.apps/hellok8s-go-http configured

$ kk get svc                    
NAME                         TYPE           CLUSTER-IP     EXTERNAL-IP                   PORT(S)          AGE
cloud-mysql-svc              ExternalName   <none>         mysql-s23423.db.tencent.com   <none>           2d14h
kubernetes                   ClusterIP      20.1.0.1       <none>                        443/TCP          3d1h
service-hellok8s-clusterip   ClusterIP      20.1.106.177   <none>                        3000/TCP         30h
service-hellok8s-nodeport    NodePort       20.1.252.217   <none>                        3000:30000/TCP   2d17h

# 通过nodeport方式访问服务
$ curl 10.0.2.3:30000     
[v4] Hello, Kubernetes! From host: hellok8s-go-http-6649fc59cd-blt75, Get Database Connect URL: http://DB_ADDRESS
```

可以看到app已经拿到了configmap中定义的env变量。若要更新env，直接更改configmap的yaml文件然后应用，然后删除业务pod即可。

## 参考

- [k8s教程](https://github.com/guangzhengli/k8s-tutorials/blob/main/docs/pre.md)
- [Kubernetes从入门到实践 @赵卓](https://www.epubit.com/bookDetails?id=UB72096269c1157)
- [Docker教程](https://yeasy.gitbook.io/docker_practice/)
- [kubectl全部命令-官方](https://kubernetes.io/docs/reference/kubectl/cheatsheet/)
- [K8s对外服务之Ingress](http://www.uml.org.cn/yunjisuan/202303134.asp?artid=25653)
