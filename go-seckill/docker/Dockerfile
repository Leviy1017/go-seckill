# ============================================================
# 阶段一：构建 Go 二进制文件
# ============================================================
FROM golang:1.21-alpine AS builder
#FROM —— 指定基础镜像
#给这个阶段起个名字叫 builder

# 设置 Go 代理加速依赖下载（国内用户建议保留）
ENV GOPROXY=https://goproxy.cn,direct

#进到容器的 /app 目录，后面所有命令都在这个目录下执行
WORKDIR /app

# 先复制依赖文件，利用 Docker 缓存层
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并编译
COPY . .
#：把项目所有代码复制进来（但 .dockerignore 里的不会复制）

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o seckill .

# ============================================================
# 阶段二：最小运行时镜像
#第二个 FROM 开头，意味着多阶段构建的第二阶段开始
# ============================================================
FROM alpine:3.19

# 安装运行依赖：netcat 用于等待 MySQL/Redis 就绪，wget 用于健康检查
RUN apk add --no-cache ca-certificates tzdata netcat-openbsd

# 设置时区为上海
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/seckill .

# 复制启动脚本  给它执行权限
COPY docker/entrypoint.sh .
RUN chmod +x entrypoint.sh

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/auth/buyer/login > /dev/null || exit 1

# 通过启动脚本启动（自动等待 MySQL/Redis 就绪）
ENTRYPOINT ["./entrypoint.sh"]
