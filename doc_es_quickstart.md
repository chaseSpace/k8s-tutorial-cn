# ElasticSearch快速上手

## 1. 简介

ES是一个基于Lucene的开源的、分布式的、支持多用户的、由Java实现的一个业界非常流行的全文实时搜索引擎。它还拥有以下特征：

- 基于RESTful web接口
- 海量数据规模下的性能表现稳定，接近实时返回结果
- 支持水平扩展
- 支持多种编程语言进行客户端开发
- 大量网站和企业使用
    - Wikipedia
    - Stack Overflow
    - GitHub

### 1.1 ES VS Solr

- ES部署安装简单，自带分布式协调机制。Solr则需要安装Zookeeper
- ES开箱即用，Solr上手成本略高
- Solr支持多种数据格式的文件，如JSON、XML、CSV等。而ES仅支持JSON
- Solr数据搜索速度快，但插入和删除慢；而ES都比较快
- Solr提供的功能繁杂；ES则注重核心功能，高级功能由第三方插件提供，如Kibana提供GUI功能。

### 1.2 为什么学习ES

好处如下：

- 全文搜索是企业平台常用的功能，ES可提供良好的体验
- ES具备强大的数据分析功能
- ES部署方便（单机或集群）
- 国内很多互联网公司都在使用ES，学习ES有助于提升竞争力

### 1.3 ES的主要功能和应用场景

1. 海量数据的分布式存储以及集群管理
2. 近乎实时的数据搜索能力。支持对（非）结构化数据、地理位置等类型数据进行处理和分析
3. 各种聚合分析功能，如统计、排序、过滤、分组等

应用场景：

1. 网站搜索、代码搜索等
2. 日志管理和分析，应用性能监控、web舆情分析等
3. 对海量业务订单进行分析和处理，利用ES的聚合函数和分析功能统计处各种各样的数据报表

### 1.4 ELK架构

ES是与Logstash（数据采集和转换）和Kibana（数据分析和可视化）一起开发的，它们共同组成ELK架构。

> 最早Logstash是由一个小公司开发的，在2013年被Elastic公司收购。

一个常见的例子是使用Logstash采集Nginx日志，并将其发送到Elasticsearch，然后使用Kibana进行可视化展示。

- [ES][ES]
- [Logstash][Logstash]
- [Kibana][Kibana]

### 1.5 Beats

[Beats][Beats]是Elastic公司使用Go语言开发的**一组**比Logstash更轻量的数据采集器，用于采集服务器、网络设备等数据源的数据，并将数据发送到Logstash或Elasticsearch。
它可以Sidecar模式与应用容器一同部署，也可以按节点维度部署。

Beats家族有多个成员，包括Auditbeat、Filebeat、Metricbeat、Packetbeat、Winlogbeat等，它们分别用于采集不同类型的数据。

Beats通常由两种部署模式：

- 在中小规模架构中，通常在每个节点或K8s Pod中部署一个Beats组件，将采集数据直接发送到Elasticsearch
- 在大规模架构中，可能需要先将Beats采集到的数据发送到Logstash，再由Logstash将数据发送到Elasticsearch
    - 这是因为Logstash将提供比Beats更丰富的过滤器和插件来处理数据，以及多数据源聚合、对ES节点的负载均衡和缓冲的功能
    - Logstash还可以持久化数据，这在ES集群不可用时非常有用，降低数据丢失的可能性

## 2. 安装ES

ES的安装方式有多种，可以参考官方文档：[Installing Elasticsearch](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html)。

这里由于是快速入门，因此使用Docker来安装ES（常用于开发和测试环境）。

### 2.1 Docker安装ES

首先需要找到Docker的设置：Preferences > Resources > Advanced，调整内存至少4GB。然后执行下面命令：

```shell
docker network create elastic

# ES_JAVA_OPTS限制ES使用的内存大小，避免ES占用过多宿主机内存。其中Xms表示min，Xmx表示max，二者应设置同一个值
# 若是开发环境，可以添加 -e xpack.security.enabled=false 关闭身份验证，关闭后Kibana也可无密码连接es
docker run --name elasticsearch --net elastic \
  -p 9200:9200 \
  -e "discovery.type=single-node" \
  -e ES_JAVA_OPTS="-Xms512m -Xmx512m" \
  -t library/elasticsearch:8.11.3
```

