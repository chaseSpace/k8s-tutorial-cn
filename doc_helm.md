## Helm手记

### 1. 创建Chart

```shell
helm create example-chart

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

`_helpers.tpl`与其他模板文件不同，它可以被除了自己和`Chart.yaml`以外的所有模板文件引用。一般用来定义通用信息，比如某项命名/标签等。

这个文件的语法也很简单，主要使用Helm
模板引擎的[各种函数](https://helm.sh/zh/docs/chart_template_guide/functions_and_pipelines/)。

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

```yaml
# 默认是一个Pod，测试对Service的访问连通性
apiVersion: v1
kind: Pod
metadata:
  name: "{{ include "example-chart.fullname" . }}-test-connection"
  labels:
    { { - include "example-chart.labels" . | nindent 4 } }
  annotations:
    "helm.sh/hook": test
spec:
  containers:
    - name: wget
      image: busybox
      command: [ 'wget' ]
      args: [ '{{ include "example-chart.fullname" . }}:{{ .Values.service.port }}' ]
  restartPolicy: Never
```

#### 1.4 解释 values.yaml

这是最主要的配置文件，用于定义应用部署的各项参数。比如Pod副本数量，镜像名称等。

通过查看`values.yaml`可以知道，默认的配置是一个使用Nginx镜像的Deployment控制器，副本数量为1。并且基于Deployment控制器创建了一个Service，
类型ClusterIP，监听80端口；此外还创建了Pod专属的serviceAccount。Ingress和Hpa配置项默认未启用。

### 2. 验证Chart

发布前需要对Chart配置进行验证：

```shell
$ helm lint example-chart
==> Linting example-chart
[INFO] Chart.yaml: icon is recommended

1 chart(s) linted, 0 chart(s) failed
```

在最终执行`helm install`
进行部署时，会将Chart文件解析为K8s能够识别的各种对象模板以进行部署。可使用`helm install --dry-run --debug [Chart目录位置]`
来检查Chart生成的k8s对象模板。

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

当Chart编写和验证完成后，你如果由分发给给其他用户使用的需求（像分享镜像那样），则可以打包Chart到仓库中。

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

# 然后更新发布
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
参数就无效了，会直接使用当前目录下的Chart配置进行升级。无论你是否修改了Chart配置，Helm都会为发布增加`REVISION`号。
当然，实际的K8s对象如Deployment只会在模板变化时重新部署Pod。

实际环境中，我们通常会使用`-f values.yaml`参数来指定配置文件（或使用`--set`指定某个配置参数）进行升级。使用`helm upgrade -h`
查看全部参数。

> 例如:  
> helm upgrade helm-nginx example-chart -f example-chart/values.yaml --set "serviceType=NodePort"

其中`--set`可以指定多个键值对参数（只用于替换`values.yaml`中的配置），使用`helm show values example-chart`
查看Chart的`values.yaml`配置。
此外，它还有一些细节上的规范（比如如何设置值为数组的字段），
但笔者没有找到相关官方文档，可以参考网络上的一些文章：

- [“set”参数的高级用法（英）](https://itnext.io/helm-chart-install-advanced-usage-of-the-set-argument-3e214b69c87a)
- [helm --set的使用示例及基本使用命令整理](https://blog.csdn.net/a772304419/article/details/125915827)

#### 5.2 回滚

查看helm发布的记录:

```shell
$ helm history helm-nginx   
REVISION	UPDATED                 	STATUS    	CHART              	APP VERSION	DESCRIPTION     
1       	Mon Dec  4 20:23:48 2023	superseded	example-chart-0.1.0	1.16.0     	Install complete
2       	Mon Dec  4 20:57:14 2023	superseded	example-chart-0.1.0	1.16.1     	Upgrade complete
```

回滚：

```shell
# 1是REVISION
$ helm rollback helm-nginx 1           
Rollback was a success! Happy Helming!
```

注意，Helm默认最多保留10条发布记录，也就是说，当REVISION为11的时候（只有2~11），1就被删除了，也不能回滚到1了。

#### 5.3 删除

新版本中，`helm delete RELEASE-NAME`命令已经不再保留发布记录了，而是彻底删除发布涉及的所有K8s对象和Helm中的记录。

`delete`可以使用关键字`uninstall/del/un`进行等价替换。

### 推荐的文章

- [Helm template快速入门_掘金](https://juejin.cn/post/6844904199818313735)