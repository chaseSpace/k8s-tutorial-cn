# K8s实战（更新中）

本文以一个简单Go应用为例，演示如何一步步在生产环境中使用Kubernetes。

注意文中命令行使用的`kk`是kubectl的别名。

## 1. 部署一个完整的应用

### 1.1 编写一个简单的Go应用

- [main_multiroute.go](k8s_actions_guide/version1/main_multiroute.go)

这个Go应用的逻辑很简单，它是一个支持从配置动态加载路由的HTTP服务器。初始化此应用：

```shell
go mod init k8s_action
go mod tidy
```

### 1.2 使用ConfigMap存储配置

传统方式下，我们通常将配置文件存储在单独文件中，并通过环境变量或者命令行参数传递给应用。

在K8s环境中，我们需要将配置迁移到ConfigMap中，并通过环境变量或者卷挂载的方式传递给应用。

- [k8s-manifest/configmap.yaml](k8s_actions_guide/version1/k8s-manifest/configmap.yaml)

注意将K8s清单放在一个单独的目录（如`k8s-manifest`）下，以便后续批量部署。

> 虽然可以在Dockerfile中直接将配置文件打包到容器，但这种方式通常伴随的是将配置文件存储在代码库中，这并不符合K8s的最佳实践。
> 同时也不适合用来存储重要配置。

如果有重要的配置，比如证书私钥或Token之类的敏感信息，请使用Secret来存储。

### 1.3 使用Secret存储敏感信息

通常一个后端应用会链接到数据库来对外提供API服务，所以我们需要为应用提供数据库密码。

虽然ConfigMap也可以存储数据，但Secret更适合存储敏感信息。在K8s中，Secret用来存储敏感信息，比如密码、Token等。

- [k8s-manifest/secret.yaml](k8s_actions_guide/version1/k8s-manifest/secret.yaml)

#### 1.3.1 加密存储Secret中的数据

虽然Secret声称用来存储敏感信息，但默认情况下它是非加密地存储在集群存储（etcd）上的。
任何拥有 API 访问权限的人都可以检索或修改 Secret。

请参考以下链接来加密存储Secret中的数据：