在输出的日志中，搜索关键字`current.health`会看到一下日志行（其中包含了集群信息、节点基本信息如node-id、node-name等）：

```json
{
  "@timestamp": "2024-01-05T09:08:45.532Z",
  "log.level": "INFO",
  "current.health": "GREEN",
  "message": "Cluster health status changed from [YELLOW] to [GREEN] (reason: [shards started [[.security-7][0]]]).",
  "previous.health": "YELLOW",
  "reason": "shards started [[.security-7][0]]",
  "ecs.version": "1.2.0",
  "service.name": "ES_ECS",
  "event.dataset": "elasticsearch.server",
  "process.thread.name": "elasticsearch[b527ebb76d2b][masterService#updateTask][T#1]",
  "log.logger": "org.elasticsearch.cluster.routing.allocation.AllocationService",
  "elasticsearch.cluster.uuid": "yz51YWzAR6CJFTr_xQlVFQ",
  "elasticsearch.node.id": "3dE37A4dTxOOSYOiqw2p3Q",
  "elasticsearch.node.name": "b527ebb76d2b",
  "elasticsearch.cluster.name": "docker-cluster"
}
```

> 如果设置`xpack.security.enabled=false`，则docker日志是以`adding index template`结束。

从8.0版本开始，ES默认为集群启用了各种安全设置（包含TLS、默认用户名密码等）。
第一次启动Elasticsearch时，生成默认用户名和密码和Kibana注册令牌会输出到终端。通过搜索`Password`得到：

```
ℹ️  Password for the elastic user (reset with `bin/elasticsearch-reset-password -u elastic`):
  {elastic账号密码}
ℹ️  HTTP CA certificate SHA-256 fingerprint:
  {CA证书SHA256指纹}
```

容器日志中还包括Kibana注册token（关键字`Configure Kibana`）、新节点加入集群的token（关键字`join this cluster`）。

> ES在生产环境中是多节点部署的，这里简单起见，以单节点方式启动ES集群，

使用下面的API端点可以查看集群状态：

```shell
# 如果设置xpack.security.enabled=false，则使用http
curl https://localhost:9200/_cat/health
curl https://localhost:9200/_cluster/health?pretty=true

# 查看集群信息
curl https://localhost:9200
```

### 2.2 Docker安装Kibana

ES通过REST APIs与外界交互，包含接收数据和其他请求。通常，我们：

- 使用[各种编程语言][Client SDKs]中的ES client完成搜索请求
- 使用Kibana完成日常管理ES需求（配置索引、报表等）
- 其他日志采集工具向ES发送数据

所以如想要图形化使用ES，需要安装Kibana。参照下面的命令：

```shell
docker run --name kibana --net elastic -p 5601:5601 \
  -e "i18n.locale=zh-CN" \
  library/kibana:8.11.3
```

复制最后日志输出中的链接到浏览器中，替换`0.0.0.0`为`localhost`，回车。将上一步中得到的Kibana注册token复制到页面中的输入框，再确认。
此刻Kibana将完成ES的连接配置。完成配置后，使用`elastic`和上一步中得到的密码登录ES，然后点击`Explorer on my own`
开始通过Kibana使用ES。

Kibana还方便的为开发者提供了控制台（**Management > Dev Tools**），用于调试ES。

### 2.3 ES配置

生产环境需要根据实际情况对ES进行配置调优，其中大部分设置可以在集群启动后使用
[Cluster update settings API][ClusterupdatesettingsAPI] 进行修改。

还有很多配置需要我们在安装前进行修改，这些配置很多是节点特定的，每个节点使用的配置不会完全一致。

ES的配置目录根据安装方式而定：

- 对于归档发行版（`tar.gz`这种）的安装方式，配置目录为`$ES_HOME/config`
    - 这个目录下的配置文件存在升级时被删除的风险，建议修改到`$ES_HOME`之外
- 对于软件包发行版，配置目录为`/etc/elasticsearch`。但某些发行版可能不同
    - Debian发行版：`/etc/default/elasticsearch`
    - RPM发行版：`/etc/sysconfig/elasticsearch`
