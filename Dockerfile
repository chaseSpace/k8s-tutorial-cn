FROM golang:1.20-buster AS builder

WORKDIR /project
ADD . .

# 实战中，如果有gomod文件，单独一行
#RUN go mod tidy

# 关闭cgo的原因：使用了多阶段构建，go程序的编译环境和运行环境不同
RUN GOPROXY=https://goproxy.cn,direct GOOS=linux CGO_ENABLED=0 GOARCH=amd64 GO111MODULE=auto go build -o main -ldflags "-w -extldflags -static"


FROM scratch as prod
# 关于docker基础镜像：scratc、busybox、alpine
# http://www.asznl.com/post/48


COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
COPY --from=builder /project/main .

EXPOSE 3000
ENTRYPOINT ["/main"]