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
- [使用k8s playground](https://labs.play-with-k8s.com/)

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

> 如果是生产部署，则不会使用docker官方仓库，而是使用harbor等项目搭建本地仓库，以保证稳定拉取镜像。

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

操作步骤如下：

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

操作步骤如下：

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

Kubernetes提供了多种类型的Service，包括ClusterIP、NodePort、LoadBalancer、Headless和ExternalName，每种类型服务不同的需求和用例。
Service类型的选择取决于你的应用程序的具体要求以及你希望如何将其暴露到网络中。

- ClusterIP:
    - 原理：使用这种方式发布时，会为Service提供一个固定的集群内部虚拟IP，供集群内（包含节点）访问。
    - 场景：内部数据库服务、内部API服务等。
- NodePort:
    - 原理：通过每个节点上的 IP 和静态端口发布服务。 这是一种基于ClusterIP的发布方式，因为它应用后首先会生成一个集群内部IP，
      然后再将其绑定到节点的IP和端口，这样就可以在集群外通过 `nodeIp:port` 的方式访问服务。
    - 场景：Web应用程序、REST API等。
- LoadBalancer:
    - 原理：这种方式又基于 NodePort，另外还会使用到外部由云厂商提供的负载均衡器。由后者向外发布 Service。
      一般在使用云平台提供的Kubernetes集群时，会用到这种方式。
    - 场景：Web应用程序、公开的API服务等。
- Headless:
    - 原理：这种方式不会分配任何集群IP，也不会通过Kube-proxy进行反向代理和负载均衡，而是通过DNS提供稳定的网络ID来访问，
      并且DNS会将无头Service的后端解析为Pod的后端IP列表，以供集群内访问（不含节点），属于**向内发布**。
    - 场景：一般提供给StatefulSet使用。
- ExternalName:
    - 原理：与上面提到的发布方式不太相同，这种方式是通过CNAME机制将外部服务引入集群内部，为集群内提供服务，属于**向内发布**。
    - 场景：连接到外部数据库服务、外部认证服务等。

### 7.2 Service类型之ClusterIP

ClusterIP通过分配集群内部IP来在集群内（包含节点）暴露服务，这样就可以在集群内通过 `clusterIP:port` 访问到pod服务，集群外则无法访问。

> 这种方式适用于那些不需要对外暴露的服务，如节点守护agent等。

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
5. 应用Service配置文件，并观察Endpoint资源

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

然后我们通过`kk get endpoints`获取到的是Service后端的逻辑Pod组的信息，`ENDPOINTS`
列中包含的两个地址则是两个就绪的pod的访问地址（这个IP是Pod专属网段，节点无法直接访问），
这些端点是和就绪的pod保持一致的（Service会实时跟踪），下面通过控制Pod数量增减来观察。

> 在 Kubernetes 中，Endpoints 是一种资源对象，用于指定与一个 Service 关联的后端 Pod 的 IP 地址和端口信息。
> Endpoints 对象充当服务发现机制的一部分，它告诉 Kubernetes 如何将流量路由到 Service 的后端 Pod。
>
> Endpoints一般都是通过Service自动生成的，Service会自动跟踪关联的Pod，当Pod状态发生变化时会自动更新Endpoints。

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
   可提前在node上拉取镜像：`ctr images pull docker.io/library/nginx:latest`)
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

这里有 `KUBE-SERVICES`和 `KUBE-SVC-BRULDGNIV2IQDBPU`两条链，前者引用了后者，在第一条链中，可以看到 **target**
为`20.1.120.16`(ClusterIP)的流量将转发至3个目标 `KUBE-SVC-BRULDGNIV2IQDBPU`：

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

可以看到链`KUBE-SEP-YHSEP23J6IVZKCOG`的规则之一就是将转入的流量全部转发到目标`20.2.36.78:3000`
，这个IP也是名字为`hellok8s-go-http-6bb87f8cb5-wtdht`的Pod的内部IP。

### 7.3 Service类型之NodePort

ClusterIP 只能在集群内访问Pod服务，而 NodePort 则进一步将服务暴露到集群节点的静态端口上。

比如k8s集群有2个节点：node1和node2，暴露后就可以通过 `node1-ip:port` 或 `node2-ip:port` 的方式来稳定访问Pod服务。

操作步骤：

1. 定义 [service-nodeport.yaml](service-nodeport.yaml)，并应用；
2. 通过访问k8s集群中的任一节点ip+端口进行验证

具体指令如下：

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

LoadBalancer 是通过使用云提供商的负载均衡器（一般叫做SLB，Service LoadBalancer）的方式向外暴露服务。
负载均衡器可以将集群外的流量转发到集群内的节点，后者再转发到Pod，
假如你在 AWS 的 EKS 集群上创建一个 Type 为 LoadBalancer 的 Service。它会自动创建一个 ELB (Elastic Load Balancer)
，并可以根据配置的 IP 池中自动分配一个独立的 IP 地址，可以供外部访问。