- 对于Docker方式，配置目录为`/usr/share/elasticsearch/config`

以上都可通过ENV方式修改配置目录，例如`ES_PATH_CONF=/etc/elasticsearch`。

配置文件布局：

```shell
$ ls /usr/share/elasticsearch/config
certs                              elasticsearch.keystore  jvm.options    log4j2.file.properties  role_mapping.yml  users
elasticsearch-plugins.example.yml  elasticsearch.yml       jvm.options.d  log4j2.properties       roles.yml         users_roles
```

其中常修改的配置文件有：

- `elasticsearch.yml`是ES的核心配置文件，[查看示例](efk-arch/es-master.yml)
- `jvm.options`和`jvm.options.d`是ES JVM的配置文件（性能调优）
- `log4j2.properties`和`log4j2.file.properties`是ES日志的配置文件

其他文件用途如下：

- `elasticsearch.keystore`：存储 ES 实例的安全信息和敏感配置。替代以前那种明文存储秘钥的方式
- `elasticsearch-plugins.example.yml`：ES插件的示例配置文件
- `roles.yml` 和 `role_mapping.yml`：配置角色（授权用户的访问权限）和角色映射，一般不需要修改（而是通过Kibana方式修改）
- `users` 和 `users_roles`：存储用户信息和用户与角色的关系，用于配置 ES 的身份验证和授权，一般不需要修改（理由同上）

### 2.4 安装多节点集群

在2.1小节中介绍的是如何安装单节点集群，这种方式你可能接触不到配置文件的修改以及各字段的含义。
本节将介绍如何安装多节点集群，这个过程中你将会进一步熟悉ES的安装步骤。

集群角色介绍：

- 主节点（master）：ES集群中同一时刻只有一个主节点（从多个具有master角色的节点中选举出），相当于集群大脑。负责管理集群状态，例如选举、索引创建/删除、分片迁移等。
    - 大规模集群中，一般会规划仅`master`节点，这样可以让主节点避免运行高负载任务导致无法响应更重要的master请求。
- 数据节点（data）：存放数据的节点，负责数据的增删改查。
    - 此节点通常执行I/O、内存和CPU密集型的操作，需要重点监控。并在必要时添加数据节点
    - 大规模集群中，一般会规划仅`data`节点
- 仅投票节点（master+voting_only）：有投票权，没有被选举权，通常用于降低集群资源消耗（因为减少了一个master节点）
    - 一个多节点es集群至少需要3个主节点，其中需要至少2个非仅投票节点（所以可以是2master+1voting_only的组合）
    - 由于永远不会被选举为master，所以仅投票节点的CPU/内存配置可以低于标准master节点
    - 仅投票节点可以与`data`角色组合作为仅投票的数据节点（master+voting_only+data）
- 远程客户端（remote_cluster_client）：跨群集搜索时需要，较少使用。

其他还有Ingest和ML等非必要角色，在使用到其他功能时需要。

一个高可用ES集群至少需要3个有资格成为master的节点，其中至少两个不是仅投票（`voting_only`）节点。

下面介绍安装的是一个比较标准的多节点ES集群配置，如下：

- 1个仅master节点
- 2个master+data节点

配置文件如下：

- [docker-compose.yml](efk-arch/docker-compose.yml)
    - 其中的`setup`容器负责完成证书生成、es节点的健康监控工作
    - 此文件对比官方配置增加了几项内容：
        - 给不同节点设置角色（而不是默认）
        - 为kibana配置中文语言
        - 为kibana开启ssl
        - 为kibana配置所有master资格节点作为es连接主机
- [.env](efk-arch/.env)：部分docker-compose文件引用的环境变量需要根据实际情况修改（其中包含es密码设置）

启动集群：

```shell
docker-compose up -d
```

查看集群容器状态：

```shell
# 其中setup容器在完成任务后会变成exited状态
docker-compose ps
```

若容器异常，可查看容器日志进行排查。

测试完成后，清理创建的资源：

```shel
docker-compose stop # 停止文件内定义的容器
docker-compose down # 停止并删除文件内包含的容器和网络，保留volume
docker-compose down -v # 停止并删除文件内包含的容器、volume和网络
```

