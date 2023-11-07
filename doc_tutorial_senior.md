# Kubernetes 进阶教程

为了方便阅读，建议点击网页右侧的 ![toc.jpg](img/toc.jpg) 按钮在右侧展开目录。

**环境准备**：

```
10.0.2.2 k8s-master  
10.0.2.3 k8s-node1
```

一些提高效率的设置：

1. [安装ohmyzsh](doc_install_ohmyzsh.md)
2. 设置kubectl的alias为`kk`，下文会用到。

## 1. 存储与配置

如果你的应用需要使用存储功能，那么你需要先了解存储卷（Volume）的概念。k8s定义了下面几类存储卷（volume）抽象来实现相应功能：

1. 本地存储卷：用于Pod内多个容器间的存储共享，或这Pod与节点之间的存储共享；
2. 网络存储卷：用于多个Pod之间甚至是跨节点的存储共享；
3. 持久存储卷：基于网络存储卷，用户无须关心存储卷的创建所使用的存储系统，只需要自定义具体消费的资源额度（将Pod与具体存储系统解耦）；

所有的卷映射到容器都是以**目录或文件**的形式存在。

此外，这一节还会提到StatefulSet控制器，它用来管理有状态应用程序的部署。有状态应用程序通常是需要唯一标识、稳定网络标识和有序扩展的应用程序，
例如数据库、消息队列和存储集群。StatefulSet 为这些应用程序提供了一种在 Kubernetes 集群中管理和维护的方法。

### 1.1 本地存储卷

本地存储卷（LocalVolume）是Pod内多个容器间的共享存储，Pod与节点之间的共享存储。它主要包括`emptyDir`和`hostPath`两种方式，
这两种方式都会直接使用节点上的存储资源，区别在于`emptyDir`的存储卷在Pod的生命周期内存在，而`hostPath`的存储卷由节点进行管理。

#### 1.1.1 emptyDir

emptyDir是一个纯净的空目录，它占用节点的一个临时目录，在Pod重启或重新调度时，这个目录的数据会丢失。Pod内的容器都可以读写这个目录（也可以对容器设置只读）。
一般用于短暂的临时数据存储，如缓存或临时文件。

[pod_volume_emptydir.yaml](pod_volume_emptydir.yaml) 定义了有两个容器（write和read）的Pod，并且都使用了emptyDir定义的卷，
现在应用它并查看Pod内read容器的日志：

```shell
$ kk apply -f pod_volume_emptydir.yaml
pod/busybox created
$ kk get pod                           
NAME      READY   STATUS    RESTARTS   AGE
busybox   2/2     Running   0          2m8s
$ kk logs busybox read
hellok8s!
```

注意模板中先定义了write容器，所以它先启动且写入了数据，然后再启动read容器以至于能够读到数据。

Pod使用的emptyDir具体的位置在节点上的 `/var/lib/kubelet/pods/<pod-uid>/volumes/kubernetes.io~empty-dir`目录下找到：

```shell
# 在master节点查看 pod uid
$ kk get pod busybox  -o jsonpath='{.metadata.uid}'
b6abc3f2-d9b3-4297-b636-e33f06d0278d

# 在node1查看具体位置
[root@k8s-node1 ~]# ls /var/lib/kubelet/pods/b6abc3f2-d9b3-4297-b636-e33f06d0278d/volumes/
kubernetes.io~empty-dir  kubernetes.io~projected
[root@k8s-node1 ~]# ls /var/lib/kubelet/pods/b6abc3f2-d9b3-4297-b636-e33f06d0278d/volumes/kubernetes.io~empty-dir/
temp-dir
```

**使用内存作为emptyDir**  
k8s允许我们在定义emptyDir时使用内存作为实际存储卷，以提高临时卷的读写速度，但需要注意容器对内存的占用需求，避免超限或占用过高影响节点上其他Pod。
按下面的方式定义：

```yaml
volumes:
  - name: cache-volume
    emptyDir:
      medium: Memory
```