这一步由于没有条件，不再演示。LoadBalancer 架构图如下：

<div align="center">
<img src="img/k8s-loadbalancer.png" width = "800" height = "400" alt=""/>
</div>

从架构图可看出，`LoadBalancer`是基于 NodePort
的一种Service，这里提供模板供参考：[service-loadbalancer.yaml](service-loadbalancer.yaml)

所以如果是使用公有云托管的k8s集群，那么通常也会使用它们提供的SLB服务。若是自己搭建的集群，
那么一般也不会使用`LoadBalancer`（私有集群一般也不支持`LoadBalancer`）。

- [阿里云使用私网SLB教程](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/configure-an-ingress-controller-to-use-an-internal-facing-slb-instance?spm=a2c4g.11186623.0.0.5d1736e0l59zqg)

### 7.5 Service类型之Headless

这是一种特殊的Service类型，它不分配任何集群IP，而是通过分配的DNS域名来访问Pod服务。由于没有Cluster
IP，所以节点和集群外都无法直接访问Service。
无头Service主要提供给StatefulSet使用。

操作步骤：

1. 定义 [service-clusterip-headless.yaml](service-clusterip-headless.yaml)，并应用；
2. 定义 [pod_curl.yaml](pod_curl.yaml) 并应用（具有curl和nslookup命令），用来作为client访问定义好的service;
3. 进入curl容器，使用curl和nslookup命令进行访问测试；

具体指令如下：

```shell
kk apply -f service-clusterip-headless.yaml

kk apply -f pod_curl.yaml

# 进入curl容器
kk exec -it curl -- /bin/sh
# 访问测试
/ # curl service-hellok8s-clusterip-headless.default.svc.cluster.local:3000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-57r86
/ # curl service-hellok8s-clusterip-headless.default.svc.cluster.local:3000
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-lgtgf

# CNAME记录查询
/ # nslookup service-hellok8s-clusterip-headless.default.svc.cluster.local
nslookup: can't resolve '(null)': Name does not resolve

Name:      service-hellok8s-clusterip-headless.default.svc.cluster.local
Address 1: 20.2.36.77 20-2-36-77.service-hellok8s-clusterip-headless.default.svc.cluster.local
Address 2: 20.2.36.78 20-2-36-78.service-hellok8s-clusterip-headless.default.svc.cluster.local
```

这里的`service-hellok8s-clusterip-headless.default.svc.cluster.local`就是Service提供给集群内部访问Pod组的域名，
组成方式为`{ServiceName}.{Namespace}.svc.{ClusterDomain}`，其中ClusterDomain表示集群域，默认为`cluster.local`，
`Namespace`在Service的yaml文件中未指定那就是default。

在上述操作中，我们通过curl进行了访问测试，可以看到没问题，但是并不提供负载均衡功能，读者可多访问几次进行观察。
然后我们通过nslookup查看域名的DNS信息，可以看到Service域名指向两个Pod IP，并且它们还有对应的专有域名，但因为Pod IP非固定，
所以这个专有域名也没任何作用。

除了直接调用域名访问服务之外，还可解析域名来根据需求决定访问哪个Pod。这种方式更适合StatefulSet产生的有状态Pod。

### 7.6 Service类型之ExternalName

ExternalName 也是k8s中一个特殊的Service类型，它不需要指定selector去选择哪些pods实例提供服务，而是使用DNS
CNAME机制把svc指向另外一个域名，这个域名可以是任何能够访问的地址，
比如`mysql.db.svc`这样的建立在db命名空间内的mysql服务，也可以指定`www.baidu.com`这样的外部真实域名。

比如可以定义一个service指向 `www.baidu.com`，然后可以在集群内的任何一个pod上访问这个service的域名，
请求service域名将自动重定向到`www.baidu.com`。

> 注意 ExternalName 这个类型也仅在集群内（不含节点）可访问。

操作步骤：

1. 定义 [service-externalname.yaml](service-externalname.yaml)，并应用
2. 进入上一节中准备好的curl容器，使用curl和nslookup命令进行访问测试；

具体指令如下：

```shell
kk apply -f service-externalname.yaml

# 进入curl容器
$ kk exec -it curl -- /bin/sh          
/ # ping service-hellok8s-externalname.default.svc.cluster.local
PING service-hellok8s-externalname.default.svc.cluster.local (14.119.104.254): 56 data bytes
64 bytes from 14.119.104.254: seq=0 ttl=54 time=9.353 ms
64 bytes from 14.119.104.254: seq=1 ttl=54 time=9.278 ms
^C
--- service-hellok8s-externalname.default.svc.cluster.local ping statistics ---
2 packets transmitted, 2 packets received, 0% packet loss
round-trip min/avg/max = 9.278/9.315/9.353 ms
/ # nslookup service-hellok8s-externalname.default.svc.cluster.local
nslookup: can't resolve '(null)': Name does not resolve

Name:      service-hellok8s-externalname.default.svc.cluster.local
Address 1: 14.119.104.189
Address 2: 14.119.104.254
Address 3: 240e:ff:e020:37::ff:b08c:124f
Address 4: 240e:ff:e020:38::ff:b06d:569b
```

