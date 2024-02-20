## Helm手记

Helm是一个用于Kubernetes应用的包管理工具。它允许你定义、安装和升级Kubernetes应用。 Helm
使用称为“Charts”的打包格式，每个Chart都包含用于部署一个具体应用程序的相关文件。

### 1. 创建Chart

```shell
$ helm create example-chart

$ tree example-chart                                 
example-chart
├── charts  # 初始为空目录，存放本 Chart 依赖的其他 Charts
├── Chart.yaml # 记录这个Chart的元数据，如名称/描述/版本等
├── templates # 主要。存放k8s部署文件的helm模板，不完全等于k8s模板，扩展go template语法
│   ├── deployment.yaml  # 用于定义 Kubernetes Deployment 对象，描述如何部署你的应用程序。
│   ├── _helpers.tpl # 包含了一些 Helm 模板引擎的辅助函数，可以在其他所有模板文件中使用。
│   ├── hpa.yaml # 用于定义 Horizontal Pod Autoscaler 对象，允许根据 CPU 使用率或其他指标动态调整 Pod 的数量。
│   ├── ingress.yaml # 用于定义 K8s Ingress 对象
│   ├── NOTES.txt #  当执行 helm install 时，Helm 将在安装完成后显示这个文件中的注释。
│   ├── serviceaccount.yaml # 用于定义 Kubernetes ServiceAccount 对象，用于为 Pod 中的进程提供身份验证信息。
│   ├── service.yaml # 用于定义 Kubernetes Service 对象，用于将流量路由到你的 Pod
│   └── tests # 包含用于测试 Chart 的测试文件
│       └── test-connection.yaml
└── values.yaml # 该文件包含了 Helm Chart 的默认值，这些值将用于渲染模板文件; 用户可以通过传递自定义的 values.yaml 文件或通过命令行选项来覆盖这些默认值
```

其中`deployment.yaml`和`service.yaml`
是必须要使用的（即需要修改），其他K8s对象模板文件都是用到时才会改动，包含`hpa.yaml`, `ingress.yaml`, `serviceaccount.yaml`，
在这几个文件的首行包含`if .Values.*.enabled`字样表示动态启用，需要在`values.yaml`文件中的`enabled`字段为`true`时才会启用。

#### 1.1 解释 deployment.yaml

```
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "example-chart.fullname" . }}
  labels:
    {{- include "example-chart.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "example-chart.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      {{- with .Values.podAnnotations }}
      annotations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
...
```

其中`{{ ... }}`是Go Template语法。大括号中以

- `.Values`开头的属性值是在`values.yaml`中定义的
- 其他属性是在`Chart.yaml`中定义的
- `.Release`开头的是在发布版本时确定

通过Go Template，可以使模板的具体部署操作和部署参数分离开来，各自单独维护。最关键的是可以多个对象复用同一套Chart模板。

#### 1.2 解释 _helpers.tpl

`_helpers.tpl`与其他模板文件不同，它可以被除了`Chart.yaml`以外的所有模板文件（包括自己）引用。
一般用来定义生成逻辑稍微复杂的变量，比如某项命名/标签等。

> 一般我们可以直接将变量的生成逻辑写入K8s YAML文件中，但这样会使得它们变得臃肿而降低模板可读性，所以会用到`_helpers.tpl`。