- [K8s Secrets](https://kubernetes.io/zh-cn/docs/concepts/configuration/secret/)
- [加密 K8s Secrets 的几种方案](https://www.cnblogs.com/east4ming/p/17712715.html)

### 1.4 使用Dockerfile打包应用

这一步中，我们编写一个Dockerfile文件将Go应用打包到一个镜像中，以便后续部署为容器。

- [Dockerfile](k8s_actions_guide/version1/Dockerfile)

注意在Dockerfile中定制你的Go版本。

### 1.5 准备镜像仓库

为了方便后续部署，我们需要将打包好的镜像上传到镜像仓库中。

作为演示，本文使用Docker Hub作为镜像仓库。但在生产环境中，为了应用安全以及提高镜像拉取速度，我们应该使用（搭建）私有的仓库。

常用的开源镜像仓库有Docker Registry和Harbor。如果你使用云厂商的托管集群，可以使用它们提供的镜像仓库产品。

### 1.6 编写Deployment模板

Deployment是K8s中最常用的用来部署和管理应用的资源对象。它支持应用的多副本部署以及故障自愈能力。

- [k8s-manifest/deployment.yaml](k8s_actions_guide/version1/k8s-manifest/deployment.yaml)

你可以在模板中定制应用的副本数量、资源限制、环境变量等配置。

> 注意：你可能看情况需要修改模板中的namespace，生产环境不建议使用default命名空间。因为这不利于对不同类型的应用进行隔离和资源限制。
> 比如，你可以为后端服务和前端服务分别使用backend和frontend命名空间。

**为镜像指定指纹**  
docker拉取镜像时支持使用如下命令：

```shell
docker pull busybox:1.36.1@sha256:7108255e7587de598006abe3718f950f2dca232f549e9597705d26b89b7e7199
# docker images --digests 获取镜像hash
```

后面的`sha256:710...`是镜像的唯一hash。当有人再次推送相同tag的镜像覆盖了旧镜像时，拉取校验就会失败，这样可以避免版本管理混乱导致的部署事故。

所以我们可以在Deployment模板中指定镜像的tag的同时使用`@sha256:...`来指定镜像的hash以提高部署安全性。

### 1.7 使用CI/CD流水线

按前述步骤完成后，应该得到以下文件布局：

```
├── Dockerfile
├── go.mod
├── go.sum
├── k8s-manifest
│   ├── configmap.yaml
│   └── deployment.yaml
└── main_multiroute.go

```

现在可以将它们提交到代码仓库中。然后使用CI/CD流水线来构建镜像并部署应用。

> 代码库中通常会存储***非生产环境**的配置文件，对于生产环境使用的配置文件（ConfigMap和Secret），不应放在代码库中，
> 而是以手动方式提前部署到环境中。

参考下面的指令来配置CI/CD流水线：

```shell
# 假设现在已经进入到构建机（需要连接到k8s集群）

IMAGE=leigg/go_multiroute
TAG=v1 # 每次迭代时手动指定
_IMAGE_=$IMAGE:$TAG

# 构建镜像（将leigg替换为你的镜像仓库地址）
$ docker build . -t $_IMAGE_
...
Successfully built 20e2a541e835
Successfully tagged leigg/go_multiroute:v1

# 推送镜像到仓库
$ docker push $_IMAGE_
The push refers to repository [docker.io/leigg/go_multiroute]
f658e2d998f1: Pushed
06d92acd05c8: Pushed
3ce819cc4970: Mounted from library/alpine
v1: digest: sha256:74bf6d94ea9af3e700dfd9fe64e1cc6a04cd75fb792d994c63bbc6d69de9b7ee size: 950

# 部署应用
$ kk apply -f ./k8s-manifest
configmap/go-multiroute created
deployment.apps/go-multiroute unchanged

# 迭代应用
$ kk set image deployment/go-multiroute go-multiroute=$_IMAGE_
```

查看应用部署情况：

```shell
$ kk get deploy
NAME            READY   UP-TO-DATE   AVAILABLE   AGE
go-multiroute   2/2     2            2           7s
$ kk get po    
NAME                            READY   STATUS    RESTARTS   AGE
go-multiroute-f4f8b64f4-564qq   1/1     Running   0          8s
go-multiroute-f4f8b64f4-v64l6   1/1     Running   0          8s
```

### 1.8 为服务配置外部访问

现在已经在集群内部部署好了应用，但是还无法从集群外部访问。我们需要再部署以下资源来提供外部访问能力。

- Service：为服务访问提供流量的负载均衡能力（支持TCP/UDP/SCTP协议）
- Ingress：管理集群外部访问应用的路由端点，支持HTTP/HTTPS协议
    - Ingress需要安装某一种Ingress控制器才能正常工作，常用的有Nginx、Traefik。

> 如果应用是非HTTP服务器（如仅TCP、Websocket服务），则无需Ingress，仅用Service来暴露服务就可。

- [k8s-manifest/service.yaml](k8s_actions_guide/version1/k8s-manifest/service.yaml)
- [k8s-manifest/ingress.yaml](k8s_actions_guide/version1/k8s-manifest/ingress.yaml)

部署Ingress控制器的步骤这里不再赘述，请参考[基础教程](doc_tutorial.md#82-安装Nginx-Ingress控制器)。

下面是部署Service和Ingress的步骤：

```shell
$ kk apply -f ./k8s-manifest
configmap/go-multiroute unchanged
deployment.apps/go-multiroute unchanged
ingress.networking.k8s.io/go-multiroute created
service/go-multiroute created

# 注意：service/kubernetes是默认创建的，不用理会
$ kk get svc,ingress                           
NAME                    TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
service/go-multiroute   ClusterIP   10.96.219.33   <none>        3000/TCP   19s
service/kubernetes      ClusterIP   10.96.0.1      <none>        443/TCP    6d

NAME                                      CLASS   HOSTS   ADDRESS   PORTS   AGE
ingress.networking.k8s.io/go-multiroute   nginx   *                 80      19s
```

现在，可以通过Ingress控制器开放的端口来访问应用了。笔者环境安装的Nginx Ingress控制器，查看其开放的端口：

```shell
# PORT(S) 部分是Nginx Ingress控制器内对外的端口映射
# 内部80端口映射到外部 30073，内部443端口映射到外部30220
kk get svc -ningress-nginx
NAME                                 TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
ingress-nginx-controller             LoadBalancer   10.96.171.227   <pending>     80:30073/TCP,443:30220/TCP   3m46s
ingress-nginx-controller-admission   ClusterIP      10.96.7.58      <none>        443/TCP                      3m46s
```

使用控制器的端口访问服务：

```
# 在任意节点上进行访问
$ curl 127.0.0.1:30073/route1
Hello, You are at /route1, Got: route1's content
 
# /route2 尚未在ingress的规则中定义，所以不能通过ingress访问
$ curl 127.0.0.1:30073/route2
<html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr><center>nginx</center>
</body>
</html>
```

OK，现在访问没有问题了。

#### 1.8.1 为什么通过30073端口访问

Nginx Ingress控制器默认通过NodePort方式部署，所以会在宿主机上开放两个端口（本例中是30073和30220），
这两个端口会分别代理到Ingress控制器内部的80和443端口。

本例中部署的Go应用是一个后端服务，对于向外暴露的端口号没有要求。
但如果是一个前端应用，比如是一个Web网站，那么可能就有对外暴露80/443端口的要求。
此时就需要调整Ingress控制器部署方式，使用LoadBalancer或`HostNetwork`方式部署。

### 1.9 更新配置的最佳实践

应用上线后，我们可能会有更改应用配置的需求。一般的做法是直接更新现有的ConfigMap，然后重启所有Pod。
但这不是K8s的最佳实践。原因有以下几个：

- 更新ConfigMap不会触发Deployment中所有Pod重启
- 若更新后的配置有问题不能迅速回滚（需要再次编辑现有ConfigMap），导致服务短暂宕机

而最佳实践是为ConfigMap命名带上版本号如`app-config-v1`，然后部署新版本的ConfigMap，
再修改Deployment模板中引用的ConfigMap名称，最后更新Deployment（触发所有Pod滚动更新）。

当需要回滚时，再次更新Deployment即可。

## 2. 开发者工作流

为了提高开发人员的工作效率和最大化在日常工作中模拟在生产环境中开发，我们通常会搭建一个开发集群来提供给开发者们完成日常开发。
但随之而来的就是集群的用户管理、资源额度限制以及回收工作，我们可以开发脚本来自动化完成这些工作。

### 2.1 搭建不同环境的集群

**开发集群**  
我们需要一个共享K8s集群来提供给开发者们使用，他们的日常开发流程包括PR提交、代码review、构建、部署等工作都将和生产环境保持基本一致。

但我们必须为每个开发者分配单独的K8s命名空间，然后设置资源额度限制，防止开发者们无节制地使用集群资源以造成互相干扰。
当团队规模较大时，建议为10~20人共享一个集群，而不是上百人共享一个超大集群，这样可以简化集群的管理工作。

**测试环境**  
当开发者在开发环境完整测试过自己提交的代码后，就可以考虑部署到测试环境，并告知测试人员进行测试。
测试环境通常只有一个K8s集群由所有人共享。

**预发布环境**  
大部分公司都会有一个预发布环境，用来部署一些需要经过测试环境验证的应用版本，比如一些新功能或一些需要修复的bug。
这个环境应该与生产环境尽可能保持高度一致，包括节点数量、网络等其他配置，且相对测试环境具有更严格的权限控制。
不允许开发者随意更改数据库等配置，测试人员在预发布环境应按照生产环境的标准进行测试。

**镜像仓库**  
镜像仓库是用来存放应用镜像的地方，通常由运维人员来维护。每个环境都应该有一个本地的镜像仓库，以加快应用部署。

### 2.2 管理员使用的脚本

为了简化集群的用户管理、资源额度限制以及回收工作，我们可以开发一些脚本来协助管理员轻松完成这些工作。

- [new_user.sh](k8s_actions_guide/version1/k8s-script/new_user.sh)：在集群中添加一个新用户，并创建相应的命名空间、角色和额度限制。
    - 此脚本会同时生成`client-cert-$USER.crt`和`client-cert-$USER.key`
- [del_user.sh](k8s_actions_guide/version1/k8s-script/del_user.sh)：删除集群中的一个用户命名空间（包含空间下的所有资源）。
- [setup_kubeconfig.sh](k8s_actions_guide/version1/k8s-script/setup_kubeconfig.sh)：初始化用户使用的kubeconfig文件。
    - 此脚本会在执行目录下生成`dev-config`文件，将此文件分发给对应开发人员作为kubeconfig文件即可。

> 笔者在脚本中添加了详实的注释，你可以阅读它们来了解脚本用法和步骤含义。

### 2.3 更方便的查询应用日志

应用上线后，我们通常会有查看应用日志的需求。
默认的应用日志分散存储在每个Pod所在节点的`/var/log/pods`目录，且它们会伴随Pod的消失而被删除。
这不利用开发者们查看日志的需求，因此我们需要一个中心化存储日志的方案。
请参考[Kubernetes 日志收集](doc_log_collection.md)来了解如何搭建日志收集系统。

如果因为某些原因不希望搭建日志收集系统（或者说是偷懒~），
你也可以通过修改每个节点上的kubelet配置来调整容器日志的存储大小和文件数量，具体步骤也请参考[Kubernetes 日志收集中的
*业务容器日志*](doc_log_collection.md#11-业务容器日志)。
其次，你还可以参考[Kubernetes 维护指导中的*日志查看*](doc_maintaintion.md#15-日志查看)来使用第三方工具来高效查询容器日志。

> 除了Pod日志，我们还需要关注集群组件日志、节点日志、集群审计日志。

### 2.4 启动开发

开发过程中，开发者需要频繁地构建并推送镜像、更新应用、查看应用日志。
这其中最为重要的步骤就是镜像tag定义以及更新应用的操作。

**镜像tag使用版本化语义而不是latest**  
在构建用于开发环境的自测试镜像时，对于镜像tag，开发者最好是使用版本化语义而不是`latest`。
使用`latest`可以免去每次构建镜像时都需要手动修改tag的麻烦，但同时也带来了不确定性。
因为你无法完全确定所更新的应用使用了你刚刚构建并推送的镜像。倘若你将误以为自测成功的代码发布到了生产环境，
难以想象将会发生的事情和面临的后果😑。

**更新应用**  
在使用版本化语义的镜像tag后，我们可以更从容的使用`kubectl set image...`命令来更新应用。
注意，在开发以及后续的上线过程中，我们都不需要修改代码库中的Deployment YAML文件。

### 2.5 使用第三方工具为开发提效

#### 2.5.1 IDE插件

编写此文时已经是2024年了，主流的VSCode和Jetbrains家族IDE都已经有大量的Kubernetes插件可用，
这些插件可以帮助开发者通过图形化的方式与开发环境的K8s集群进行便捷交互，免去手动输入kubectl命令的繁琐。

#### 2.5.2 K9s终端面板

K9s是一个终端形式的K8s资源面板，它支持在终端中以可视化方式查看和管理Kubernetes集群中的资源，包括Pod、Deployment、Service等。
它支持以方向键、回车键和空格键与面板交互，免去手敲命令的麻烦。

我们可以在测试环境和预发布环境（也包括生产环境）安装K9s，这样可以更便捷的查看应用状态、日志、事件、以及进入Pod内的容器Shell，极大地改善了K8s的使用体验。

#### 2.5.3 开源K8s日志工具

常规查看容器日志的命令是`kubectl logs`，但这个命令有一些局限性，比如：

- 一次只能查看一个Pod的日志（若不使用`-l`的话）
- 不能指定Deployment、Service、Job、Stateful和Ingress名称进行日志查看
- 不能查看指定节点上的所有Pod日志
- 不支持颜色打印

等等。你可以使用开源的K8s Pod日志查看工具来提高效率，具体可以参考 *Kubernetes 维护指导*
中的[日志查看](doc_maintaintion.md#15-日志查看)小节。

#### 2.5.4 K8s资源清单风险分析工具

或许在较小的集群中或者是不那么重要的业务中，我们并不会去特别注意K8s资源清单的编写规范，例如对容器的CPU/内存限制、设置容器安全上下文等。
但我们需要知道，Pod中的容器是本质上来说还是运行在集群节点上的，不安全的资源清单可能会导致容器在节点上被恶意利用，从而导致集群被攻击。

好在已经有不少开源工具可以协助我们轻松完成资源清单的修复工作，这里推荐以下几个工具：

- [KubeLinter](https://github.com/stackrox/kube-linter)
- [KubeSec](https://kubesec.io/)
- [kube-score](https://github.com/zegl/kube-score)
- [polaris](https://github.com/FairwindsOps/polaris)

你可以将这些工具中的一个或几个添加CI/CD流水线中，以在每次提交代码时自动检查资源清单的安全性。

## 3. 采集集群指标

### 3.1 简介

指标是指针对某一种资源在一段时间内的使用情况的统计，例如CPU使用率、内存使用量、网络带宽、磁盘使用量等。

指标采集通常有两种说法，即黑盒监控和白盒监控。黑盒监控是从集群的外部视角采集数据。多用于传统的CPU、内存、磁盘等硬件资源的监控，
非常适用于对基础设施的监控。而白盒监控更关注应用程序的状态细节，比如HTTP请求总数、500错误请求数和请求延迟等。
白盒监控让我们知道系统为什么处于当前状态，让故障定位进一步有迹可循。

### 3.2 两种监控模式

它们分别是USE和RED模式。

#### 3.2.1 USE模式

USE的释义如下：

- U——Utilization（利用率）
- S——Saturation（饱和度）
- E——Errors（错误率）

这种模式专注于基础设施监控，对应黑盒监控。

#### 3.2.2 RED模式

RED的释义如下：

- R——Rate（每秒接受的请求数）
- E——Error（每秒失败的请求数）
- D——Duration（每个请求的耗时）

这种模式专注于应用程序监控，对应白盒监控。

### 3.3 采集目标

了解了上面的监控模式后，现在我们需要知道应该在集群中进行指标采集的目标有哪些。
这里笔者将它们分类列出：

- 控制平面：API Server、etcd、Scheduler、Controller Manager
- 工作节点：kubelet、容器运行时、kube-proxy、kube-dns和Pod

上面除了工作节点的Pod以外，其他可以归类为基础设施组件。我们需要监控这些组件暴露的各项指标并及时做出响应，
才能确保集群的稳定运行。

### 3.4 采集架构

#### 3.4.1 使用Prometheus作为存储后端

Prometheus是CNCF（云原生计算基金会）中排名仅次于 Kubernetes 的一个重量级开源项目，是一个用于监控和告警的开源系统。
它提供了一个灵活的查询语言叫做**PromQL**，让我们可以方便地查询和分析监控数据。目前，Prometheus已经是业界公认的监控事实标准。

Prometheus架构图如下
![](./img/prometheus_architecture.png)

简单来说，Prometheus由以下几个部分组成：

- Prometheus Server：负责数据采集和存储，并提供PromQL查询语言的支持。
- Push Gateway：支持临时性任务的数据采集。
- Exporter：用于暴露被监控组件的数据接口。
    - 对于不同的采集目标（例如主机节点）需要部署对应的Exporter，然后配置Prometheus主动采集即可。
    - 常见的有：node_exporter、blackbox_exporter、mysqld_exporter、redis_exporter等。
- Client Library：客户端库，为需要监控的组件提供方便的接入方式。
    - 对于那些没有Exporter的采集目标（比如业务应用），我们可以通过客户端库自行上报数据到Prometheus Server中。
- Alert-manager：负责接收Prometheus的告警信息，并决定如何对告警进行处理，如发送邮件、短信、调用Webhook等。

通过上面的架构图和文字说明，我们可以了解到Prometheus支持以推/拉的方式采集各种目标提供的指标数据。

#### 3.4.2 Prometheus四种指标类型

Prometheus中的指标可以分为以下四种类型：

- Counter（计数器）
    - 简介：Counter 是一个累加器，只能增加，不能减少。通常用于表示累积的事件计数，比如请求总数、错误总数等。
    - 典型用法：记录事件的总数量，例如 HTTP 请求总数、错误数量、任务完成次数等。
    - 示例：http_requests_total, errors_total, tasks_completed_total。
- Gauge（仪表盘）
    - 简介：Gauge 是一个可变化的数值，可以增加也可以减少。用于表示可变的度量，如温度、内存使用率等。
    - 典型用法：跟踪随时间变化的指标，例如 CPU 使用率、内存占用量、连接数等。
    - 示例：cpu_usage, memory_usage, active_connections.
- Histogram（直方图）
    - 简介：Histogram 统计和存储数据的分布情况，如请求响应时间的分布。
    - 典型用法：衡量持续时间或值的分布情况，例如请求响应时间、API 调用耗时等。
    - 示例：http_request_duration_seconds.
- Summary（摘要）
    - 简介：Summary 也用于记录持续时间数据，但它提供的是可变精度的摘要，而不是固定数量的桶。
    - 典型用法：与 Histogram 类似，用于记录持续时间，但通常用于更复杂的分布情况，比如 p50、p90、p99 等分位数。
    - 示例：api_request_duration_seconds_summary.

这四种指标类型几乎覆盖所有场景，并且每种类型都提供了丰富的标签（label）用于描述指标的维度信息。
我们只需要在推/拉数据时指定需要采集的指标类型和标签，Prometheus就能自动进行数据采集和存储。

#### 3.4.3 使用Grafana作为可视化组件

Prometheus本质上只是一个时序数据库，它本身并不具备强大的可视化能力。要想将采集到的指标数据进行丰富的可视化展示，
我们需要使用一个可视化组件，它就是Grafana。Prometheus+Grafana是一个常见的兄弟组合，几乎不会分开使用。

Grafana是一个开源的度量分析和可视化平台，它可以通过将时序数据导入其中而建立一个数据仪表盘。想象一下，
你只需要通过一个网页上的数据大盘就能对整个集群（包括几十上百甚至更多的节点）的运行状态了如指掌，这该是多么酷的一件事情。

当然这个兄弟组合并不仅仅用于Kubernetes集群监控，它还可以用于各种需要监控和可视化的场景。比如在你的业务场景中，
需要监控今/昨日的营收、昨日的PV、今日的UV、今日的订单量等。

#### 3.4.4 采集容器指标（cAdvisor）

cAdvisor是Google开源的一款用于展示和分析容器运行状态的可视化工具。通过在主机上运行CAdvisor用户可以轻松的获取到当前主机上容器的运行统计信息，
例如CPU使用率、内存使用量、网络带宽和磁盘使用量等。你可以参考 [cadvisor的安装与使用][cadvisor] 来进一步了解它的基本原理和使用方法。

cAdvisor暴露了Prometheus支持的指标格式，通过二者结合，我们可以轻松获取到Kubernetes集群中的Pod内部容器级别的监控数据。

> kubelet也是通过内置cAdvisor来监控容器指标。

#### 3.4.5 Metrics Server

Metrics Server是K8s的一个附加组件，它实现了API Server定义的[Metrics API][MetricsAPI]。
Metrics API主要为用户提供集群中处于运行状态的Pod和Node的CPU和内存使用情况，
设计用于K8s的HPA（Horizontal Pod Autoscaling，Pod水平自动伸缩）以及VPA（Vertical Pod Autoscaling，Pod垂直自动伸缩）功能。

Metrics Server内部通过调用kubelet API来监控容器数据，然后通过Metrics API暴露给API Server使用。
当安装Metrics Server后，我们可以使用`kubelet top`命令来查看集群中Pod和Node的CPU和内存使用情况。关于它的安装和使用细节，
你可以参考笔者的另一篇文章*K8s进阶教程*中的 [安装Metrics Server插件](doc_tutorial_senior.md#341-安装Metrics-Server插件)
一节。

了解更多：

- [Resource Metrics Pipeline](https://kubernetes.io/zh-cn/docs/tasks/debug/debug-cluster/resource-metrics-pipeline/)
- [Metrics Server](https://github.com/kubernetes-incubator/metrics-server)

#### 3.4.6 自定义指标

前面我们说到使用Prometheus+Grafana的组合来监控Kubernetes集群，这种方式已经可以监控任何的指标数据。
如果我们想要把Prometheus中存储的指标数据通过暴露给Kubernetes API Server，然后通过kubectl命令行来查询，
那我们可以通过自定义指标的方式来完成，这需要在集群中安装**prometheus-adapter**，
并在其配置文件中编写需要查询的指标信息，大致步骤请参考*K8s进阶教程*
中的[3.4.6 使用多项指标、自定义指标和外部指标](doc_tutorial_senior.md#346-使用多项指标自定义指标和外部指标)。

但请注意，如果仅仅是为了方便使用kubectl来查询指标，那其实大可不必，因为性价比太低（有一定维护成本），使用Grafana查询足以。
使用自定义指标更多是为了完成HPA（Horizontal Pod Autoscaling，Pod水平自动伸缩）和VPA（Vertical Pod
Autoscaling，Pod垂直自动伸缩）工作。

### 3.5 告警

有了指标数据后，我们需要根据SLO（服务水平目标）来设置相应的告警规则（在Grafana中设置）。SLO是对服务的某些可测量特性设置的目标期望，
例如可用性、吞吐量、频率和响应时间。如果没有SLO，我们对服务就会抱有不切实际的期望，也无法设置合适的告警规则。
对于Kubernetes这样服务具有高度自愈能力的系统，我们应该针对终端用户的服务体验来设置告警规则。
例如，为前端服务设置的SLO是响应时间不得高于20ms，当观测到一段时间内的平均响应时间高于20ms，就需要及时发出告警。

下面是一些常见的注意点以供参考：

- 仅对关键指标进行告警，并忽略一些可以被K8s自动处理的指标，例如CPU/内存利用率等
- 设置合理的阈值周期，避免过短的周期导致频繁告警。最后有一套阈值设置规范，来避免个性化的阈值设置。例如，可以遵循5min、10min、30min、1h这样的特定频率来统一配置阈值周期
- 在配置告警规则时，应该确保通知中包含必要的上下文信息，例如服务名称、告警持续时间、建议处理措施等
- 告警通知不要发给一群人，而是仅发给需要关注或处理问题的人，否则容易被当成不重要的信息而被忽略

## 4. 日志监控

日志监控是监控系统中的重要一环，它可以帮助我们快速定位问题和恢复服务。Kubernetes中的日志监控目标包含：

- 节点日志（节点关键服务的日志。例如容器运行时组件的日志、内核日志等）
- Kubernetes组件日志（如API Server、ControllerManager和Scheduler）
- 容器日志（主要是应用日志）
- Kubernetes审计日志（与权限相关，非常重要）

如果你使用云托管的Kubernetes集群，那建议你也使用托管的日志监控服务，这样有助于大幅降低运维成本。维护自建的日志服务起初看起来不错，
但随着环境复杂度的增长，维护工作会变得越来越费时费力。

如果选择自建日志服务，向你推荐笔者的另一篇文章[Kubernetes 日志收集](doc_log_collection.md)。
这篇文章会手把手指导你如何完成集群的日志收集工作。

## 参考

- [Kubernetes实战@美 Brendan Burns Eddie Villalba](https://book.douban.com/subject/35346815/)

[MetricsAPI]: https://kubernetes.io/zh-cn/docs/tasks/debug/debug-cluster/resource-metrics-pipeline/#metrics-api

[cadvisor]: https://learn.lianglianglee.com/专栏/由浅入深吃透%20Docker-完/08%20%20容器监控：容器监控原理及%20cAdvisor%20的安装与使用.md