注意：这里无法通过curl测试达到访问百度的效果，笔者推断是因为curl在使用service域名访问时只能拿到`www.baidu.com`
，拿不到百度服务器IP，即curl不具备DNS解析功能，所以无法正常访问百度获取到HTML。而ping工具可以执行DNS解析，所以能够拿到IP。

**用途说明**：ExternalName 类Service一般用在集群内部需要调用外部服务的时候，比如云服务商部署的DB等服务。

**无头Service + Endpoints**  
另外，很多时候，比如是自己部署的DB服务，只有IP而没有域名，ExternalName
是无法实现这个需求的，需要使用 `无头Service`+`Endpoints`来实现，
这里提供一个测试通过的模板 [service-headless-endpoints.yaml](service-headless-endpoints.yaml) 供读者自行练习。

> Endpoints对象一般不需要手动创建，Service controller会在service创建时自动创建，只有在需要关联集群外的服务时可能用到。
>
这个时候就可定义Endpoints模板，其中填入外部服务的IP和端口，然后应用即可。如果集群外的服务提供的地址是域名而不是IP，则使用`ExternalName`。

### 7.7 搭配externalIP

前面小节介绍的 ClusterIP（含Headless）/NodePort/LoadBalancer/ExternalName 五种Service都可以搭配 externalIP 使用，
externalIP是Service模板中的一个配置字段，位置是`spec.externalIP`。配置此字段后，在原模板提供的功能基础上，
还可以将Service注册到指定的externalIP（通常是节点网段内的空闲IP）上，从而增加Service的一种暴露方式。

这里提供一个测试通过的模板 [service-clusterip-externalIP.yaml](service-clusterip-externalIP.yaml) 给读者自行练习。

`spec.externalIP`可以配置为任意局域网IP（你需要的），而不必是节点网段ip，Service Controller会自动为每个节点添加路由。

### 7.8 服务发现

k8s支持下面两种服务发现方式：

- kube-dns（推荐）
- 环境变量

#### 7.8.1 kube-dns

如果你足够细心，你可能已经发现了`kube-system`空间下有个名为`kube-dns`的service，这个service就是k8s内置的DNS组件，
它用来为集群中所有Pod提供服务发现功能。这个service通过selector`k8s-app=kube-dns`关联了名为`coredns`的Pod组。

```shell
$ kk get pod,deployment,svc  -nkube-system |grep dns
pod/coredns-c676cc86f-4vzdl                    1/1     Running   1 (2d14h ago)       2d17h
pod/coredns-c676cc86f-v8s8k                    1/1     Running   1 (2d14h ago)       2d17h
deployment.apps/coredns                   2/2     2            2           2d18h
service/kube-dns   ClusterIP   20.1.0.10    <none>        53/UDP,53/TCP,9153/TCP   2d18h
```

k8s通过每个节点部署的kubelet组件向每个新启动的Pod注入DNS配置（通过`/etc/resolv.conf`），从而实现服务发现。这里随意选择一个Pod，
查看DNS配置。

```shell
$ kk exec -it hellok8s-go-http-6bb87f8cb5-c6bvs --  cat /etc/resolv.conf 
search default.svc.cluster.local svc.cluster.local cluster.local
nameserver 20.1.0.10
options ndots:5
```

详细解释这个配置：

- `search default.svc.cluster.local svc.cluster.local cluster.local`   
  这一行定义了DNS搜索域。它告诉DNS解析器，如果在域名中没有明确指定的主机名，那么应该依次尝试附加这些搜索域来查找主机名。
  在这种情况下，如果你尝试解析一个名为example的主机名，DNS解析器会首先尝试example.default.svc.cluster.local，
  然后是example.svc.cluster.local，最后是example.cluster.local。这对于Kubernetes集群中的服务发现非常有用，
  因为它允许你使用短名称来引用服务，而不必指定完整的域名。
- `nameserver 20.1.0.10`  
  这一行指定了要使用的DNS服务器的IP地址（对应`kube-dns`的ClusterIP）。在这种情况下，DNS解析器将查询由IP地址`20.1.0.10`
  指定的DNS服务器（即`pod/coredns`）来解析域名。
- `options ndots:5`  
  这一行定义了DNS解析选项。ndots是一个数字，表示DNS解析器应该在域名中查找多少次点（.）以确定绝对域名。在这种情况下，
  ndots:5表示如果一个域名中包含至少5个点，则DNS解析器会将它视为绝对域名，否则会依次附加搜索域来查找主机名。

比如现在有如下部署：

