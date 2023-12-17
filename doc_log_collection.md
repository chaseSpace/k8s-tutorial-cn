## K8s日志收集

在Kubernetes（K8s）集群中，日志管理是关键的运维任务之一。由于容器化应用程序的特性，它们产生的日志通常分散在多个Pod中，因此有效地收集和分析这些日志对于故障排除、性能监控和安全审计至关重要。

除了容器应用日志以外，集群组件（控制面Pod、kubelet）也会产生日志，这些日志对于集群的运行和维护同样重要。

### 1. 日志分类

在K8s系统中，日志一共分为以下几类：

- 业务容器日志（stdout、stderr）
- K8s组件容器日志（包括apiserver/controller-manager/kube-proxy/scheduler）
- 节点上的kubelet日志（journald）

#### 1.1 业务容器日志

业务容器日志（stdout、stderr）由kubelet收集并**临时存储**到节点上（位于`/var/log/pods`），并且日志文件会随容器的删除而删除。
可使用`kubectl logs`命令查看Pod或容器日志。

> 同时，你还可以在`/var/log/container/`
> 下看到相应的容器日志文件，准确的说，这个目录下的文件只是一个个**符号链接**，它们映射到`/var/log/pods`
> 目录下日志文件。你可以通过`ls -l`命令查看链接目标。

**日志轮转**  
K8s环境中的容器应用日志（stdout&stderr）不会全部存储在节点上，而是通过日志轮转的方式限制单个容器产生的日志文件大小和数量。

日志轮转是K8s集群中一种常见的日志管理方式，通过配置日志文件大小、日志文件最大数量等参数，可以实现日志的自动轮转。

