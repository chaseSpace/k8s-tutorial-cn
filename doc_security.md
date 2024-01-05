## K8s安全分析

声明，本篇文章的内容主要来自[Kubernetes修炼手册 @Nigel Poulton](https://book.douban.com/subject/35486781/)。

### 1. 安全模型

本文采用STRIDE模型来对K8s进行安全分析，该模型定义了6种潜在威胁：

- 伪装
- 篡改
- 抵赖
- 信息泄露
- 拒绝服务
- 提升权限

下文将对这些威胁进行详细分析。

### 2. 伪装

在信安领域，伪装指攻击者为了获得更多的系统权限而冒充另一个人或主体。

#### 2.1 与API Server的安全通信

K8s是一个由多个组件构成的系统，它们包括：

- API Server（Pod）
- Controller manager（Pod）
- Scheduler（Pod）
- Store（etcd）
- kube-proxy（Pod）
- kubelet（journald）

以上这些组件基本上都是与API Server进行通信的，K8s在组件通信之间采用了Mutual TLS（mTLS）认证。
这要求通信双方都要提供自己的证书给对方进行身份验证，这种方式区别于传统的单向证书认证（仅限于客户端验证服务端）。

K8s内部通过自旋证书来简化了mTLS的实现。简单来说，K8s部署时自动生成了一个自签名的CA，这个CA将用来为集群内的所有组件颁发证书。
所以mTLS的安全性依赖于CA的可靠性，CA私钥的泄露将导致整个集群组件之间的通信彻底陷入危险。
除此之外，还需要注意：

- CA证书仅在集群内使用
- 使用CA批准证书签名请求（CSR）时保持严谨
- 确保CA不会被系统外的任何组件设置为可信CA

此外，API Server还可能与集群外的组件进行通信（比如Webhook）。这时候推荐使用两套不同的可信秘钥——分别用于认证内部组件和外部组件。
要实现这一点，K8s需要将一个集群外的CA添加为可信CA，然后对于集群外的组件，应当使用外部CA颁发的证书来与API Server进行通信。

#### 2.2 Pod间的安全通信

Pod间通信的认证方式可以通过Service Account（SA）来实现。每个Pod启动时默认都会自动分配一个ServiceAccount作为Secret挂载到Pod中。
并且该SA是被允许访问API Server的，只是权限受限，但我们要知道，大多数Pod并不需要访问API Server。

所以，对于不需要与API Server通信的Pod，我们建议将Pod清单中的`automountServiceAccountToken`属性设置为`false`。
如果需要挂载SA，那么有些非默认的配置需要了解一下：

- expireSeconds：设置token有效期
- audience：限制token的受众

这些属性的具体示例，你可以在官方找到。

### 3. 篡改

篡改通常基于以下目的：

- 拒绝服务：篡改资源使其不可用
- 提高权限：篡改资源获取额外权限

#### 3.1 对K8s组件的篡改

K8s系统内可以篡改的资源有以下几种：

- etcd
- 配置文件：API Server、ControllerManager、Scheduler、Kube-proxy和Kubelet
- 容器运行时的二进制
- 容器镜像
- K8s的二进制

篡改行为通常发生在（网络）传输和保存过程中。TLS可以确保数据的完整性，下面的建议有助于防范对保存在集群中数据的篡改攻击：

- 严格限制对运行由K8s组件的服务器的访问，尤其是部署了控制层组件的节点
- 严格限制对保存有K8s配置文件的库的访问
- 仅在最初部署K8s时进行ssh访问节点
- 对下载的二进制文件进行哈希验证
- 严格限制镜像仓库及相关库的访问

此外，建议在生产环境中对关键组件的二进制文件的审计和监控，相关的工具有`auditctl`等。

#### 3.2 对于运行在K8s中的应用的篡改

推荐将Pod中容器的文件系统设置为只读是一种推荐的办法。我们可以在Pod的清单中设置`securityContext.readOnlyRootFilesystem`
属性为`true`，
也可以通过部署PodSecurityPolicy对象来全局性的限制所有Pod的文件系统的读写权限。

> PodSecurityPolicy特性从v1.25开始被删除，转而使用[Pod 安全性标准][Pod 安全性标准]进行代替。

### 4. 抵赖

抵赖就是制造对某件事的不确定性。不可抵赖就是提供证据（以证实某事），具体来说应该能够证明以下信息：

- 发生了什么
- 什么时间发生的
- 谁操作的
- 在哪里发生的
- 为什么发生的
- 如何发生的

对于后两个信息，通常需要一段时间内的多个相关事件的信息。

K8s提供针对API Server的审计（Audit）功能来完成回答以上问题，该功能需要手动开启，请参考官方文档[审计][审计]部分。

然而，除了API Server，K8s还提供了对其他组件的审计，比如容器运行时、Kubelet等各应用的审计日志。如果要收集多个组件的日志，
那么就需要一个中心化的日志后端来实现对事件的保存和分析。一种常见的做法是在每个节点上部署DaemonSet类型的日志代理来收集日志，
然后发送至中心日志数据库，同时要确保这个中心化的日志库是安全的。

### 5. 信息泄露

主要是指敏感数据的泄露。

#### 5.1 保护集群数据

K8s中的所有集群配置都是保存在集群存储中的（目前是etcd），包含网络和存储配置以及Secret形式保存的密码登敏感数据。
这就使得集群存储会成为被攻击的首要目标。

我们必须对运行由集群存储的节点进行访问限制和审计。

#### 5.2 保护Pod中的数据

前面提到，K8s提供了Secret对象来保存密码等敏感数据。但请注意，Secret是以未加密的形式存储在集群存储中的，
我们可以为其使用静态加密，请参考官方文档[Secrets良好实践][Secrets良好实践]部分。

### 6. 拒绝服务

这种方式的攻击目的在于使服务不可用。在K8s中，拒绝服务的最大可能攻击对象就是API Server。

#### 6.1 保护集群资源免于DoS攻击

首先，我们应该对主节点进行高可用（HA）部署。

进一步，我们还可以考虑将主节点部署在多个可用域中，
这样可以避免某个可用域的网络遭受故障导致整个集群不可用。这个防范原则也适用于工作节点。

此外，我们还应当为以下资源配置限额（参考`ResourceQuota`对象）：

- 内存
- CPU
- 存储
- K8s对象（ReplicaSet、Pod、Service、Configmap、Secret等）

添加配额限制有助于避免重要系统资源被消耗殆尽，从而提高抵御DoS的能力。

#### 6.2 保护API Server防范DoS攻击

参考以下方法：

- 高可用部署主节点
- 对到达API Server的请求进行合理监控和预警
- 不将API Server暴露在互联网上（借助防火墙/安全组规则）

即使K8s拥有良好的鉴权机制，也需要注意妥善保管具有较高权限的账号。

#### 6.3 保护集群存储防范DoS攻击

集群存储（etcd）的重要性已经不需要过多强调，参考一下方法加固etcd：

- 将etcd部署为3个或5个节点的集群
- 对etcd收到的请求进行监控和预警
- 在网络层面对etcd进行隔离，只允许控制平面组件与其交互

默认情况下，K8s会将etcd安装到与控制平面组件相同的节点上，对于非生产环境这是OK的。但是生产环境就不太合适了，
应该考虑为K8s单独部署一个etcd集群，有助于提高系统的性能和弹性。

在性能方面，etcd可能是大型K8s集群的瓶颈所在。所以在集群部署阶段应当进行适当的性能测试，以确保整体架构能够在较大规模下维持较高性能。
性能不足的etcd集群的表现基本等同于正在遭受DoS攻击的etcd集群。

#### 6.4 保护应用组件防范DoS攻击

多数Pod会将其服务暴露于互联网之上，如果没有采取合理的控制，任何人都可以访问Pod并对其实施DoS攻击。
好在，K8s支持对Pod进行资源请求限制。推荐下面的安全措施：

- 对Pod之间和Pod与外部的通信配置网络安全策略以控制流量的出站/入站（NetworkPolicy）
- 利用mTLS和基于Token的API认证来提供（Pod层面）应用级的认证

### 7. 提升权限

#### 7.1 保护API Server

K8s提供以下鉴权方式来保证API Server的安全性：

- RBAC（基于角色的访问控制）
- webhook
- 节点认证

建议同时开启多个鉴权机制。例如，一种常见的方式是同时启用RBAC和节点两种鉴权机制。

RBAC模式能够指限制哪个用户（组）能够对哪些资源执行哪些操作。Webhook模式则是将鉴权工作交给外部的基于REST接口的策略引擎来完成。
不过需要额外搭建和维护一套外部引擎，且这个引擎也可能会带来API Server的单点故障隐患，因为API Server会持续调用外部引擎来完成每一次鉴权。
所以一旦外部引擎宕机，则API Server无法继续完成请求。鉴于此，应该严格谨慎地对待webhook鉴权服务的设计与实现。

节点认证模式是指对来自kubelet的请求进行鉴权。对节点请求的鉴权工作由节点鉴权器（node authorizer）完成。

#### 7.2 保护Pod

下面提供一些手段来显著降低以Pod和容器为目标的"提升权限"攻击的风险。

- 避免进程以root身份运行（通过PodSpec中的`spec.securityContext.runAsUser`，此属性还可以在容器层面进行配置）
    - 若Pod由不同容器组成，建议为不同容器配置不同用户，以避免容器之间互相干扰
- 限制capabilities
    - 在linux内部，root用户的权限是由不同的capabilities的权限组合而成的，比如，名为`SYS_TIME`的capability允许用户设置系统时钟。
    - `NET_ADMIN`允许用户执行网关相关操作（如修改本地路由表、配置本地接口）
    - root用户拥有所有的capability
    - 我们可以在配置非root用户的同时，为其添加capability来满足容器需求
- 过滤系统调用
    - Seccomp是linux自 2.6.12 以来一直是Linux 内核的一个特性。它可以用来沙箱化进程的权限，限制进程从用户态到内核态的调用。
- 避免权限提升
    - linux中允许子进程申请比父进程更多的权限。我们可以通过`spec.securityContext.allowPrivilegeEscalation`属性来禁止权限提升
- 配置selinux

这些功能配置基本都是通过`spec.securityContext`
属性来完成的。具体配置参考官方文档的[为 Pod 或容器配置安全上下文][为 Pod 或容器配置安全上下文]。

**Capabilities**  
如今[linux-capabilities][linux-capabilities]总共有30个，我们可以在`spec.securityContext.capabilities`
中配置容器所拥有的capabilities。

### 8. K8s安全展望

2019年CNCF委托了一个第三方机构对K8s进行了安全审计。通过安全威胁建模、人工代码审查、动态渗透测试和加密审查等方面的审计方法发现了一些安全隐患。
所有的隐患都给出了难度和严重程度级别。这次审查非常细致，本着负责人的态度，所有严重级别的隐患都在对外发布前被修复了。

不过，仍然有许多问题等待社区来解决。

### 9. 一些实践建议汇总

**CI/CD流水线**：

- 使用私有镜像仓库（或公共仓库如DockerHub中的私有repository）
    - 并且使用分离的repository（如开发、测试、生产）
- 使用已验证的基础镜像
- 控制镜像库的访问权限（push/pull）
    - 生产repo限关键用户拥有push权限
    - 限制用户进行push/pull镜像
    - 限制某些客户端节点能访问镜像库
- 整合漏洞扫描
    - 在扫描到镜像漏洞后自动令构建流水线失败
- 审查配置文件（Dockerfile、K8s YAML文件）
- 使用镜像签名，确保镜像完整性
    - push时添加签名，pull时验证签名

**基础设施隔离**：

- 使用K8s命名空间隔离工作负载
    - 对单一命名空间进行资源限额
    - 创建单独的用户管理特点的命名空间
- 节点隔离
    - 对于需要特殊权限（如root）运行的应用Pod，应将其部署到特定的节点（通过亲合度/污点/标签）
- 运行时隔离
    - 容器和虚拟机的隔离性是不同的
    - 多个容器会共享一个CPU内核，隔离性由内核指令提供。是一种软性隔离，不如虚拟机。
    - 虚拟机的隔离性由硬件虚拟化技术提供，每台虚拟机独享一个硬件CPU内核
    - 可以使用不同容器运行时（如gVisor和Kata）来提供更高级别的Pod隔离性
        - 首先为特定节点安装配置特定的容器运行时
        - 然后使用Runtime Class（K8s v1.20 stable）和nodeSelector将Pod调度到这类节点

**网络隔离**：

- K8s中的Pod网络通过CNI（Container Network Interface）实现，主流实现分为Overlay和BGP两类
- 使用K8s的网络策略（NetworkPolicy）来限制Pod之间的通信，支持跨命名空间

**身份认证和访问控制管理**：

- 充分使用RBAC系统
- 严格限制哪些人员能够ssh到控制平面节点的权限
- 部署审计和监控系统，应对未来某个时候被攻破的情况
- 安全配置
    - 在PodSpec中限制容器以非root身份运行
    - 信息安全中心（CIS）发布了一套针对K8s安全性的行业标准要求
    - Aqua Security公司编写了一个易于使用的K8s安全评估工具kube-bench来执行对节点的CIS要求的测试

**审计和安全监控**：

- 保留容器和Pod的生命周期事件
    - 通过采集每个节点上的容器运行时日志发送到外部存储（如ES），以便在未来调查问题时使用
- 采集每个节点上的Pod日志
- 记录每个节点上的命令执行，发送到日志中心（可以采用跳板机）

### 参考

- [Kubernetes修炼手册 @Nigel Poulton](https://book.douban.com/subject/35486781/)

[Pod 安全性标准]: https://kubernetes.io/zh-cn/docs/concepts/security/pod-security-standards/

[审计]: https://kubernetes.io/zh-cn/docs/tasks/debug/debug-cluster/audit/

[Secrets良好实践]: https://kubernetes.io/zh-cn/docs/concepts/security/secrets-good-practices/

[linux-capabilities]: https://linux-audit.com/linux-capabilities-hardening-linux-binaries-by-removing-setuid/

[为 Pod 或容器配置安全上下文]: https://kubernetes.io/zh-cn/docs/tasks/configure-pod-container/security-context/