```shell
$ kk get pod,svc                                                       
NAME                                    READY   STATUS    RESTARTS   AGE
pod/hellok8s-go-http-6bb87f8cb5-c6bvs   1/1     Running   0          3h7m
pod/hellok8s-go-http-6bb87f8cb5-g8fmd   1/1     Running   0          3h7m

NAME                                 TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
service/kubernetes                   ClusterIP   20.1.0.1       <none>        443/TCP    3h36m
service/service-hellok8s-clusterip   ClusterIP   20.1.151.162   <none>        3000/TCP   3h33m
```

那么`service-hellok8s-clusterip`就是一个集群内有效的虚拟主机名（指向两个`hellok8s-go-http`Pod），我们可以启动一个`curl`
容器来测试：

```shell
$ kk apply -f pod_curl.yaml                           
pod/curl created
$ kk exec -it curl --  curl service-hellok8s-clusterip:3000      
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-g8fmd
```

有了内置的服务发现功能，我们在部署微服务项目时就无需再单独部署如consul这样的服务发现组件了，节省了不少的开发及维护工作。

#### 7.8.2 环境变量

在每个新启动的Pod中，kubelet也会向其注入当前namespace中已存在的Service信息（以环境变量形式），Pod可以通过这些环境变量来发现其他Service的IP地址。
这里假设已经启动了`service/service-hellok8s-clusterip`，然后重新启动`pod/curl`
，然后在后者shell中查看`service/service-hellok8s-clusterip`的环境变量：

```shell
$ kk exec -it curl --  printenv |grep HELLOK8S
SERVICE_HELLOK8S_CLUSTERIP_PORT_3000_TCP_PORT=3000
SERVICE_HELLOK8S_CLUSTERIP_PORT_3000_TCP=tcp://20.1.151.162:3000
SERVICE_HELLOK8S_CLUSTERIP_PORT_3000_TCP_ADDR=20.1.151.162
SERVICE_HELLOK8S_CLUSTERIP_SERVICE_HOST=20.1.151.162
SERVICE_HELLOK8S_CLUSTERIP_PORT=tcp://20.1.151.162:3000
SERVICE_HELLOK8S_CLUSTERIP_SERVICE_PORT=3000
SERVICE_HELLOK8S_CLUSTERIP_PORT_3000_TCP_PROTO=tcp
```

所以此时我们也可以通过env的方式访问`service/service-hellok8s-clusterip`：

```shell
$ kk exec -it curl --  sh                                                                                    
/ # curl $SERVICE_HELLOK8S_CLUSTERIP_SERVICE_HOST:$SERVICE_HELLOK8S_CLUSTERIP_SERVICE_PORT
[v3] Hello, Kubernetes!, From host: hellok8s-go-http-6bb87f8cb5-g8fmd
```

但是，环境变量方式对资源的创建顺序有要求。比如`pod/curl`先启动，某个service后创建，那么启动后的`pod/curl`
中就不会有这个Service相关的环境变量。
所以这里不推荐使用环境变量的方式访问Service，而是推荐使用内置DNS的方式。

## 8. 使用Ingress

上节中的Service可以通过 NodePort 或者 LoadBalancer 或者 配置externalIP 或 Pod中配置HostPort 的方式对外暴露服务，
这些方式都有一个严重问题，那就是需要占用节点端口。当需要暴露的服务逐渐增加，节点端口的占用会越来越多，且增加很大管理成本。
除此之外，这些方式也都不支持域名以及SSL配置，还需要额外配置Nginx等反向代理组件。

Ingress就是为了解决这个问题而设计的，它允许你将 Service 映射到集群对外提供的某个端点上（无需占用节点端口），从而实现对外部提供服务的功能。

举个例子：集群对外的统一端点是`api.example.com:80`，可以这样为集群内的两个Service（backend:8080、frontend:8082）配置映射：

- api.example.com/backend 指向 backend:8080
- api.example.com/frontend 指向 frontend:8082

Ingress可以为多个主机名配置不同的路由规则，提供与Nginx功能相似的服务。

总的来说，Ingress提供以下功能：

- **路由规则**：Ingress 允许你定义路由规则，使请求根据主机名和路径匹配路由到不同的后端服务。这使得可以在同一 IP
  地址和端口上公开多个服务。
- **Rewrite 规则**：Ingress 支持 URL 重写，允许你在路由过程中修改请求的 URL 路径；
- **TLS/SSL 支持**：你可以为 Ingress 配置 TLS 证书，以加密传输到后端服务的流量；
- **负载均衡**：Ingress 可以与云提供商的负载均衡器集成，以提供外部负载均衡和高可用性；
- **虚拟主机**：你可以配置多个主机名（虚拟主机）来公开不同的服务。这意味着你可以在同一 IP 地址上托管多个域名；
- **自定义错误页面**：你可以定义自定义错误页面，以提供用户友好的错误信息；
- **插件和控制器**：社区提供了多个 Ingress 控制器，如 Nginx Ingress Controller 和 Traefik，它们为 Ingress 提供了更多功能和灵活性。

Ingress 可以简单理解为集群服务的网关（Gateway），它是所有流量的入口，经过配置的路由规则，将流量重定向到后端的服务。