在K8s集群中，节点上的kubelet负责收集当前节点上运行的容器日志和轮转工作。你可以通过在节点上的`/var/lib/kubelet/config.yaml`
文件中添加`containerLogMaxSize`和`containerLogMaxFiles`
两个参数来配置日志轮转，具体配置方式[参考这里](https://kubernetes.io/zh-cn/docs/reference/config-api/kubelet-config.v1beta1/#kubelet-config-k8s-io-v1beta1-KubeletConfiguration)。

> 修改后记得使用`service kubelet restart`使配置生效。

默认情况下，kubelet的日志轮转配置如下：

```
containerLogMaxSize: 10Mi
containerLogMaxFiles: 5
```

笔者将配置分别修改为`4Mi`和`4`
后，通过`watch ls -lh /var/log/pods/default_hellok8s-logtest-fast-64b8597b68-bfw48_7570f83a-2c0b-4414-bc39-2ef7e259a703/hellok8s/`
观察[deployment_logtest_fast.yaml](deployment_logtest_fast.yaml)
中定义的Pod中容器`hellok8s`的日志轮转情况，具体如下：

```shell
Every 5.0s: ls -lh /var/log/pods/default_hellok8s-logtest-fast-64b8597b68-bfw48_7570f83a-2c0b-4414-bc39-2ef7e259a703/hellok8s/                                                                               Fri Dec 15 22:00:41 2023

total 9.6M
-rw-r-----. 1 root root 3.8M Dec 15 22:04 0.log
-rw-r--r--. 1 root root 343K Dec 15 22:02 0.log.20231215-220035.gz
-rw-r--r--. 1 root root 342K Dec 15 22:03 0.log.20231215-220205.gz
-rw-r-----. 1 root root 5.0M Dec 15 22:03 0.log.20231215-220312
```

这里可以看到两点信息：

- 产生的日志文件大小可能略微超过定义的值（受到容器日志的输出速率和kubelet轮转检查频率影响）
- 除了当前容器实例和上一个容器实例的日志文件外，kubelet会压缩之前的日志文件

修改轮转配置**不会影响**旧的（已完成写入或已压缩的）日志文件。

> 在使用集群初期，你可以不用着急搭建日志系统，并且把参数`containerLogMaxSize`调大一点。例如`20M`
> ，搭配默认参数`containerLogMaxFiles: 5`，一个容器就可以（在一个节点上）最多存储100M日志。

#### 1.2 K8s组件容器日志

这里的组件包括apiserver/controller-manager/kube-proxy/scheduler，并且他们都是以Pod形式运行在集群中。所以我们在收集的时候把他们当做普通的业务容器日志来处理。

#### 1.3 节点上的kubelet日志

在使用 systemd 的 Linux 节点上，kubelet 和容器运行时默认写入 journald。 你要使用 journalctl 来阅读 systemd
日志；例如：`journalctl -u kubelet`。如果 systemd 不存在，kubelet 和容器运行时将写入到 /var/log 目录中的 `*.log` 文件。

> journald 是一个系统日志服务，通常与 Linux 操作系统一起使用。它是 Systemd 套件的一部分，负责收集、存储和管理系统日志。
> journald以二进制方式存储管理日志，支持自动轮换和持久化存储，它使用专门的Journal查询语言来过滤日志。

journalctl 常用命令如下：

```shell
# 查看系统日志，pageUp和pageDown按钮进行翻页
# 在日志页面输入 ?pattern 使用正则过滤，比如 ?k8s.* 
journalctl --system

# 查看内核日志
journalctl -k

# 指定时间查找的几种方式
journalctl --since "2023-11-01 00:00:00" --until "2023-11-03 00:00:00"
journalctl --since yesterday
journalctl --since 09:00
journalctl --since 09:00 --until "1 hour ago"

# 查看指定服务单元的日志（默认按时间升序显示，添加-r倒序显示。首行会显示日志的时间范围）
journalctl -u kubelet

# 从尾部跟踪查看
journalctl -u kubelet -f

# -p 按0~7范围内的日志级别查看
journalctl -p 0 -u kubelet

# json格式输出
journalctl -o json
```

### 2. 日志收集方案

K8s官方本身没有提供原生的日志收集解决方案，但推荐了下面几种方案：

- 使用在每个节点上运行的节点级日志记录代理。
- 在应用程序的 Pod 中，包含专门记录日志的边车（Sidecar）容器。
- 将日志直接从应用程序中推送到日志记录后端。

这些方案各有优缺点，下面我们分别介绍。

#### 2.1 使用节点级日志代理

通过在节点上以DaemonSet方式部署日志代理，然后将节点上所有Pod的stdout&stderr输出作为日志收集的输入。

- 优点（相对Sidecar模式而言）：部署和维护成本低，资源消耗低；
- 缺点：需要统一所有容器的日志输出目录（需要映射到节点目录）和日志格式，灵活性和扩展性较差。
    - 此方式也无法通过`kubelet logs`命令查看容器日志，因为已经写入文件。

这种方式适用于业务不多的集群。

#### 2.2 使用Sidecar容器

这种方式还可细分为两种部署模式：

- Sidecar容器将应用容器的日志输出到自己的stdout（或直接传送到日志后端）；
    - 在部分场景下：可能在一个容器中输出了不止一条日志流（比如分为2个日志文件）用以区分不同业务日志，这时需要在一个Pod中部署2个Sidecar容器分别跟踪两个日志文件，以便在收集时区分。
    - [pod_two_sidecar_container.yaml](pod_two_sidecar_container.yaml)是来自官方的示例。
- Sidecar容器运行一个日志代理，收集应用容器的日志（stdout&stderr或文件）并传送到日志后端；
    - 建议应用容器通过stdout&stderr方式输出日志，否则无法通过`kubelet logs`
      命令即时查看容器日志（即使我们可以在日志后端查看，但有时通过`kubectl logs`命令更快）。
    - **后文将主要介绍这种方式**。

这种方式的利弊如下：

- 优点：每个Pod可以自定义Sidecar容器，灵活性高（但同时每个Pod都要定义Sidecar容器增加了维护工作，也是一种缺点）；
- 缺点：因为每个Pod都要运行Sidecar容器，相比节点级日志代理，资源消耗较高；

虽然占用资源高，但在大型集群中业务种类繁多，通常需要使用这种方式单独收集不同业务容器的日志，以便实现较好的隔离性。

#### 2.3 直接将日志推送到日志后端

这种方式是在应用代码中通过编码的方式将日志直接输出到日志后端。它有几个明显的缺点：

- 业务硬编码日志相关逻辑
- 性能影响：在高负载或频繁产生大量日志的情况下。直接将日志写入后端存储可能导致额外的网络开销和IO操作，影响应用的响应时间。
- 网络延迟和故障： 直接推送日志可能受到网络延迟和故障的影响。如果后端存储不可用，或者网络出现问题，可能导致日志数据的丢失或延迟
- 无法使用`kubelet logs`命令查看日志

综上，这种方式并不常用，不再过多介绍。

### 3. 使用EFK架构部署Sidecar模式

EFK架构是Kubernetes集群日志收集的常用架构（Sidecar模式），它由Elasticsearch、Fluentd和Kibana三大开源软件构成。
它们分别用途是：

- Elasticsearch（通常简称ES）：用于存储、索引和搜索大量的结构化和非结构化数据。
- Fluentd：用于采集、转换和发送日志数据到Elasticsearch，支持多种数据源。
- Kibana：用于可视化和分析从Elasticsearch中检索到的数据。

EFK架构的**工作流程**如下：

- Fluentd 收集日志： Fluentd 在集群中的每个节点上运行，收集来自应用程序、容器和其他服务的日志数据。Fluentd
  具有丰富的插件，可与各种数据源和目标集成。
- Fluentd 过滤和转发： Fluentd 可以对收集到的日志数据进行过滤和转换，然后将其发送到 Elasticsearch 进行持久性存储。这使得日志数据能够在
  Elasticsearch 中被高效地检索和分析。
- Elasticsearch 存储和索引： Elasticsearch 接收来自 Fluentd 的日志数据，将其索引到分布式的数据存储中。Elasticsearch
  提供灵活的查询语言，支持实时搜索和聚合操作。
- Kibana 可视化和查询： Kibana 提供 Web 界面，允许用户通过直观的界面查询和可视化 Elasticsearch
  中的日志数据。用户可以创建仪表板、图表和报表，以监控系统的状态和性能。

使用EFK架构可以帮助我们快速发现问题、进入故障定位和监控系统状态。ES的横向扩展能力，可以支持大规模集群的日志收集，
而且都是这几个组件都是开源项目，拥有活跃的社区支持和丰富的文档资源。

**Fluentd Vs Logstash**  
常用来与EFK比较的是ELK架构，它们的区别在于Fluentd（F）和Logstash（L）。之所以在集群中推荐使用Fluentd，是因为它是一个比Logstash更轻量级的日志收集器，注重简单性和性能。
Logstash是基于 Java 编写的，运行在 JVM 上，这导致它需要更多的资源来运行。Logstash支持更丰富的功能，包括多种输入和输出插件、强大的过滤器和转换功能，这也导致它更高的配置难度以及文档复杂性。

**Fluentd Vs Filebeat**  
Filebeat也是一个日志转发工具。它与Fluentd的区别在于，就像Fluentd比Logstash轻量一样，Filebeat比Fluentd还要轻量级。这体现在它支持更少的输入源，更少的输出模板以及更简单的过滤功能。
虽然Filebeat也支持通过插件扩展，但Fluentd已经有一个丰富的插件生态，并提供了一个大型的插件存储库，可以轻松集成用于各种目的，如数据收集，解析，缓冲等。

此外，还有其他的日志转发工具可用，例如Fluentbit、Vector（Rust实现，性能极高）和Promtail，读者可以自行了解。

下面的章节将介绍如何使用Helm来快速安装EFK各组件。

> Helm是K8s生态中一个非常流行的包管理工具，它允许用户通过一个chart包来安装和配置K8s集群上的各种应用。
> 你可以查看 [Helm手记](doc_helm.md)
> 来快速上手Helm。

#### 3.1 使用Helm部署ES

下面是安装步骤：

```shell
# 添加&更新repo
helm repo add elastic https://helm.elastic.co
helm repo update

# 搜索chart
$ helm search repo elastic/elasticsearch               
NAME                 	CHART VERSION	APP VERSION	DESCRIPTION                                  
elastic/elasticsearch	8.5.1        	8.5.1      	Official Elastic helm chart for Elasticsearch

# 下载&解压chart
helm pull elastic/elasticsearch --version=8.5.1 --untar

$ ls elasticsearch                                
Chart.yaml  examples  Makefile  README.md  templates  values.yaml
```

接下来，需要编辑`values.yaml`以适应我们的需求。[values.yaml](helm/elasticsearch/values.yaml)
是一个副本示例。下面`values.yaml`中常见的修改位置：

```yaml
# 笔者只有一个普通节点（默认3）。
# 由于是statefulSet部署并且设置了Pod反亲和性（表示每个节点最多存在一个es pod），所以副本数量不应该超过节点数量。
replicas: 1
minimumMasterNodes: 1 # 默认最小2个master实例

# 因为笔者的测试节点内存较少，所以修改默认资源配置
resources:
  requests: # 降低
    cpu: "100m"
    memory: "200M"
  limits: # 不变
    cpu: "1000m"
    memory: "2Gi"

# 设置ES密码（留空自动生成）
secret:
  enabled: true
  password: "123"

# 客户端连接端口（kibana使用）
protocol: https
httpPort: 9200

# 默认传输数据的端口
transportPort: 9300

# 默认开启持久化（否则数据存在内存中）
# 且需要30G空间（不会立即占用，部署时也不会检查磁盘可用>=30G）
persistence:
  enabled: true
volumeClaimTemplate:
  accessModes: [ "ReadWriteOnce" ]
  resources:
    requests:
      storage: 30Gi
```

> 你可以在[这个页面](https://github.com/elastic/helm-charts/blob/main/elasticsearch/README.md)找到每个字段的解释。

部署前检查chart生成的各项K8s对象模板：

```shell
helm install --dry-run --debug es ./elasticsearch
```

提取去普通节点拉取镜像（如果是本地仓库则不需要）：

```shell
ctr -n k8s.io i pull docker.elastic.co/elasticsearch/elasticsearch:8.5.1
```

准备存储后端（使用[storageclass_hostpath_es.yaml](storageclass_hostpath_es.yaml)）：

- 注意：elasticsearch是数据库类应用，需要用到磁盘，在实际部署时需要为其提前准备。
- 如果只是体验，可以将上面的`persistence.enables`设置为`false`，这样数据会保存在内存中。

```shell
kk apply -f storageclass_hostpath_es.yaml

$ kk get sc,pv         
NAME                                        PROVISIONER                    RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
storageclass.storage.k8s.io/elasticsearch   kubernetes.io/no-provisioner   Delete          WaitForFirstConsumer   true                   6m12s

NAME                             CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS      CLAIM   STORAGECLASS    REASON   AGE
persistentvolume/elasticsearch   100Gi      RWO            Retain           Available           elasticsearch            6m12s

# 在部署es的目标节点执行
$ mkdir /home/k8s-elasticsearch-data
```

安装chart：

```shell
# 将es安装在单独的namespace: efk 中，同时设置storageClassName以便在对应的sc中申请存储卷
helm install elasticsearch elasticsearch \
  --set volumeClaimTemplate.storageClassName=elasticsearch \
  -n efk --create-namespace
```

查看部署状态（主要观察Pod是否`Running`状态）：

```shell
helm status elasticsearch --show-resources -nefk
```

需要注意的是，笔者首次部署时遇到了普通节点内存不足（2g，节点终端响应慢至无法操作）导致elasticsearch无法启动的问题，将节点内存扩容到4g后正常启动。
通过`kubectl top po`看到单个es pod占用了1.5Gi内存。

> `kubectl top`命令需要安装Metrics-Server组件才能正常运行，
> 可参阅[K8s 进阶教程][K8s 进阶教程]。


[K8s 进阶教程]:https://github.com/chaseSpace/k8s-tutorial-cn/blob/main/doc_tutorial_senior.md#341-安装metrics-server插件

通过命令查看ES的初始账号`elastic`的访问密码（自动生成或忘记时可能用到）：

```shell
kubectl get secrets --namespace=efk elasticsearch-master-credentials -ojsonpath='{.data.password}' | base64 -d
```

#### 3.2 使用Helm部署Kibana

下载并解压chart包：

```shell
helm pull elastic/kibana --version=8.5.1 --untar
```

编辑`values.yaml`，[values.yaml](helm/kibana/values.yaml)是一个副本示例。下面是常见的修改位置：

```yaml
# 默认1个Pod副本，无需更改
replicas: 1

# 笔者测试环境内存不足，所以降低一些资源消耗
resources:
  requests:
    cpu: "100m"
    memory: "200Mi"
  limits:
    cpu: "1000m"
    memory: "2Gi"

# web端口，默认5601
httpPort: 5601

# 为了方便测试（跳过ingress），service使用NodePort类型
service:
  type: NodePort

# 其中的 `elasticsearchCredentialSecret` 字段用以配置访问ES的密码，它会与前面安装ES时使用的Secret名称一致
```

> 你可以在[这个页面](https://github.com/elastic/helm-charts/blob/main/kibana/README.md)找到每个字段的解释。

检查模板：

```yaml
helm install --dry-run --debug kibana ./kibana
```

提取去普通节点拉取镜像（如果是本地仓库则不需要）：

```shell
ctr -n k8s.io i pull docker.elastic.co/kibana/kibana:8.5.1
```

安装chart：

```shell
# 安装在单独的namespace: efk 中
helm install kibana ./kibana -n efk
```

查看部署状态（主要观察Pod是否`Running`状态）：

```shell
# 注意记下service对象中5601映射到节点的端口，假设为32498
helm status kibana --show-resources -nefk
```

通过浏览器访问Kibana（部署Kibana的节点地址）：http://kibana_node_host:32498，输入初始账号`elastic`，密码通过上一小节获取。

> 你可能需要为Kibana安装插件，参考[这个页面][kibana-plugin]。

[kibana-plugin]:https://github.com/elastic/helm-charts/blob/main/kibana/README.md#how-to-install-plugins

#### 3.3 使用Helm安装Fluentd

TODO