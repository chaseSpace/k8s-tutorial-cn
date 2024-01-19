#!/usr/bin/env bash
set -e

<<comment
@集群管理员执行
comment

usage="用法：sh setup_kubeconfig.sh  <用户名> <API Server地址>"

# 首先需要安装kubectl

USER=$1   # 例如luxun
SERVER=$2 # 例如https://1.1.1.1

if [ $# != 2 ]; then
  echo "参数数量错误"
  echo $usage
  exit 1
fi

# 新增一个kubeconfig文件
KUBECONFIG=developer-kubeconfig
touch $KUBECONFIG

CLUSTER=work
LOCAL_USER=developer
NAMESPACE=ns-$USER

# 准备以下三个文件
CA=/etc/kubernetes/pki/ca.crt
CLIENT_CRT=developer-cert-$USER.crt
CLIENT_KEY=developer-cert-$USER.key

# 添加cluster
kubectl config --kubeconfig=$KUBECONFIG set-cluster $CLUSTER --server=$SERVER --certificate-authority=$CA --embed-certs

# 添加user
kubectl config --kubeconfig=$KUBECONFIG set-credentials $LOCAL_USER --embed-certs=true --client-certificate=$CLIENT_CRT --client-key=$CLIENT_KEY

# 添加context
kubectl config --kubeconfig=$KUBECONFIG set-context $LOCAL_USER@$CLUSTER --cluster=$CLUSTER --user=$LOCAL_USER --namespace=$NAMESPACE

# 使用这个新建的context
kubectl config --kubeconfig=$KUBECONFIG use-context $LOCAL_USER@$CLUSTER

# 查看当前上下文关联信息
kubectl config --kubeconfig=$KUBECONFIG view --minify

# 检查权限
# kk --kubeconfig=$KUBECONFIG auth can-i list rolebinding --namespace default
# kk --kubeconfig=$KUBECONFIG auth can-i list po --namespace YOUR_NS

# 了解更多kubeconfig管理命令
# https://kubernetes.io/zh-cn/docs/tasks/access-application-cluster/configure-access-multiple-clusters/