### 8.1 Ingress控制器

使用Ingress时一般涉及2个组件：

- **Ingress**：是 Kubernetes 中的一种 API 资源类型，它定义了从集群外部访问集群内服务的规则。通常，这些规则涉及到 HTTP 和
  HTTPS 流量的路由和负载均衡。
  Ingress 对象本身只是一种规则定义，它需要一个 Ingress 控制器来实际执行这些规则。
- **Ingress 控制器**：是 Kubernetes 集群中的一个独立组件或服务，它实际处理 Ingress 规则，根据这些规则配置集群中的代理服务器（如
  Nginx、HAProxy、Traefik 等）来处理流量路由和负载均衡。
  Ingress 控制器负责监视 Ingress 对象的变化，然后动态更新代理服务器的配置以反映这些变化。Kubernetes社区提供了一些不同的
  Ingress 控制器，您可以根据需求选择合适的控制器。

Ingress控制器不会随集群一起安装，需要单独安装。可以选择的Ingress控制器很多，这里是
[社区提供的Ingress控制器列表](https://kubernetes.io/zh-cn/docs/concepts/services-networking/ingress-controllers/)，
可根据情况自行选择，常用的是Nginx、Traefik。

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

# 等待各项资源就绪
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s
  
#查看安装的各种资源
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

可能会遇到image拉取失败，解决如下：

```
$ kk get pod -ningress-nginx                                           
NAME                                        READY   STATUS              RESTARTS   AGE
ingress-nginx-admission-create-csfjc        0/1     ImagePullBackOff    0          5m55s
ingress-nginx-admission-patch-rgdxr         0/1     ImagePullBackOff    0          5m55s
ingress-nginx-controller-6f4df7b5d6-dhfg2   0/1     ContainerCreating   0          5m55s

$ kk describe pod ingress-nginx-admission-create-csfjc -ningress-nginx
...
Events:
Type     Reason     Age                   From               Message
  ----     ------     ----                  ----               -------
Normal   Scheduled  3m6s                  default-scheduler  Successfully assigned ingress-nginx/ingress-nginx-admission-create-csfjc to k8s-node1
Normal   BackOff    2m19s                 kubelet            Back-off pulling image "registry.k8s.io/ingress-nginx/kube-webhook-certgen:v20230407@sha256:543c40fd093964bc9ab509d3e791f9989963021f1e9e4c9c7b6700b02bfb227b"
Warning  Failed     2m19s                 kubelet            Error: ImagePullBackOff
Normal   Pulling    2m5s (x2 over 3m20s)  kubelet            Pulling image "registry.k8s.io/ingress-nginx/kube-webhook-certgen:v20230407@sha256:543c40fd093964bc9ab509d3e791f9989963021f1e9e4c9c7b6700b02bfb227b"
Warning  Failed     15s (x2 over 2m19s)   kubelet            Failed to pull image "registry.k8s.io/ingress-nginx/kube-webhook-certgen:v20230407@sha256:543c40fd093964bc9ab509d3e791f9989963021f1e9e4c9c7b6700b02bfb227b": rpc error: code = DeadlineExceeded desc = failed to pull and unpack image "registry.k8s.io/ingress-nginx/kube-webhook-certgen@sha256:543c40fd093964bc9ab509d3e791f9989963021f1e9e4c9c7b6700b02bfb227b": failed to resolve reference "registry.k8s.io/ingress-nginx/kube-webhook-certgen@sha256:543c40fd093964bc9ab509d3e791f9989963021f1e9e4c9c7b6700b02bfb227b": failed to do request: Head "https://us-west2-docker.pkg.dev/v2/k8s-artifacts-prod/images/ingress-nginx/kube-webhook-certgen/manifests/sha256:543c40fd093964bc9ab509d3e791f9989963021f1e9e4c9c7b6700b02bfb227b": dial tcp 142.251.8.82:443: i/o timeout


# 发现无法访问 registry.k8s.io，参考https://github.com/anjia0532/gcr.io_mirror 进行解决
# 笔者发起issue来同步nginx用到的几个镜像到作者的docker仓库，大概1min完成同步，然后现在在节点手动拉取这个可访问的docker.io下的镜像进行替代
# 在非master节点执行（ctr是containerd cli）：
ctr image pull docker.io/anjia0532/google-containers.ingress-nginx.kube-webhook-certgen:v20230407
ctr image pull docker.io/anjia0532/google-containers.ingress-nginx.controller:v1.8.2

# 替换模板中的镜像
sed -i 's#registry.k8s.io/ingress-nginx/kube-webhook-certgen:v20230407@sha256:543c40fd093964bc9ab509d3e791f9989963021f1e9e4c9c7b6700b02bfb227b#docker.io/anjia0532/google-containers.ingress-nginx.kube-webhook-certgen:v20230407#' deploy.yaml
sed -i 's#registry.k8s.io/ingress-nginx/controller:v1.8.2@sha256:74834d3d25b336b62cabeb8bf7f1d788706e2cf1cfd64022de4137ade8881ff2#docker.io/anjia0532/google-containers.ingress-nginx.controller:v1.8.2#' deploy.yaml
# 再次应用
kk apply -f deploy.yaml
```

这里重点关注`service/ingress-nginx-controller`这一行，这是Nginx Ingress自动创建的`LoadBalancer`类型的service，
它会跟踪Ingress配置中的后端Pod组端点变化，并实时更新Pod `ingress-nginx-controller`中的转发规则，
后者再转发流量到 `service-hellok8s-clusterip`，然后最终到达业务pod。

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
4. 部署 [deployment_httpd_svc.yaml](deployment_httpd_svc.yaml) 作为 Ingress 后端之一
5. 定义 Ingress [ingress-hellok8s.yaml](ingress-hellok8s.yaml)，其中定义了路由规则，然后应用
6. 在节点上验证

```shell
# 查看部署的资源（省略了不相关的资源）
$ kk get pods,svc,ingress          
NAME                                    READY   STATUS    RESTARTS       AGE
pod/hellok8s-go-http-6bb87f8cb5-57r86   1/1     Running   1 (12h ago)    37h
pod/hellok8s-go-http-6bb87f8cb5-lgtgf   1/1     Running   1 (12h ago)    37h
pod/httpd-69fb5746b6-5v559              1/1     Running   0              97s

NAME                                            TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)    AGE
service/httpd-svc                               ClusterIP      20.1.140.111   <none>          8080/TCP   97s
service/service-hellok8s-clusterip              ClusterIP      20.1.112.41    <none>          3000/TCP   28h

NAME                                         CLASS   HOSTS   ADDRESS   PORTS   AGE
ingress.networking.k8s.io/hellok8s-ingress   nginx   *                 80      9m18s

# 1-通过clusterIP访问httpd
$ curl 20.1.140.111:8080
<html><body><h1>It works!</h1></body></html>

# 前一节讲到的nginx 以 LoadBalancer部署的svc，所以要通过节点访问，需要先获知svc映射到节点的端口号，如下为 80:31504, 443:32548
$ kk get svc -ningress-nginx
NAME                                 TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)                      AGE
ingress-nginx-controller             LoadBalancer   20.1.251.172   <pending>     80:31504/TCP,443:32548/TCP   19h
ingress-nginx-controller-admission   ClusterIP      20.1.223.76    <none>        443/TCP                      19h

# 2-通过节点映射端口访问 /httpd
$ curl 127.0.0.1:31504/httpd
<html><body><h1>It works!</h1></body></html>

# 3-通过节点映射端口访问 /hello
$ curl 127.0.0.1:31504/hello
[v3] Hello, Kubernetes!, this is ingress test, host:hellok8s-go-http-6df8b5c5d7-75qb6
```

这就是基本的ingress使用步骤，还可以通过`kk describe -f ingress-hellok8s.yaml`查看具体路由规则。

若要更新路由规则，修改Ingress yaml文件后再次应用即可，通过`kk logs -f ingress-nginx-controller-xxx -n ingress-nginx`
可以看到请求日志。

这里列出几个常见的配置示例，供读者自行练习：

- [虚拟域名：ingress-hellok8s-host.yaml](ingress-hellok8s-host.yaml)
- [配置证书：ingress-hellok8s-cert.yaml](ingress-hellok8s-cert.yaml)
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
   说明：如果要把Ingress部署到公有云，那用这种方式比较合适。用Deployment部署ingress-controller，创建一个type为 LoadBalancer
   的 service 关联这组 Pod。大部分公有云，都会为 LoadBalancer 的 service 自动创建一个负载均衡器，通常还绑定了公网地址。
   只要把域名解析指向该地址，就实现了集群服务的对外暴露。

2. **DaemonSet + HostNetwork + nodeSelector**  
   说明：用DaemonSet结合nodeSelector来部署ingress-controller到特定的node上，然后使用HostNetwork直接把该pod与宿主机node的网络打通，直接使用节点的80/433端口就能访问服务。这时，ingress-controller所在的node机器就很类似传统架构的边缘节点，比如机房入口的nginx服务器。该方式整个请求链路最简单，性能相对NodePort模式更好。
   有一个问题是由于直接利用宿主机节点的网络和端口，一个node只能部署一个ingress-controller
   pod，但这在生产环境下也不算是问题，只要完成多节点部署实现高可用即可。
   然后将Ingress节点公网IP填到域名CNAME记录中即可。  
   笔者提供测试通过的Ingress-nginx模板供读者练习：[ingress-nginx-daemonset-hostnetwork.yaml](ingress-nginx-daemonset-hostnetwork.yaml)
   ，主要修改了3处：
    - `Deployment` 改为 `DaemonSet`
    - 注释`DaemonSet`模块的`strategy`部分（strategy是Deployment模块下的字段）
    - 在DaemonSet模块的`spec.template.spec`下添加`hostNetwork: true`
      使用这个模板后，可以观察到在k8s-node1上会监听80、443和8443端口（Ingress-nginx需要的端口）。
3. **Deployment + `NodePort`模式的Service**  
   说明：同样用Deployment模式部署ingress-controller，并创建对应的service，但是type为NodePort。这样，Ingress就会暴露在集群节点ip的特定端口上。
   然后可以直接将Ingress节点公网IP填到域名CNAME记录中。    
   笔者提供测试通过Ingress-nginx模板供读者练习：[ingress-nginx-deployment-nodeport.yaml](ingress-nginx-deployment-nodeport.yaml)
   ，主要修改了2处：
    - `Service`模块下`spec.ports`部分新增`nodePort: 30080`和`nodePort: 30443`（注意nodePort设置的端口受到范围限制：30000-32767）

   这种方式不适用于对外暴露80/443端口的应用。

练习时，如对Ingress-nginx模板有修改，建议完全删除该模板对应资源（使用`kk delete -f deploy.yaml`，操作可能会耗时几十秒），否则直接应用可能不会生效。

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

## 10. 使用ConfigMap和Secret

ConfigMap 和 Secret 用来保存配置数据和敏感数据，在模板定义和使用上没有大太差别。

### 10.1  ConfigMap

K8s 使用 ConfigMap 来将你的配置数据和应用程序代码分开，将非机密性的数据保存到键值对中。ConfigMap 在设计上不是用来保存大量数据的。
在 ConfigMap 中保存的数据不可超过 1 MiB。如果你需要保存超出此尺寸限制的数据，你需要考虑挂载存储卷。

ConfigMap 可以用四种方式进行使用：

- 在容器命令和参数内
- 容器的环境变量（常见）
- 在只读卷里面添加一个文件，让应用来读取（常见）
- 编写代码在 Pod 中运行，使用 Kubernetes API 来读取 ConfigMap（不常见）

下面使用ConfigMap来保存app`hellok8s`的配置信息：

```yaml
# configmap-hellok8s.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: hellok8s-configmap
data: # 用来保存UTF8字符串
  DB_URL: "http://mydb.example123.com"
binaryData: # 用来保存二进制数据作为 base64 编码的字串。
  app-config.json: eyJkYl91cmwiOiJteXNxbC5leGFtcGxlLmNvbSJ9Cg==  # echo '{"db_url":"mysql.example.com"}' |base64

# 对于一个大量使用 configmap 的集群，禁用 configmap 修改会带来以下好处
# 1. 保护应用，使之免受意外（不想要的）更新所带来的负面影响。
# 2. 通过大幅降低对 kube-apiserver 的压力提升集群性能， 这是因为系统会关闭对已标记为不可变更的 configmap 的监视操作。
# 一旦标记为不可更改，这个操作就不可逆，再想要修改就只能删除并重建 configmap
immutable: true
```

然后修改 deployment 以读取configmap，具体看 [deployment-use-configmap.yaml](deployment-use-configmap.yaml)。

然后再修改main.go为 [main_read_configmap.go](main_read_configmap.go)，接着重新构建并推送镜像：

```shell
# 先删除所有资源
$ kubectl delete pod,deployment,service,ingress --all

docker build . -t leigg/hellok8s:v4_configmap
docker push leigg/hellok8s:v4_configmap

kk apply -f deployment-use-configmap.yaml

$ kk get pod                               
NAME                                READY   STATUS    RESTARTS   AGE
hellok8s-go-http-684ff55564-qf2x9   1/1     Running   0          3m47s
hellok8s-go-http-684ff55564-s5bfl   1/1     Running   0          3m47s

# pod直接映射到节点端口
$ curl 10.0.2.3:3000/hello
[v4] Hello, Kubernetes! From host: hellok8s-go-http-c548c88b5-25sl9
Get Database Connect URL: http://mydb.example123.com
app-config.json:{"db_url":"mysql.example.com"}
```

可以看到app已经拿到了configmap中定义的配置信息。若要更新，直接更改configmap的yaml文件然后应用，然后重启业务pod即可（使用`kubectl rollout restart deployment <deployment-name>`）。

若configmap配置了`immutable: true`，则无法再进行修改：

```shell
$ kk apply -f configmap-hellok8s.yaml      
The ConfigMap "hellok8s-configmap" is invalid: data: Forbidden: field is immutable when `immutable` is set
```

最后需要注意的是，在上面提到的四种使用configmap的方式中，只有挂载volume和调用k8s
API的方式会收到configmap的更新影响，其余两种则需要重启Pod才能看到更新。

### 10.2 Secret

Secret用于存储敏感信息，例如密码、Token、（证书）密钥等，在使用上与ConfigMap不会有太大差别，但需要注意下面两点。
- `data`的value部分必须是base64编码后的字符串（**创建时会执行base64解码检查**），但Pod中获取到的仍然是明文；
- 模板语法上稍有不同
  - Secret 支持的是`stringData`而不是`binaryData`，它的value可以是任何UTF8字符
  - 额外支持`type`字段，用来在创建时检查资源合法性

#### 10.2.1 Secret的类型

创建 Secret 时，还可以使用 Secret 资源的 type 字段（可选），它用来告诉k8s我要创建何种类型的secret，并根据类型对其进行基础的合法性检查。
当前支持的类型如下：

| 类型                                  | 描述                             |
|-------------------------------------|--------------------------------|
| Opaque                              | 用户定义的任意数据（默认）                  |
| kubernetes.io/service-account-token | 服务账号令牌                         |
| kubernetes.io/dockercfg             | ~/.dockercfg 文件的序列化形式          |
| kubernetes.io/dockerconfigjson      | ~/.docker/config.json 文件的序列化形式 |
| kubernetes.io/basic-auth            | 用于基本身份认证的凭据                    |
| kubernetes.io/ssh-auth              | 用于 SSH 身份认证的凭据                 |
| kubernetes.io/tls                   | 用于 TLS 客户端或者服务器端的数据            |
| bootstrap.kubernetes.io/token       | 启动引导令牌数据                       |

比如type为`kubernetes.io/tls`时，k8s要求secret中必须包含`tls.crt`和`tls.key`两个字段（data或stringData都可），
但这里不会对值进行任何检查，并且这个类型也限制了创建后再修改，只能删除重建。
[secret-hellok8s-cert.yaml](secret-hellok8s-cert.yaml) 是一个合法的`kubernetes.io/tls`类型的Secret。其他类型的要求查看[官方文档](https://kubernetes.io/zh-cn/docs/concepts/configuration/secret/#secret-types)。

#### 10.2.2 引用时设置可选key

正常情况下，如果引用Secret的某个字段不存在，则启动Pod时会报错，比如：
```yaml
...
- name: LOG_LEVEL
  valueFrom:
    secretKeyRef:
      name: hellok8s-secret # name必须是有效且存在的
      key: not_found_key
#      optional: true
```
k8s允许在其他位置引用Secret时添加`optional: true`属性，以允许Secret中的字段不存在时不会影响Pod启动（但Secret本身必须存在）。

#### 10.2.3 拉取私有镜像使用Secret
如果你尝试从私有仓库拉取容器镜像，你需要一种方式让每个节点上的 kubelet 能够完成与镜像库的身份认证。
你可以配置 `imagePullSecrets`字段来实现这点。 Secret 是在 Pod 层面来配置的。

有两种方式来配置 `imagePullSecrets`：

1. [直接在 Pod 上指定 `ImagePullSecrets`](https://kubernetes.io/zh-cn/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod)；
2. [向 ServiceAccount 添加 `ImagePullSecrets`](https://kubernetes.io/zh-cn/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account)；

第一种方式需要对使用私有仓库的每个 Pod 执行以上操作，如果App较多，可能比较麻烦。
第二种方式是推荐的方式，因为 ServiceAccount 是一个集群范围内的概念，所以只需要在 ServiceAccount 上添加 Secret，
所有引用该 ServiceAccount 的 Pod 都会自动使用该 Secret。

完整演示这一步需要私有镜像仓库，此节测试略过。

#### 10.2.4 在Pod内测试
笔者提供测试通过的模板和代码供读者自行测试：
- [secret-hellok8s-misc.yaml](secret-hellok8s-misc.yaml)
- [deployment-use-secret.yaml](deployment-use-secret.yaml)
- [main_read_secret.go](main_read_secret.go)

测试结果：
```shell
$ curl 10.0.2.3:3000/hello
[v4] Hello, Kubernetes! From host: hellok8s-go-http-869d4548dc-vtmcr, Get Database Passwd: pass123

some.txt:hello world
cert.key:-----BEGIN OPENSSH PRIVATE KEY-----
J1a9V50zOAl0k2Fpmy+RDvCy/2LeCZHyWY9MR248Ah2Ko3VywDrevdPIz8bxg9zxqy0+xy
jbu09sNix9b0IZuZQbbGkw4C4RcAN5HZ4UnWWRfzv2KgtXSdJCPp38hsWH2j9hmlNXLZz0
EqqtXGJpxjV67NAAAACkxlaWdnQEx1eWk=
-----END OPENSSH PRIVATE KEY-----
config.yaml:username: hellok8s
password: pass123
```
## TODO

## 参考

- [k8s教程](https://github.com/guangzhengli/k8s-tutorials/blob/main/docs/pre.md)
- [Kubernetes从入门到实践 @赵卓](https://www.epubit.com/bookDetails?id=UB72096269c1157)
- [Docker教程](https://yeasy.gitbook.io/docker_practice/)
- [kubectl全部命令-官方](https://kubernetes.io/docs/reference/kubectl/cheatsheet/)
- [K8s对外服务之Ingress](http://www.uml.org.cn/yunjisuan/202303134.asp?artid=25653)
