## docker build --no-cache=true -f dockerfile -t registry.cn-hangzhou.aliyuncs.com/generals-kuber/crd-ipkeeper:1.0.0 .
########################################################
FROM golang:1.12 as builder
## docker镜像通用设置
LABEL author=general
LABEL email="generals.space@gmail.com"
## 环境变量, 使docker容器支持中文
ENV LANG C.UTF-8

WORKDIR /crd-ipkeeper
COPY . .
ENV GO111MODULE on
ENV GOPROXY https://goproxy.cn
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o crd-ipkeeper ./cmd

########################################################
FROM generals/alpine
## docker镜像通用设置
LABEL author=general
LABEL email="generals.space@gmail.com"
## 环境变量, 使docker容器支持中文
ENV LANG C.UTF-8

COPY --from=builder /crd-ipkeeper/crd-ipkeeper /
CMD ["/crd-ipkeeper"]
