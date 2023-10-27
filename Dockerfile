FROM golang:1.20-alpine AS builder

WORKDIR /project
ADD . .

# 实战中，如果有gomod文件，单独一行
#RUN go mod tidy


# 关闭cgo的原因：使用了多阶段构建，go程序的编译环境和运行环境不同，不关就无法运行go程序
RUN GOPROXY=https://goproxy.cn,direct GOOS=linux CGO_ENABLED=0 GOARCH=amd64 GO111MODULE=auto go build -o main -ldflags "-w -extldflags -static"

#FROM scratch as prod
FROM alpine as prod
# 关于docker基础镜像：scratc、busybox、alpine
# http://www.asznl.com/post/48

# alpine设置时区
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories &&  \
    apk add -U tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && apk del tzdata && date

COPY --from=builder /project/main .

EXPOSE 3000
ENTRYPOINT ["/main"]