如果要重建单个有问题的es或kibana容器，参考下面的命令：

```shell
# 由于其他容器都依赖setup容器，所以setup容器也得一起删除再重建
docker stop <container-name/id>
docker volume rm <data-volume-used-by-container> # 注意不要删除setup使用的certs卷
docker-compose up -d --no-recreate
```

当所有容器处于Running状态大约十多秒后，我们就可以查询集群状态：

```shell
# 将123456换成你的es密码
docker exec -it test-es-es01-1 curl \
  -s --cacert config/certs/ca/ca.crt \
  -u elastic:123456 \
  https://localhost:9200/_cluster/health?pretty=true
```

浏览器访问kibana：https://localhost:5601，用户名是`elastic`，密码是`.env`文件中的`ELASTIC_PASSWORD`。
注意，首次访问kibana页面时，浏览器会提示不安全的站点/链接，这是正常的。因为kibana使用的证书的内部CA签名的。具体请查看docker-compose.yml中的注释。

> 注意，YML文件的`kibana_system`用户是专门用于 Kibana 内部运作的 Elasticsearch 用户，不是给我们使用的。
> ES为它分配了内置的`kibana_system`角色，该角色不能进行UI登录。

最后请注意，你不能使用除了`localhost`或`127.0.0.1`以外的HOST来访问es或kibana，这是由他们的证书SAN决定的。
如果需要，你就得修改`setup`容器的command中生成`instances.yml`的部分，然后重建所有容器。

