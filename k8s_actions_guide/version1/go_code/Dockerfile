FROM golang:1.20-alpine AS builder

WORKDIR /go/cache
COPY go.mod .
COPY go.sum .
RUN GOPROXY=https://goproxy.cn,direct go mod tidy

WORKDIR /build

# 复制项目代码到容器中（注意不要包含代码以外的文件，避免占用过多构建机空间）
COPY . .

# 关闭cgo的原因：使用了多阶段构建，go程序的编译环境和运行环境不同，不关就无法运行go程序
RUN GOPROXY=https://goproxy.cn,direct GOOS=linux CGO_ENABLED=0 GOARCH=amd64 GO111MODULE=auto go build -o main -ldflags "-w -extldflags -static"

FROM alpine:3.19.0 as prod

# alpine设置时区
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories &&  \
    apk add -U tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && apk del tzdata && date

COPY --from=builder /build/main .

ENTRYPOINT ["/main"]