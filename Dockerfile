FROM golang:1.20-alpine AS builder

WORKDIR /build
COPY code/ .

# 安装必要的构建工具
RUN apk add --no-cache gcc musl-dev

# 下载依赖并构建
RUN go mod download
# 显式安装缺失的依赖包
RUN go get github.com/larksuite/oapi-sdk-go/v3/service/cardkit/v1
RUN CGO_ENABLED=0 GOOS=linux go build -o feishu-bot

FROM alpine:latest

WORKDIR /app
COPY --from=builder /build/feishu-bot .
COPY code/role_list.yaml .

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata curl

# 创建日志目录
RUN mkdir logs

EXPOSE 9000
CMD ["./feishu-bot"]