登录成功后，你可以在[这个页面](https://localhost:5601/app/management/security/users)进行用户/角色管理。

此小节参考官网[docker安装es集群指导](https://www.elastic.co/guide/en/elasticsearch/reference/current/docker.html#docker-compose-file)。

## 3. 核心概念

在 Elasticsearch（ES）中，有两个核心概念：索引（Index）和文档（Document）。理解这两个概念对于有效使用 Elasticsearch 非常重要。

### 3.1 索引（Index）

**概念**  
索引是 Elasticsearch 存储、索引和搜索的基本单元。它类似于数据库中的表，但是 Elasticsearch
中的索引更加灵活，它可以包含多种类型的文档，并且可以跨越多个物理分片。

> 在英文中还有一个indexing（翻译过来也是索引），但这个指的是对文档字段进行索引（indexing）的过程，而不是index（文档集合）本身。
> 可以为文档中的多个字段进行indexing。但和关系型数据库一样，indexing会占用更多磁盘以及内存空间。
>
> 在没有特别说明时，索引一般指文档集合。

**用途**  
索引用于组织和存储具有相似结构的文档。每个文档都属于一个索引，而索引本身是一个包含有关这些文档的信息的逻辑容器。

### 3.2 文档（Document）

**概念**  
文档是 Elasticsearch 中的基本信息单元，它类似于数据库中的一行。每个文档是一条具有结构化或非结构化数据的记录，通常是 JSON
格式。一个document可以包含多个字段（field），每个字段都有名称和值，它是一种JSON表现形式：`{"key1": "value1"...}`。

**用途**  
文档是实际存储在 Elasticsearch 中的数据。它们包含了有关实际信息的字段和值，可以是文本、数字、日期等。

### 3.3 字段（Field）

字段就是文档中的一个属性，它具有名称和值。字段是文档的基本组成部分，每个文档可以包含多个字段，每个字段都有不同的数据。

### 3.4 类型（Type）

索引是文档集合，文档是字段的集合，而每个字段都有一个类型。比如Text、Keyword、Numeric、Date、IP和Geopoint等。

- [所有字段类型][field_types]

在未预先设置时，新插入的文档的每个字段会自动分配一个合适的类型。不同类型支持的搜索模式不同，比如Text支持全文搜索，而Keyword仅支持完全匹配。
所以我们必须为每个字段指定合适的类型（当自动分配的类型不合适时），以便在搜索时使用对应的搜索模式。

此外，为了帮助ES在插入新的文档时能够更好的为每个字段建立索引，我们需要创建（下一小节的）映射规则来使得某些字段必须转化为ES内的某种类型，这样才能支持那些字段的搜索模式。
比如对text类型字段进行模糊搜索、对keyword字段进行精确搜索、对数字字段进行聚合搜索（sort/lt/gt...）。

### 3.5 映射（Mapping）

[映射][Mapping]是 Elasticsearch 中用于定义文档和字段的结构化方式，类似关系型DB中的表结构。它定义了字段的类型、索引选项等。
每个索引都有且仅有一个映射。

每个index都会有一个默认的映射，这个映射会包含index中的所有字段，并且每个字段都会自动分配一个合适的类型。
如果要修改/定义索引的映射，ES支持两种映射方式：

- 显式映射：手动为每个字段指定类型
- 动态映射（默认启用）：使用动态模板自定义映射，在文档插入时根据模板定义使用不同的类型，更灵活
    - 默认情况，ES为文档中的字段设置与其值类型相似的类型（如日期、文本和数字等），以便使用对应的搜索模式
    - 为了防止索引（indexing）过多字段导致内存使用爆炸，一般会设置映射限制设置

### 3.6 分片和副本（Shard and Replica）

- 分片（Shard）：创建索引时可以指定分片数量，每个分片也表现为一个独立的索引，多个分片可以并行处理搜索请求
    - 一个节点可以存放一个索引的多个分片
    - 合理的分片数量可以提高ES性能
- 副本（Replica）：一个分片可以有多个副本，以防止数据丢失和丢失后服务不可用
    - 一个节点最多只能存放一个索引的一个副本

### 3.7 小结

简而言之，索引是一种组织和存储文档的逻辑结构，而文档则是实际包含数据的记录。一个索引可以包含多个文档，每个文档都有自己的字段和值。在
Elasticsearch 中，你可以使用 RESTful API 或者客户端库来创建索引、添加、更新和检索文档，以及执行各种类型的查询。

当默认的映射类型不满足需求时，你可以自定义映射，以满足你的查询和搜索需求。

## 4. 快速上手

下面通过Kibana中的控制台来演示ES中document的增删改查。

### 4.1 添加document

HTTP请求：

```shell
# customer是index，1是document-id
# index不存在会自动创建，document-id存在会覆盖
# -- 在请求中可以不包含document-id，这样交给ES自动创建
POST /customer/_doc/1
{
  "firstname": "Jennifer",
  "lastname": "Walters"
}
```

POST可以改为PUT，语义一致。

HTTP响应：

```json
{
  "_index": "customer",
  "_id": "1",
  "_version": 1,
  "result": "created",
  "_shards": {
    "total": 2,
    "successful": 1,
    "failed": 0
  },
  "_seq_no": 0,
  "_primary_term": 1
}
```

返回的`_id`是文档的唯一主键，当POST不提供document-id时，ES会自动生成一个index中的唯一字符串作为`_id`。

### 4.2 查询document

HTTP请求：

```shell
GET /customer/_doc/1
```

HTTP响应：

```json
{
  "_index": "customer",
  "_id": "1",
  "_version": 2,
  "_seq_no": 1,
  "_primary_term": 1,
  "found": true,
  "_source": {
    "firstname": "Jennifer",
    "lastname": "Walters"
  }
}
```

响应中的`found`字段表示是否查询到目标文档，如有，则存在`_source`字段，包含目标文档的完整内容。

### 4.3 批量添加document

HTTP请求：

```shell
PUT customer/_bulk
{ "create": { } }
{ "firstname": "Monica","lastname":"Rambeau"}
{ "create": { } }
{ "firstname": "Carol","lastname":"Danvers"}
{ "create": { } }
{ "firstname": "Wanda","lastname":"Maximoff"}
{ "create": { } }
{ "firstname": "Jennifer","lastname":"Takeda"}
```

HTTP响应：

```json
{
  "errors": false,
  "took": 3,
  "items": [
    {
      "create": {
        "_index": "customer",
        "_id": "ZR4O2YwBNp0JCdUx-pck",
        "_version": 1,
        "result": "created",
        "_shards": {
          "total": 2,
          "successful": 1,
          "failed": 0
        },
        "_seq_no": 3,
        "_primary_term": 1,
        "status": 201
      }
    },
    {
      "create": {
        "_index": "customer",
        "_id": "Zh4O2YwBNp0JCdUx-pck",
        "_version": 1,
        "result": "created",
        "_shards": {
          "total": 2,
          "successful": 1,
          "failed": 0
        },
        "_seq_no": 4,
        "_primary_term": 1,
        "status": 201
      }
    },
    {
      "create": {
        "_index": "customer",
        "_id": "Zx4O2YwBNp0JCdUx-pck",
        "_version": 1,
        "result": "created",
        "_shards": {
          "total": 2,
          "successful": 1,
          "failed": 0
        },
        "_seq_no": 5,
        "_primary_term": 1,
        "status": 201
      }
    },
    {
      "create": {
        "_index": "customer",
        "_id": "aB4O2YwBNp0JCdUx-pck",
        "_version": 1,
        "result": "created",
        "_shards": {
          "total": 2,
          "successful": 1,
          "failed": 0
        },
        "_seq_no": 6,
        "_primary_term": 1,
        "status": 201
      }
    }
  ]
}
```

### 4.4 搜索document

搜索是最常用的功能。HTTP请求如下：

```shell
# match表示这是一个模糊搜索
GET customer/_search
{
  "query" : {
    "match" : { "firstname": "Jennifer" }
  }
}
```

HTTP响应：

```json
{
  "took": 1,
  "timed_out": false,
  "_shards": {
    "total": 1,
    "successful": 1,
    "skipped": 0,
    "failed": 0
  },
  "hits": {
    "total": {
      "value": 2,
      "relation": "eq"
    },
    "max_score": 0.87546873,
    "hits": [
      {
        "_index": "customer",
        "_id": "1",
        "_score": 0.87546873,
        "_source": {
          "firstname": "Jennifer",
          "lastname": "Walters"
        }
      },
      {
        "_index": "customer",
        "_id": "bR4a2YwBNp0JCdUxEZeh",
        "_score": 0.87546873,
        "_source": {
          "firstname": "Jennifer",
          "lastname": "Takeda"
        }
      }
    ]
  }
}
```

> 注意：若请求Body为空，则返回全部文档。

### 4.5 删除document

HTTP请求：

```shell
# 删除document-id对应的document
DELETE customer/_doc/1
```

HTTP响应：

```json
{
  "_index": "customer",
  "_id": "1",
  "_version": 2,
  "result": "deleted",
  "_shards": {
    "total": 2,
    "successful": 1,
    "failed": 0
  },
  "_seq_no": 1,
  "_primary_term": 1
}
```

删除整个index的请求：

```shell
DELETE customer
```

HTTP响应：

```json
{
  "acknowledged": true
}
```

### 4.6 查看index的汇总信息

HTTP请求：

```shell
# customer是index名
GET customer
```

HTTP响应：

```json
{
  "customer": {
    "aliases": {},
    "mappings": {
      "properties": {
        "firstname": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        },
        "lastname": {
          "type": "text",
          "fields": {
            "keyword": {
              "type": "keyword",
              "ignore_above": 256
            }
          }
        }
      }
    },
    "settings": {
      "index": {
        "routing": {
          "allocation": {
            "include": {
              "_tier_preference": "data_content"
            }
          }
        },
        "number_of_shards": "1",
        "provided_name": "customer",
        "creation_date": "1704449411558",
        "number_of_replicas": "1",
        "uuid": "SPAcuTKXSk63NIN_bJwY7A",
        "version": {
          "created": "8500003"
        }
      }
    }
  }
}
```

其中：

- `mappings` 定义了索引的映射，即字段的数据类型和属性。
- `firstname` 和 `lastname` 是两个doc字段。
    - 对于每个字段，type 定义了数据类型。在这里，它们的类型都是 text，表示这些字段是用于全文搜索的。
    - 对于每个字段，ES还默认为其创建了`{field-name}.keyword`的子字段，其类型为`keyword`，用于不分词的精确搜索
    - 这里提到了text和keyword两种ES字段类型，不同类型支持不同的搜索模式
- `settings` 包含了索引的设置信息。
    - `number_of_shards`: 1 表示索引只有一个主分片（分片数量基本等同于数据节点数量）。
    - `number_of_replicas`: 1 表示索引有一个副本（同上）。
    - `provided_name`: customer 是索引的名称。
    - `uuid` 是索引的唯一标识符。
    - `version` 包含有关索引创建版本的信息。

### 4.7 运维使用的API

#### 4.7.1 非JSON响应

这些API返回非JSON格式的响应，主要是查询集群的一些基础信息，仅在监控或维护时使用。

查看集群中的索引列表：

```shell
GET /_cat/indices # 非JSON响应，其中包含索引状态、磁盘占用、分片数量、文档数量（包括删除的）、主分片大小、总分片大小等信息
```

查看集群健康：

```shell
GET /_cat/health # 非JSON响应

# 返回示例
1704524822 07:07:02 docker-cluster yellow 1 1 36 36 0 0 2 0 - 94.7%
```

这个API返回第四列是一个颜色字段，表示集群的健康状态：

- 绿色：完全健康，所有主分片和副本分片都可用
- 黄色：所有主分片可用，但部分副本分片不可用
- 红色：部分主分片不可用，但部分数据仍可查到

在后面的一些API响应中你还会看到这个颜色字段，它们代表相同的含义。

查看集群主节点信息：

```shell
GET /_cat/master # 非JSON响应
```

查看集群所有节点的统计信息：

```shell
GET /_cat/nodes # 非JSON响应
```

查看分片信息：

```shell
GET /_cat/shards # 非JSON响应
```

#### 4.7.2 JSON响应

查看集群健康：

```shell
GET /_cluster/health?pretty=true
```

在单节点集群中，你可能会看到返回的颜色是yellow，这是正常的，因为默认索引的分片和副本数量都是1（通过`GET {index_name}`
查询），显然还需要一个节点来容纳副本分片的存在。这就验证了上面对yellow状态的解释。我们可以通过下面两个API来进一步定位问题：

```shell
# 查看所有分片的状态，返回结果中的
# -- 第三列表示主分片（p）或副本分片（r）
# -- 第四列表示分片状态，STARTED是正常，UNASSIGNED是未分配
# -- 第五列是已分配的节点
# -- 第六列是未分配的原因
# 你会看到一行：你新增的索引的副本分片是未分配状态
GET _cat/shards?v=true&h=index,shard,prirep,state,node,unassigned.reason&s=state

# 查看分配有问题的索引，并返回具体原因
GET _cluster/allocation/explain?filter_path=index,node_allocation_decisions.node_name,node_allocation_decisions.deciders.*
```

既然得知了副本分片未分配的原因，那么我们就可以通过下面的API来解决：

```shell
# 将索引的副本分片数量设置为0，这样你的index就只有一个主分片位于主节点
PUT {index-name}/_settings
{
    "index": {
        "number_of_replicas" : 0
    }
}
```

注意，这里只是介绍如何定位集群健康处于yellow状态的原因以及如何解决的过程。实际情况中，你可能需要对集群进行扩容，以满足索引分片的需求。

查看集群最全面的统计信息（响应是一个大JSON）：

```shell
GET /_cluster/state?pretty
```

查看集群节点监控：

```shell
GET /_nodes/state?pretty
```

### 4.8 操作映射

下面是一些常见的操作映射的HTTP请求（以`customer`为索引示例）：

```shell
# 查询索引customer的映射
GET customer/_mapping

######
# 注意：映射只能在创建索引前设置，否则会得到index xxx already exists的提示。
# -- 也就是说，你必须预先为你的索引创建一个完善的映射配置，无论是动态映射还是显式映射。
######

# 设置动态映射中的日期格式
PUT customer
{
  "mappings": {
    "dynamic_date_formats": ["yyyy/MM/dd"]
  }
}

# 禁用日期识别（默认开启）
PUT customer
{
  "mappings": {
    "date_detection": false
  }
}

# 禁用动态映射（默认开启）。dynamic的其他参数是true/runtime/strict
PUT customer
{
  "mappings": {
    "dynamic": false
  }
}

# 启用数字识别（默认关闭），启用后可以识别字符串形式的整数/浮点数，并为其创建numeric索引以支持聚合搜索
PUT customer
{
  "mappings": {
    "numeric_detection": true
  }
}

# 配置一个简单的动态映射模板
# - 帮助更好的创建字段索引（indexing）
# - 这个模板包含t1,t2两个规则。
#   - t1表示将JSON类型为long的值映射为integer类型
#   - t2表示将JSON类型为string的值映射为text类型并创建一个子类型keyword
PUT customer
{
  "mappings":{
    "dynamic_templates": [
      {
        "t1": {
          "match_mapping_type": "long",
          "mapping":{
            "type":"integer"
          }
        }
      },
      {
        "t2": {
          "match_mapping_type": "string",
          "mapping":{
            "type":"text",
            "fields":{
              "raw": {
                "type": "keyword",
                "ignore_above": 256
              }
            }
          }
        }
      }
    ]
  }
}

# 配置动态映射模板：将字符串都映射为text类型，表示此索引的所有字符串都是text类型，不需要精确搜索
PUT customer
{
	"mappings": {
		"dynamic_templates": [{
			"string_to_text": {
				"match_mapping_type": "string",
				"mapping": {
					"type": "text"
				}
			}
		}]
	}
}

# 配置动态映射模板：关闭评分（norms:false）,可以节省一定存储空间
PUT customer
{
	"mappings": {
		"dynamic_templates": [{
			"string_to_keyword": {
				"match_mapping_type": "string",
				"mapping": {
					"type": "text",
					"norms": false,
					"fields": {
						"keyword": {
							"type": "keyword",
							"ignore_above": 256
						}
					}
				}
			}
		}]
	}
}

# 配置动态映射模板：禁止为数字类型创建索引（indexing）。在不需要对数字进行聚合搜索时使用。
PUT customer
{
	"mappings": {
		"dynamic_templates": [{
			"not_indexing_long": {
				"match_mapping_type": "long",
				"mapping": {
					"type": "long",
					"index": false
				}
			}
		},
		{
			"not_indexing_double": {
				"match_mapping_type": "double",
				"mapping": {
					"type": "float",
					"index": false
				}
			}
		}]
	}
}

# 配置动态映射模板：根据字段名称映射
# - 其中的doc_values是一种支持对字段值进行聚合以及排序的设置（通过列式存储的方式支持）
#   - 除了text以外的大部分字段类型都默认开启doc_values
PUT customer
{
	"mappings": {
		"dynamic_templates": [{
			"fuzzy_match": {
				"match_mapping_type": "string",
				"match": "long_*",
				"unmatch":"*_text",
				"mapping": {
					"type": "long"
				}
			}
		},
		{
			"regex_match": {
				"match_pattern": "regex",
				"match": "^profit_\\d+$",
				"mapping": {
					"type": "keyword",
					"index": false,
					"norms": false,
					"doc_values": false
				}
			}
		}]
	}
}

# 配置显式映射
# - 下面规则是为name、age等四个命名字段配置各自的类型规则
PUT customer
{
  "mappings":{
    "properties": {
      "name": {
        "type": "text"
      },
      "age": {
        "type": "long"
      },
      "ctime": {
        "type": "date",
        "format": "yyyy-MM-dd HH:mm:ss"
      },
      "describe": {
        "type": "keyword",
        "norms": false,
        "doc_values": false
      }
    }
  }
}
```

## 5. 未完待续

TODO

[ES]: https://www.elastic.co/guide/en/elasticsearch/reference/current/index.html

[Kibana]: https://www.elastic.co/guide/en/kibana/current/index.html

[Logstash]: https://www.elastic.co/guide/en/logstash/current/introduction.html

[Beats]: https://www.elastic.co/guide/en/beats/libbeat/current/beats-reference.html

[Client SDKs]: https://www.elastic.co/guide/en/elasticsearch/client/index.html

[ClusterupdatesettingsAPI]: https://www.elastic.co/guide/en/elasticsearch/reference/current/cluster-update-settings.html

[Mapping]: https://www.elastic.co/guide/en/elasticsearch/reference/8.11/mapping.html

[field_types]: https://www.elastic.co/guide/en/elasticsearch/reference/8.11/mapping-types.html