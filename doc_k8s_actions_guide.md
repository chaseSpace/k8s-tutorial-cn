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

应用上线后，我们可能会有更改应用配置的需求。一般的做法是直接更新已有的ConfigMap，然后重启所有Pod。
但这不是K8s的最佳实践。原因有以下几个：

- 更新ConfigMap不会触发Deployment中所有Pod重启
- 若更新后的配置有问题不能迅速回滚（需要再次编辑已有ConfigMap），导致服务短暂宕机

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
默认的应用日志分散存储在每个节点的`/var/log/pods`目录，且它们会伴随Pod的消失而被删除。
这不利用开发者们查看日志的需求，因此我们需要一个中心化存储日志的方案。
请参考[Kubernetes 日志收集](doc_log_collection.md)来了解如何搭建日志收集系统。

如果因为某些原因不希望搭建日志收集系统（或者说是偷懒~），
你也可以通过修改每个节点上的kubelet配置来调整容器日志的存储大小和文件数量，具体步骤也请参考[Kubernetes 日志收集中的
*业务容器日志*](doc_log_collection.md#11-业务容器日志)。
其次，你还可以参考[Kubernetes 维护指导中的*日志查看*](doc_maintaintion.md#15-日志查看)来使用第三方工具来高效查询容器日志。

### 2.4 启动开发

开发过程中，开发者需要频繁地构建并推送镜像、更新应用、查看应用日志。
这其中最为重要的步骤就是镜像tag定义以及更新应用的操作。

**镜像tag使用版本化语义而不是latest**  
在构建用于开发环境的自测试镜像时，对于镜像tag，开发者最好是使用版本化语义而不是`latest`。
使用`latest`可以免去每次构建镜像时都需要手动修改tag的麻烦，但同时也带来了不确定性。因
为你无法完全确定所更新的应用使用了你刚刚构建并推送的镜像。倘若你将误以为自测成功的代码发布到了生产环境，
难以想象将会发生的事情和面临的后果😑。

**更新应用**  
在使用版本化语义的镜像tag后，我们可以更从容的使用`kubectl set image...`命令来更新应用。
注意，在开发以及后续的上线过程中，我们都不需要修改代码库中的Deployment YAML文件。

### 2.5 使用第三方工具为开发提速

#### 2.5.1 IDE插件

编写此文时已经是2024年了，主流的VSCode和Jetbrains家族IDE都已经有大量的Kubernetes插件可用，
这些插件可以帮助开发者通过图形化的方式与开发环境的K8s集群进行便捷交互，免去手动输入kubectl命令的繁琐。

#### 2.5.2 K9s终端面板

K9s是一个终端形式的K8s资源面板，它支持在终端中以可视化方式查看和管理Kubernetes集群中的资源，包括Pod、Deployment、Service等。
它支持以方向键、回车键和空格键与面板交互，免去手敲命令的麻烦。

我们可以在测试环境和预发布环境（也包括生产环境）安装K9s，这样可以更便捷的查看应用状态、日志、事件、以及进入Pod内的容器Shell，极大地改善了K8s的使用体验。