这个文件的语法也很简单，主要使用Helm
模板引擎的[各种函数](https://helm.sh/zh/docs/chart_template_guide/functions_and_pipelines/)来组合成具体的逻辑。

```yaml
# 定义一个变量 example-chart.name
# 其值的生成逻辑是：优先取 .Chart.Name，若为空则取.Values.nameOverride
  # 然后，|类似管道符号，继续将值调用trunc函数确保字符长度不超过63，最后去掉后缀-
  { { - define "example-chart.name" - } }
  { { - default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" } }
  { { - end } }
```

#### 1.3 解释 tests 目录

默认这个目录下有个`test-connection.yaml`文件，用于定义【部署完成后需要执行的测试内容】，以便验证应用是否成功部署。

执行`helm test <RELEASE_NAME>`来运行测试，以便验证部署的Helm资源是否正常运行。下面是一个例子：

```yaml
# 默认是一个Pod，测试对Service的访问连通性
apiVersion: v1
kind: Pod
metadata:
  name: "{{ include "example-chart.fullname" . }}-test-connection"
  labels:
    { { - include "example-chart.labels" . | nindent 4 } }
  annotations:
    "helm.sh/hook": test # 测试资源都有这个注解，它是helm的一个钩子
spec:
  containers:
    - name: wget
      image: busybox
      command: [ 'wget' ]
      args: [ '{{ include "example-chart.fullname" . }}:{{ .Values.service.port }}' ]
  restartPolicy: Never
```

注意，执行测试的Pod资源在测试完成后应该以(exit 0)成功退出，所以注意`command`部分的编写。

在Helm v3中，支持使用以下测试钩子（`helm.sh/hook`）之一：

- test-failure：这是一个针对【失败】情况的测试用例
- test-success：这是一个针对【成功】情况的测试用例（等同于旧版的`test`）
- test（向后兼容，等同于`test-success`）

#### 1.4 解释 values.yaml

这是最主要的配置文件，用于定义应用部署的各项参数。比如Pod副本数量，镜像名称等。

通过查看`values.yaml`可以知道，默认的配置是一个使用Nginx镜像的Deployment控制器，副本数量为1。并且基于Deployment控制器创建了一个Service，
类型ClusterIP，监听80端口；此外还创建了Pod专属的serviceAccount。Ingress和Hpa配置项默认未启用。

### 2. 验证Chart

发布前需要对Chart配置格式进行验证：

```shell
$ helm lint example-chart
==> Linting example-chart
[INFO] Chart.yaml: icon is recommended

1 chart(s) linted, 0 chart(s) failed
```

在最终执行`helm install`
进行部署时，会将Chart文件解析为K8s能够识别的各种对象模板以进行部署。
可使用`helm install --dry-run --debug [Chart目录位置]`来提前检查Chart生成的k8s对象模板是否正确。

```shell
# 其中helm-nginx是发布名称，最后才是chart目录作为参数
helm install --dry-run --debug helm-nginx example-chart
...输出计算后的各模板内容
```

新版的Helm必须提供发布名称参数，或者提供`--generate-name`标志使用自动生成的名称。

### 3. 发布和查看

```shell
$ helm install  helm-nginx example-chart
NAME: helm-nginx
LAST DEPLOYED: Mon Dec  4 20:23:48 2023
NAMESPACE: default
STATUS: deployed
REVISION: 1
NOTES:
...
```

查看部署：

```shell
$ helm ls
NAME      	NAMESPACE	REVISION	UPDATED                                	STATUS  	CHART              	APP VERSION
helm-nginx	default  	1       	2023-12-04 20:23:48.653998103 +0800 CST	deployed	example-chart-0.1.0	1.16.0

# status可以查看最后部署的时间，namespace，状态，递增版本号
$ helm status helm-nginx       
NAME: helm-nginx
LAST DEPLOYED: Mon Dec  4 20:23:48 2023
NAMESPACE: default
STATUS: deployed
REVISION: 1
NOTES:
...

# --show-resources 列出Chart部署的资源
$ helm status helm-nginx --show-resources
NAME: helm-nginx
LAST DEPLOYED: Mon Dec  4 20:23:48 2023
NAMESPACE: default
STATUS: deployed
REVISION: 1
RESOURCES:
==> v1/ServiceAccount
NAME                       SECRETS   AGE
helm-nginx-example-chart   0         6m5s

==> v1/Service
NAME                       TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
helm-nginx-example-chart   ClusterIP   20.1.80.84   <none>        80/TCP    6m5s

==> v1/Deployment
NAME                       READY   UP-TO-DATE   AVAILABLE   AGE
helm-nginx-example-chart   1/1     1            1           6m5s

==> v1/Pod(related)
NAME                                        READY   STATUS    RESTARTS   AGE
helm-nginx-example-chart-5b5b69cb9d-nnrpn   1/1     Running   0          6m5s
```

删除部署（无法回滚）：

```shell
helm uninstall helm-nginx
```

### 4. 打包Chart

当Chart编写和验证完成后，你如果有分发给给其他用户使用的需求（像分享镜像那样）或者需要版本化Chart包，则可以打包Chart到仓库中。

```shell
$ helm package example-chart 
Successfully packaged chart and saved it to: /mnt/hgfs/go_dev/k8s-tutorial-cn/helm/example-chart-0.1.0.tgz
```

**升级Chart**

升级表示要对Chart配置进行大或小的修改，并且更新`Chart.yaml`的版本号。在其中会有两个意义不同的版本号：

```yaml
# 打包时增加的版本号
version: 0.1.0

# 发布时增加的版本号
appVersion: "1.16.0"
```

其中`version`是`helm search xxx`输出的Chart Version。`helm search xxx --versions`会输出每个Chart的所有历史版本。

### 5. 发布的升级、回滚和删除

#### 5.1 升级

刚才我们已经发布了`example-chart`，命名为`helm-nginx`，其`Chart.yaml`中的`appVersion`为`1.16.0`。现在我们修改`appVersion`
为`1.16.1`来模拟升级所做的修改
，然后更新发布。

```shell
# 首先修改Chart.yaml中的appVersion为 1.16.1

# 然后更新发布，--description 增加发布说明
$ helm upgrade helm-nginx example-chart
Release "helm-nginx" has been upgraded. Happy Helming!
NAME: helm-nginx
LAST DEPLOYED: Mon Dec  4 20:57:14 2023
NAMESPACE: default
STATUS: deployed
REVISION: 2
...

# APP VERSION 已更新
$ helm ls                              
NAME      	NAMESPACE	REVISION	UPDATED                                	STATUS  	CHART              	APP VERSION
helm-nginx	default  	2       	2023-12-04 20:57:14.959204562 +0800 CST	deployed	example-chart-0.1.0	1.16.1   

$ helm upgrade helm-nginx example-chart
Release "helm-nginx" has been upgraded. Happy Helming!
NAME: helm-nginx
LAST DEPLOYED: Mon Dec  4 20:57:14 2023
NAMESPACE: default
STATUS: deployed
REVISION: 2
...
```

如果最后的Chart参数是引用某个仓库中的Chart（引用形式为`repo/chart_name`
），此时可以使用`helm upgrade helm-nginx example-chart --version x.x.x`来指定Chart版本进行升级。

如果是本地的Chart目录，那`--version`
参数就无效了，会直接使用所引用目录下的Chart配置进行升级。**无论你是否修改了Chart中的任何一个文件**
，Helm都会为发布增加`REVISION`号。
当然，实际的K8s对象如Deployment只会在模板变化时重新部署Pod。

实际环境中，我们通常会使用`-f values.yaml`参数来指定配置文件（或使用`--set`指定某个配置参数）进行升级。使用`helm upgrade -h`
查看全部参数。

> 例如:  
> helm upgrade helm-nginx example-chart -f example-chart/values.yaml --set "serviceType=NodePort"

其中`--set`可以指定多个键值对参数（只用于替换`values.yaml`中的配置），使用`helm show values example-chart`
查看Chart的`values.yaml`配置。
此外，它还有一些细节上的规范（比如如何设置值为数组的字段），可以参考以下文档：

- [安装前自定义chart（官方文档）](https://helm.sh/zh/docs/intro/using_helm/#安装前自定义-chart)
- [“set”参数的高级用法（英）](https://itnext.io/helm-chart-install-advanced-usage-of-the-set-argument-3e214b69c87a)

> 最后，说一点笔者的个人建议。在实际的项目开发中，建议只需要在每个服务目录下保留`values.yaml`
> 即可，而不需要保留`Chart.yaml`来定义其APP VERSION，
> 因为这样就免去了在每个服务目录下维护两个helm配置文件的麻烦。在发布时我们只需要使用`--description`来简述
> 本次发布的具体内容即可，并可以直接将镜像tag作为发布说明。这样也可以为回滚提供帮助。
>
> Helm不支持在Upgrade时设置`appVersion`，这是难以理解的。在 [ #3555](https://github.com/helm/helm/issues/3555) 这个讨论时间长达三年的
> Issue中，官方最终也没有支持这种方式，而是推荐使用`helm package --app-version`的方式来设置`appVersion`
> ，但打包就需要部署Helm仓库，增加了运维成本。
> 社区中的另一种非常规做法则是在更新发布前使用`sed`命令修改了`Chart.yaml`中的`appVersion`。

#### 5.2 回滚

查看helm发布的记录:

```shell
# 在upgrade时使用--description设置的说明会覆盖这里的 DESCRIPTION
$ helm history helm-nginx   
REVISION	UPDATED                 	STATUS    	CHART              	APP VERSION	DESCRIPTION     
1       	Mon Dec  4 20:23:48 2023	superseded	example-chart-0.1.0	1.16.0     	Install complete
2       	Mon Dec  4 20:57:14 2023	superseded	example-chart-0.1.0	1.16.1     	Upgrade complete
```

注意：`REVISION`是永远递增的。

回滚到指定`REVISION`：

```shell
# 1是REVISION，不指定就默认上个REVISION
$ helm rollback helm-nginx 1           
Rollback was a success! Happy Helming!
```

注意，Helm默认最多保留10条发布记录，也就是说，当`REVISION`为11的时候（只能看到2~11的记录），1就被删除了，也不能回滚到1了。

#### 5.3 删除

新版本中，`helm delete RELEASE-NAME`命令已经不再保留发布记录了，而是彻底删除发布涉及的所有K8s对象和Helm中的记录。

`delete`可以使用关键字`uninstall/del/un`进行等价替换。

### 6. 钩子

Helm提供了钩子（Hook）功能，允许在Helm资源的安装前/后、删除前/后等特定时机执行特定的操作。
一般使用钩子来执行以下任务：

- 升级之前检查环境是否具备升级的条件
- 安装前先创建一些基础资源，如ConfigMap、Secret等
- 删除后执行一些清理工作

所有Helm钩子：

| Hook          | 作用                               |
|---------------|----------------------------------|
| pre-install   | 在渲染模板之后，在 Kubernetes 中创建任何资源之前执行 |
| post-install  | 将所有资源加载到 Kubernetes 之后执行         |
| pre-delete    | 在从 Kubernetes 删除任何资源之前对删除请求执行    |
| post-delete   | 删除所有发行版资源后，对删除请求执行               |
| pre-upgrade   | 在呈现模板之后但在更新任何资源之前，对升级请求执行        |
| post-upgrade  | 升级所有资源后执行升级                      |
| pre-rollback  | 在呈现模板之后但在回滚任何资源之前，对回滚请求执行        |
| post-rollback | 修改所有资源后，对回滚请求执行                  |
| test          | 调用Helm test子命令时执行                |

在1.3节【解释tests目录】中我们已经看见过钩子是通过在Pod资源（其他k8s资源也可）中使用注解来使用的。下面是一个例子：

```yaml
# 通常在Pod和Job中使用
annotations:
  "helm.sh/hook": post-install,post-upgrade
```

所有钩子关联的资源都是串行阻塞加载的，当使用钩子的资源达到`Ready`状态时，
Helm会继续加载下一个钩子。如果一个资源加载失败，则不会继续加载后续的资源。

> 针对Pod和Job以外的资源，一旦K8s将资源标记为已加载(已添加或已更新)，资源会被认为是`Ready`。

此外，还可以定义：

- 钩子关联资源的权重，这决定了钩子资源加载顺序
- 钩子关联资源的删除策略，这决定了删除钩子资源的时机

示例如下：

```yaml
# 权重是字符串形式的数字，支持负数和正数，按照升序执行
"helm.sh/hook-weight": "-5"
# 删除策略
# before-hook-creation   新钩子启动前删除之前的资源 (默认)
# hook-succeeded	钩子成功执行之后删除资源
# hook-failed	如果钩子执行失败，删除资源
"helm.sh/hook-delete-policy": hook-succeeded
```

你可以通过 [Kibana-templates](helm/kibana/templates) 来进一步学习钩子的使用。

### 推荐的文章

- [Helm官方文档](https://helm.sh/zh/docs/)
- [Helm template快速入门_掘金](https://juejin.cn/post/6844904199818313735)