#### 1.1.2 hostPath

hostPath是节点上的一个**文件或目录**，Pod内的容器都可以读写这个卷，这个目录的生命周期与**节点**相同。需要注意的是，
Pod调度到其他节点就无法读取到之前它自己写入的数据。

[pod_volume_hostpath.yaml](pod_volume_hostpath.yaml) 定义了包含一个write容器的Pod，
并且使用hostPath定义了volume，映射节点的`/home/host-temp-dir`目录，现在应用它并在node1上查看容器写入的数据：

```shell
# 必须提前在node1上创建目录（当hostPath.type为空时）
[root@k8s-node1 ~]# mkdir /home/host-temp-dir

# 在master上启动Pod
$ kk apply -f pod_volume_hostpath.yaml
$ kk get pod                            
NAME               READY   STATUS    RESTARTS   AGE
busybox            2/2     Running   0          26m
busybox-hostpath   1/1     Running   0          11s

# 在node1上查看数据
[root@k8s-node1 ~]# cat /home/host-temp-dir/data 
hellok8s!
```

hostPath卷比较适用于DaemonSet控制器，运行在DaemonSet控制器中的Pod会常驻在各个节点上，一般是日志或监控类应用。

另外，hostPath允许定义`type`属性，以指定映射行为：

| 类型                | 描述                                                                              |
|-------------------|---------------------------------------------------------------------------------|
| 空字符串（默认）          | 用于向后兼容，这意味着在安装 hostPath 卷之前不会执行任何检查。                                            |
| DirectoryOrCreate | 如果在给定路径上什么都不存在，那么将根据需要创建空目录，权限设置为 0755，具有与 kubelet 相同的组和属主信息。**它可以自动创建中间目录**    |
| Directory         | 在给定路径上必须存在的目录。                                                                  |
| FileOrCreate      | 如果在给定路径上什么都不存在，那么将在那里根据需要创建空文件，权限设置为 0644，具有与 kubelet 相同的组和所有权。**它要求中间目录必须存在**！ |
| File              | 在给定路径上必须存在的文件。                                                                  |
| Socket            | 在给定路径上必须存在的 UNIX 套接字。                                                           |
| CharDevice        | 在给定路径上必须存在的字符设备。                                                                |
| BlockDevice       | 在给定路径上必须存在的块设备。                                                                 |

当使用hostPath卷时要小心，因为：

- HostPath 卷可能会暴露特权系统凭据（例如 Kubelet）或特权 API（例如容器运行时套接字），可用于容器逃逸或攻击集群的其他部分。
- 具有相同配置（例如基于同一 PodTemplate 创建）的多个 Pod 会由于节点上文件的不同而在不同节点上有不同的行为。
- 下层主机上创建的文件或目录只能由 root 用户写入。 你需要在特权容器中以 root 身份运行进程，或者修改主机上的文件权限以便容器能够写入
  hostPath 卷。

k8s官方建议避免使用 HostPath，当必须使用 HostPath 卷时，它的范围应仅限于所需的文件或目录，最好以**只读方式**挂载。

### 1.2 网络存储卷

