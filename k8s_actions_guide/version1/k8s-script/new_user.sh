#!/usr/bin/env bash
set -e # 出现任何错误则停止继续执行

<<comment
@集群管理员执行
此脚本在K8s集群中添加一个新用户，具体完成一下工作：
- 创建新的命名空间，包含用户名
- 创建新用户证书（使用集群CA签名）
- 部署RBAC资源，限制用户仅能访问和操作自己命名空间下的资源
- 部署ResourceQuota资源，对资源使用限额

使用方式：sh new_user.sh your_user
comment

USER=$1

if [[ -z $USER ]]; then
  echo '请提供一个具有辨识度的用户名称，例如 luxun'
  exit 1
fi

alias kk="kubectl"

NS_NAME=ns-$USER

# new namespace
kk create namespace $NS_NAME
kk annotate namespace $NS_NAME developer=true

CERT_NAME=developer-cert-$USER
# new user by create client certificate
openssl genrsa -out $CERT_NAME.key 1024
openssl req -new -key $CERT_NAME.key -out $CERT_NAME.csr -subj "/CN=$USER" >/dev/null
# -- 注意需要提供集群的ca.crt和ca.key路径，可以从master节点的/etc/kubernetes/pki目录下获取
sudo openssl x509 -req -in $CERT_NAME.csr -CA /etc/kubernetes/pki/ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out $CERT_NAME.crt -days 365
rm -rf $CERT_NAME.csr
# -- 查看证书信息
# openssl x509 -in $CERT_NAME.crt -text -noout
echo "***Created certificate related files: $CERT_NAME.key $CERT_NAME.crt"

# 创建ClusterRole
cat <<EOF | kk replace -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cr-developer
rules:
  - apiGroups: [ "" ]
    resources: [ "configmaps", "endpoints", "events", "persistentvolumeclaims", "persistentvolumes", "pods", "podtemplates", "replicationcontrollers", "secrets", "serviceaccounts", "services"]
    verbs: [ "*" ]

  - apiGroups: [ "apps/v1" ]
    resources: [ "deployments", "daemonsets", "replicasets", "statefulsets", "controllerrevisions"]
    verbs: [ "*" ]

  - apiGroups: [ "batch/v1" ]
    resources: [ "cronjobs", "jobs"]
    verbs: [ "*" ]

  - apiGroups: [ "networking.k8s.io/v1" ]
    resources: [ "*"]
    verbs: [ "*" ]

  - apiGroups: [ "policy/v1" ]
    resources: [ "poddisruptionbudgets"]
    verbs: [ "*" ]

  - apiGroups: [ "storage.k8s.io/v1" ]
    resources: [ "storageclasses"]
    verbs: [ "*" ]
EOF

# 创建RoleBing
cat <<EOF | kk create -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: rb-developer
  namespace: $NS_NAME # 授权使用此空间内的资源
subjects: # 你可以指定不止一个subject（主体），包含用户、组或服务账户
  - kind: User
    name: $USER
roleRef:
  kind: ClusterRole
  name: cr-developer
  apiGroup: rbac.authorization.k8s.io
EOF

# 创建ResourceQuota（进行资源限额）
cat <<EOF | kk create -f -
apiVersion: v1
kind: ResourceQuota
metadata:
  name: quota-$USER
  namespace: $NS_NAME
spec:
  hard:
    limits.cpu: "5"
    limits.memory: "1Gi"
    requests.cpu: "5"
    requests.memory: "1Gi"
    requests.storage: "2Gi"
    persistentvolumeclaims: "10"
    configmaps: "10"
    pods: "50"
    services: "10"
    services.loadbalancers: "10"
    services.nodeports: "5"
    secrets: "10"
EOF

# 检查权限
# kk auth can-i list rolebinding --namespace $NS_NAME --as $USER

echo '用户已创建。'
