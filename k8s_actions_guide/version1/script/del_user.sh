#!/usr/bin/env bash
set -e # 出现任何错误则停止继续执行

<<comment
@集群管理员执行
此脚本在K8s集群中*删除*一个新用户关联的：
- 命名空间（包含其中的所有资源）

使用方式：sh del_user.sh your_user
comment

USER=$1

if [[ -z $USER ]]; then
  echo '请提供需要在集群中删除的用户名称，例如 LuXun'
  exit 1
fi

alias kk="kubectl"

NS_NAME=ns-$USER
CERT_NAME=client-cert-$USER

# 这会删除该空间下的所有资源
kk delete ns $NS_NAME

rm -rf $CERT_NAME.crt $CERT_NAME.key