一个典型的例子是NFS，熟悉网络的读者应该知道，NFS是网络文件系统，可以实现跨主机的数据存储和共享，k8s支持多种网络存储卷，
这其中包含众多云服务商提供的存储方案，比如NFS/iSCSI/GlusterFS/RDB/azureDisk/flocker/cephfs等，最新的支持细节在 [这里](https://kubernetes.io/zh-cn/docs/concepts/storage/volumes)
查看。

网络存储卷属于第三方存储系统，所以其生命周期也是与第三方绑定，不受Pod生命周期影响。

大部分网络存储卷是集成各种第三方的存储系统，所以配置上各有差别，这里不会一一说明。[pod_volume_nfs.yaml](pod_volume_nfs.yaml)
是一个使用NFS卷的Pod模板示例，可供参考。
你还可以查看 [更多NFS示例](https://github.com/kubernetes/examples/tree/master/staging/volumes/nfs)。

### 1.3 持久存储卷

上节说到，网络存储卷是集成第三方存储系统，所以具体卷配置参数一般是对应存储管理人员才会熟悉，且这些都不应该是开发人员和集群管理者需要关心的，
所以k8s引入了持久存储卷概念，持久存储卷是集群级别的资源，由集群管理员创建，然后由集群用户去使用。

具体来说，k8s提供三种基于存储的抽象概念：

- PV（Persistent Volume）
- StorageClass
- PVC（Persistent Volume Claim）

这三者用于支持基础设施和应用程序之间的分离，以便于开发人员和存储管理人员各司其职，由存储管理人员设置PV或StorageClass，
并在里面配置存储系统和参数，然后开发人员只需要创建PVC来申请指定空间的资源以存储和共享数据即可，无需关心底层存储系统细节。
当删除PVC时，它写入具体存储资源的数据可以根据回收策略自动清理。

#### 1.3.1  使用PV和PVC

PV表示持久存储卷，定义了集群中可使用的存储资源，其中包含存储资源的类型、回收策略、存储容量等参数。

PVC表示持久存储卷声明，是用户发起对存储资源的申请，用户可以设置申请的存储空间大小、访问模式。

[pod_use_pvc.yaml](pod_use_pvc.yaml) 提供了一个Pod使用PVC的完整示例（也可以将其分离为多个单独模板），其中按顺序定义了PV和PVC以及使用PVC的Pod。
下面是测试情况：

```shell
$ kk apply -f pod_use_pvc.yaml
persistentvolume/pv-hostpath created
persistentvolumeclaim/pvc-hostpath created
pod/busybox-use-pvc configured

$ kk get pv,pvc,pod
NAME                           CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                  STORAGECLASS   REASON   AGE
persistentvolume/pv-hostpath   1Gi        RWX            Retain           Bound    default/pvc-hostpath   node-local              112s

NAME                                 STATUS   VOLUME        CAPACITY   ACCESS MODES   STORAGECLASS   AGE
persistentvolumeclaim/pvc-hostpath   Bound    pv-hostpath   1Gi        RWX            node-local     112s

NAME                   READY   STATUS    RESTARTS   AGE
pod/busybox            2/2     Running   0          5h31m
pod/busybox-hostpath   1/1     Running   0          4h38m
pod/busybox-use-pvc    1/1     Running   0          2m48s

# 在node1查看数据写入
[root@k8s-node1 ~]# cat /home/host-pv-dir/data 
hellok8s, pvc used!
```

这里可以看到，Pod使用PVC成功，并且数据已经写入到PVC对应的PV中。需要说明的是，`kk get pv`输出中的`STATUS：Bound`表示绑定存储资源成功，
这里表现为node1上已存在`/home/host-pv-dir`目录（会自动创建）。同理，`kk get pvc`输出中的`STATUS：Bound`表示申请资源成功（有足够的空间可用）。

PVC通过`storageClass`、`accessModes`和存储空间这几个属性来为PVC匹配符合条件的PV资源。具体来说，若要匹配成功，要求在PV和PVC中，
`storageClass`和`accessModes`属性必须一致，而且PVC的`storage`不能超过PV的`capacity`。

另外，这里需要说明一下上述输出中`ACCESS MODES`即访问模式属性，它们的含义如下：

- ReadWriteOnce（RWO）：允许**单个集群节点**以读写模式挂载一个PV
- ReadOnlyMany（ROX）：允许多个集群节点以只读模式挂载一个PV
- ReadWriteMany（RWX）：允许多个集群节点以读写模式挂载一个PV
- ReadWriteOncePod（RWOP，k8s v1.27 beta）：允许单个Pod以读写模式挂载一个PV

单个集群节点上可以运行多个Pod。这个属性值取决于你挂载的存储系统实际支持怎样的访问模式以及个性需求。

**当PVC申请的资源无法满足时**

```shell
# 修改pvc中的storage为大于pv中容量的数字，比如5000Gi
kk get pv,pvc,pod
NAME                           CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS      CLAIM   STORAGECLASS   REASON   AGE
persistentvolume/pv-hostpath   1Gi        RWX            Retain           Available           node-local              6s

NAME                                 STATUS    VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
persistentvolumeclaim/pvc-hostpath   Pending                                      node-local     6s

NAME                   READY   STATUS    RESTARTS   AGE
pod/busybox            2/2     Running   0          5h49m
pod/busybox-hostpath   1/1     Running   0          4h56m
pod/busybox-use-pvc    0/1     Pending   0          6s

$ kk describe pvc pvc-hostpath 
Name:          pvc-hostpath
Namespace:     default
StorageClass:  node-local
Status:        Pending
Volume:        
Labels:        <none>
Annotations:   <none>
Finalizers:    [kubernetes.io/pvc-protection]
Capacity:      
Access Modes:  
VolumeMode:    Filesystem
Used By:       busybox-use-pvc
Events:
  Type     Reason              Age   From                         Message
  ----     ------              ----  ----                         -------
  Warning  ProvisioningFailed  11s   persistentvolume-controller  storageclass.storage.k8s.io "node-local" not found
```

如上所示，当PVC申请的资源无法满足时，创建的pvc和pod都会处于Pending状态，且pvc到Events中会显示无法找到对应的storageclass。
然后我们再修改pv的容量为大于等于pvc申请的容量并apply，接着pvc和pod就会正常启动（无需干预）。

> 经笔者测试，pvc的容量不允许改小，但pv的容量却是可以改小的，且不会立即影响pvc和pod。请注意，这不是一个常规的操作！

#### 1.3.2 PV的解绑和回收

上一小节中已经创建了一个PVC关联到PV，那是否可以再创建一个PVC绑定到同个PV？单独定义[pvc_hostpath.yaml](pvc_hostpath.yaml)
进行验证：

```shell
$ kk apply -f pod-hostpath.yaml 
persistentvolumeclaim/pvc-hostpath-2 created

$ kk describe pvc pvc-hostpath-2
Name:          pvc-hostpath-2
Namespace:     default
StorageClass:  node-local
Status:        Pending
Volume:        
Labels:        <none>
Annotations:   <none>
Finalizers:    [kubernetes.io/pvc-protection]
Capacity:      
Access Modes:  
VolumeMode:    Filesystem
Used By:       <none>
Events:
  Type     Reason              Age   From                         Message
  ----     ------              ----  ----                         -------
  Warning  ProvisioningFailed  8s    persistentvolume-controller  storageclass.storage.k8s.io "node-local" not found
```

即使空间足够，一个PV也不能同时绑定多个PVC，可见PVC和PV是一对一绑定的，想要再次绑定到PV，只能删除PV已经绑定的PVC。

当PV没有被绑定PVC时的状态是`Available`，如果PVC的策略是`Retain`，在删除PVC后。PV的状态会变成`Released`
，若要再次绑定，只能重新创建。如果是`Delete`策略且删除成功，则PVC删除后，PV会直接变成`Available`。

若不想重新创建，也可以直接修改PV的信息（通过`kk edit pv pv-hostpath`删除`claimRef`部分信息）使其变成`Available`
。但建议的操作是清理PV资源后再重新创建。

#### 1.3.3 保护使用中的PV和PVC

k8s默认执行保守的删除策略，当用户想要删除PV或PVC时，k8s不会立即删除使用中的PV和PVC，强制删除也不可以，此时PV和PVC的状态是`Terminating`，
直到不再被使用。

```shell
$ kk delete pvc pvc-hostpath --force
Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.
persistentvolumeclaim "pvc-hostpath" force deleted

$ kk get pv,pvc,pod                 
NAME                           CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                  STORAGECLASS   REASON   AGE
persistentvolume/pv-hostpath   1Ti        RWX            Retain           Bound    default/pvc-hostpath   node-local              21m

NAME                                 STATUS        VOLUME        CAPACITY   ACCESS MODES   STORAGECLASS   AGE
persistentvolumeclaim/pvc-hostpath   Terminating   pv-hostpath   1Ti        RWX            node-local     70s

NAME                   READY   STATUS    RESTARTS   AGE
pod/busybox            2/2     Running   0          6h39m
pod/busybox-hostpath   1/1     Running   0          5h46m
pod/busybox-use-pvc    1/1     Running   0          70s
```

**Finalizers**  
我们可以通过describe查看pvc的信息中包含一行信息：`Finalizers:    [kubernetes.io/pvc-protection]`，Finalizers 是一种
Kubernetes 对象的属性，
用于定义在删除对象时要执行的清理操作。在 PV 对象中，kubernetes.io/pv-protection 是一个 Finalizer，它指示 PV 正在受到保护，防止被删除。
当管理员或用户尝试删除PV或PVC时，Finalizer 会阻止删除操作，直到所有的资源已经释放或者相应的清理操作完成。

这个机制的目的是确保数据的安全性，避免因意外删除而导致数据丢失。

#### 1.3.4 预留PV

有些时候，我们在创建PV时希望将其预留给指定的PVC（可能尚未创建），以便在需要时可以快速创建PVC并绑定到PV上。这主要通过模板中的`claimRef`
字段来实现：

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: foo-pv
spec:
  storageClassName: ""
  claimRef:
    name: foo-pvc # 预留pvc的名称
    namespace: foo
...
```

#### 1.3.5 关于PV和PVC的注意事项

- PV与PVC的绑定需要匹配多个属性值，即存储类名、存储大小、访问模式。
- PV允许被不同namespace中的PVC绑定。
- PV和PVC只能一对一绑定，但一个PVC可以被多个Pod同时使用。由于这一点，PVC的`storage`属性通常设置为和PV一致，不然会造成空间浪费。
- PVC的容量不能缩小，但PV可以，虽然不建议这样做。
- hostPath类型的PV资源一般只用于开发和测试环境，其目的是使用节点上的文件或目录来模拟网络附加存储。在生产集群中，你不会使用
  hostPath。 集群管理员会提供网络存储资源，比如 Google Compute Engine 持久盘卷、NFS 共享卷或 Amazon Elastic Block Store 卷。
- k8s通过一个插件层来连接各种第三方存储系统，这个插件层核心是一套接口叫CSI（Container Storage
  Interface），存储提供商可以自行实现这个接口来对接k8s。

#### 1.3.6 使用StorageClass

前面讲的PV是一种静态创建卷的方式，也就是说，在创建PVC时必须指定一个已经存在的PV。这样的操作步骤在大规模集群中是非常繁琐的，因为需要管理员手动创建和配置PV。

假设你有一个空间较大的存储系统，想要分给多个（可能几十上百个）不同k8s应用使用（每个应用独立使用一段空间），这时候按之前方式就需要手动创建与应用数量一致的PV，
这个工作量是非常大的，以后维护也很麻烦。但如果使用StorageClass，就可以通过**动态方式**创建PV，然后自动绑定到PVC上。

使用StorageClass，管理员可以对接多个存储后端，每个存储后端都可以有不同的配置。比如现有一块高速存储容量1TB，一块标准存储500GB，
就可以定义两个StorageClass，分别叫做`sc-fast-1T`和`sc-std-500G`（k8s中使用`sc`代指StorageClass），然后直接可以创建PVC去绑定其中一个存储类，
绑定成功就会自动创建一个目标大小的PV（并绑定PVC），这个PV由存储类进行自动管理。

**定义StorageClass**  
每个 StorageClass 都包含 `provisioner`、`parameters` 和 `reclaimPolicy` 字段， 这些字段会在 StorageClass 需要动态制备 PV
时会使用到。

每个 StorageClass 都有一个制备器（Provisioner），用来决定使用哪个卷插件制备 PV。 该字段必须指定。
不同的存储后端（如 AWS EBS、GCE PD、Azure Disk 等）都有不同的卷插件，因此需要根据所使用的存储后端指定对应的制备器，以及配置相应的参数。
比如使用NFS作为的存储后端的存储类定义是：

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: example-nfs
provisioner: example.com/external-nfs
parameters:
  server: nfs-server.example.com
  path: /share
  readOnly: "false"
```

而使用AWS EBS作为存储后端的存储类定义是：

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: slow
provisioner: kubernetes.io/aws-ebs
parameters:
  type: io1
  iopsPerGB: "10"
  fsType: ext4
```

可以看到`provisioner`是不同的，而`parameters`更是大相径庭。不过，配置和管理StorageClass的工作是交给专门的运维人员来完成，开发人员不需要清楚其中细节。

[pod_use_storageclass.yaml](pod_use_storageclass.yaml) 是一个使用 StorageClass 的完整模板定义。需要特别说明的是，
这是一个使用hostpath作为存储后端的示例，k8s要求当使用hostpath作为存储后端时，必须手动创建PV来作为一个StorageClass的持久卷，这是一个特例，
使用第三方存储后端时不需要。

使用 StorageClass 的时候，每个Pod使用的空间由 StorageClass 进行管理，它会在存储后端中为每个Pod划分一个单独的空间（目录）。
> 注意：使用hostpath作为存储后端是一个特例，它不会为节点上的每个Pod划分单独的目录，而是共享同一个目录。

使用第三方存储后端时如何填写 StorageClass 的`parameters`
参考[官方文档](https://kubernetes.io/zh-cn/docs/concepts/storage/storage-classes/#parameters) 。

**设置默认的StorageClass**  
可以在集群上启用动态卷制备，以便在未指定存储类的情况下动态设置所有PVC。具体步骤参考[官方文档](https://kubernetes.io/zh-cn/docs/concepts/storage/dynamic-provisioning/#defaulting-behavior) 。

> 开始下一节之前，请先删除本节创建的资源：`kk delete -f pod_use_storageclass.yaml`

#### 1.3.7 使用StatefulSet

StatefulSet 是与Deployment同级的一种 **有状态**
控制器，与无状态部署的Deployment控制器不同的是，StatefulSet可以保证Pod的顺序和唯一性。当有与部署顺序、持久数据或固定网络等有关等特殊应用需求时，
可以上使用 StatefulSet 来部署应用。它可以提供的功能特性如下：

- 有序性：严格按照定义的顺序部署和扩展Pod（正序部署/扩展，倒序删除），每个 Pod 都有一个唯一的索引，从 0 开始；
- 稳定的网络标识符：Pod重新调度后其PodName和Hostname不变，这基于无头Service实现；
- 持久性存储：StatefulSet 通常与 PersistentVolumeClaim (PVC) 配合使用，以提供持久性存储。每个 Pod 可以绑定到一个独立的
  PVC，以确保数据在 Pod 重新调度或故障恢复时不会丢失；

StatefulSet 控制器由3个部分组成：

- 无头Service：用于为Pod资源标识符生成可解析度DNS记录；
- volumeClaimTemplate：基于静态或动态PV供给方式为Pod提供独立的固定存储；
- StatefulSet：用于控制Pod的创建和销毁。

考虑这样一种场景，我们需要在集群中部署3个mysql实例，由于是数据库服务，每个实例都需要一个独立的存储空间，而且它们保存的数据各不相同，
就不能相互替代，如果使用Deployment部署，Pod在重建后，volume虽然没有删除，但没有一种机制让新的Pod实例继续使用之前的volume提供服务，而且重建后的Pod名称也是随机的，
客户端无法知晓新的Pod实例名以读取之前的数据。而使用 StatefulSet 部署就可以提供所需要这些功能。

此外，StatefulSet控制器可以在其模板中配置`volumeClaimTemplate`来为Pod提供存储卷，不需要专门定义PVC。

[stateful-svc.yaml](stateful-svc.yaml)
是一个完整的示例，下面是具体的测试步骤（在开始前，为了让Pod调度到master，请先执行 [删除master污点](https://www.cnblogs.com/zouhong/p/17351418.html)）：

```yaml
$ kk apply -f stateful-svc.yaml
service/stateful-svc created
statefulset.apps/statefulset created
storageclass.storage.k8s.io/sc-hostpath created
persistentvolume/pv-hostpath-0 created
persistentvolume/pv-hostpath-1 created

  # sts代指statefulset
$ kk get svc,sc,pv,pvc,pod,sts -o wide
NAME                   TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)    AGE     SELECTOR
service/kubernetes     ClusterIP   20.1.0.1     <none>        443/TCP    44h     <none>
service/stateful-svc   ClusterIP   None         <none>        8080/TCP   2m24s   app=stateful

NAME                                      PROVISIONER                    RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
storageclass.storage.k8s.io/sc-hostpath   kubernetes.io/no-provisioner   Delete          WaitForFirstConsumer   false                  2m24s

NAME                             CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                                 STORAGECLASS   REASON   AGE     VOLUMEMODE
persistentvolume/pv-hostpath-0   500Gi      RWX            Retain           Bound    default/stateful-data-statefulset-0   sc-hostpath             2m24s   Filesystem
persistentvolume/pv-hostpath-1   500Gi      RWX            Retain           Bound    default/stateful-data-statefulset-1   sc-hostpath             2m24s   Filesystem

NAME                                                STATUS   VOLUME          CAPACITY   ACCESS MODES   STORAGECLASS   AGE    VOLUMEMODE
persistentvolumeclaim/stateful-data-statefulset-0   Bound    pv-hostpath-0   500Gi      RWX            sc-hostpath    103s   Filesystem
persistentvolumeclaim/stateful-data-statefulset-1   Bound    pv-hostpath-1   500Gi      RWX            sc-hostpath    101s   Filesystem

NAME                READY   STATUS    RESTARTS            AGE    IP             NODE         NOMINATED NODE   READINESS GATES
pod/statefulset-0   1/1     Running   0                   103s   20.2.36.83     k8s-node1    <none>           <none>
pod/statefulset-1   1/1     Running   0                   4s     20.2.235.196   k8s-master   <none>           <none>

NAME                           READY   AGE     CONTAINERS            IMAGES
statefulset.apps/statefulset   2/2     2m24s   python-svc-stateful   python:3.7
```

观察测试结果，可以看到：

- 稳定的网络ID：StatefulSet 中的每个 Pod 根据 StatefulSet 的名称和 Pod 的序号派生出它的主机名。
  组合主机名的格式为`$(StatefulSet 名称)-$(序号)`
- Pod是按0开始的序号逐个启动，且要求前一个`Running`后才会启动下一个
- StatefulSet为每个Pod自动创建了PVC资源
- 名为`statefulset`的StatefulSet已经部署到集群中的两个节点且正常运行（删除污点后可以允许Pod调度到Master）

> 在示例模板中存在关于PV资源的定义，在前一小节中已经讲到，这是在为了方便演示使用hostpath作为存储后端而导致必须手动创建PV，在使用其他第三方存储系统时不需要手动创建PV。

现在我们开始进一步验证：

```shell
# 在master执行
[root@k8s-master ~]# cat /home/host-sts-pv-dir/data             
This host is statefulset-1!

# 在node1上执行
[root@k8s-node1 ~]# cat /home/host-sts-pv-dir/data 
This host is statefulset-0!

# 在master启动curl Pod，访问无头Service
$ kk apply -f pod_curl.yaml           
pod/curl created

$ kk exec -it curl --  sh     
/ # nslookup stateful-svc
nslookup: can't resolve '(null)': Name does not resolve

Name:      stateful-svc
Address 1: 20.2.235.201 statefulset-1.stateful-svc.default.svc.cluster.local
Address 2: 20.2.36.84 statefulset-0.stateful-svc.default.svc.cluster.local

/ # curl statefulset-0.stateful-svc.default.svc.cluster.local
<p> The host is statefulset-0</p>
/ # curl statefulset-1.stateful-svc.default.svc.cluster.local
<p> The host is statefulset-1</p>
```

这里，我们验证了两个 StatefulSet Pod对节点本地卷的写入，然后部署一个curl容器来查询部署的 StatefulSet无头服务的DNS信息，
得到了两个由`$(StatefulSet 名称)-$(序号)`组成的稳定虚拟ID（statefulset-1和statefulset-0），
并且通过curl访问了两个Pod的本地服务，得到了预期结果。客户端可以在**集群内**使用这个虚拟ID来访问服务。
当然，也可以使用无头服务的总域名`statefulset-svc.default.svc.cluster.local`来访问，但这样访问的服务是随机的，当我们使用
StatefulSet 部署应用时，说明我们有需要**指定实例ID**进行访问的需求，否则使用Deployment就足够了。

**Pod的重建**  
StatefulSet会确保Pod重建后，Pod使用的存储卷保持不变，以保证Pod重建后仍然能够访问同一份数据。下面通过手动删除Pod模拟故障进行验证：

```shell
$ kk delete pod statefulset-0        
pod "statefulset-0" deleted

# 稍等几秒 Pod 进行重建，由于之前 0号Pod是在node1上允许的，所以在node1上查看重建时写入的数据                                                                                                                                                                               
[root@k8s-node1 ~]# cat /home/host-sts-pv-dir/data                                
This host is statefulset-0!
This host is statefulset-0!
```

可以看到，Pod重建后仍然能够在之前的节点写入相同的数据。虽然这里使用hostpath作为存储后端来验证这个功能不太严谨（因为Pod-1占用了另一个节点的本地卷，所以0号Pod一定会在原来的节点重建），
但 StatefulSet控制器 确实拥有这个功能，读者可以使用其他存储系统（如NFS）进行验证。

**StatefulSet的伸缩与更新**  
和Deployment一样，StatefulSet也支持动态伸缩，当StatefulSet的Replicas数量发生变化时（或直接通过`kubectl scale`指令），StatefulSet控制器会确保Pod数量最终符合预期。
但不同的是，StatefulSet执行的是有序伸缩，具体来说是在扩容时从编号较小的开始逐个创建，而缩容时则是倒序进行。

StatefulSet有两种更新策略，可以通过`.spec.updateStrategy`字段进行控制。

- **OnDelete**：当 `.spec.updateStrategy.type` 设置为 OnDelete 时， 它的控制器将不会自动更新 StatefulSet 中的 Pod。 用户必须手动删除
  Pod 以便让控制器创建新的 Pod；
- **RollingUpdate**：当 `.spec.updateStrategy.type` 设置为 RollingUpdate 时，对 StatefulSet 中的 Pod 执行自动的滚动更新。这是默认的更新策略。
    - 这种情况下，StatefulSet 控制器会从Pod序号大到小的顺序进行逐个更新（当Pod进入`Running`时再更新下一个）；
    - 分区滚动更新：通过声明 `.spec.updateStrategy.rollingUpdate.partition` 的方式，RollingUpdate 更新策略可以实现分区。
        - 比如，当 partition 设置为 1 时，StatefulSet 控制器只会更新序号大于等于 1 的
          Pod（如果大于replicas，则不会更新任何Pod）。当你需要进行分阶段（金丝雀）更新时才会用到这个参数。

**PVC的保留**  
默认情况下，当Pod被删除时，StatefulSet控制器不会删除这个Pod使用的PVC，在 k8s v1.27版本中，可以 [进行配置](https://kubernetes.io/zh-cn/docs/concepts/workloads/controllers/statefulset/#persistentvolumeclaim-retention)。

## TODO
## 参考

- [Kubernetes从入门到实践 @赵卓](https://www.epubit.com/bookDetails?id=UB72096269c1157)
- [Kubernetes修炼手册 @Nigel Poulton](https://book.douban.com/subject/35486781/)