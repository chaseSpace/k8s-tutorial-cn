## 技巧汇总

### 1. 当Apply模板后Pod状态持续在Init时
这时候多半是卡在镜像拉取，通过`describe pod`指令查看Pod Events部分中需要拉取的镜像名称，例如：
```
$ kk describe pod kube-router-dnwxp -nkube-system
...
Events:
  Type    Reason     Age   From               Message
  ----    ------     ----  ----               -------
  Normal  Scheduled  4m6s  default-scheduler  Successfully assigned kube-system/kube-router-dnwxp to k8s-master
  Normal  Pulling    4m5s  kubelet            Pulling image "docker.io/cloudnativelabs/kube-router"
```
然后手动拉取：
```shell
# 在每个节点执行，若名称后没有tag，需要加上latest，否则ctr不识别
ctr image pull docker.io/cloudnativelabs/kube-router:latest
```
再重新部署：
```shell
kk delete -f kubeadm-kuberouter.yaml --force
kk apply -f kubeadm-kuberouter.yaml
```
也可以仅删除对应的Pod，让其自动重启。