#!/bin/sh
# ============================================================
# 等待 MySQL 和 Redis 就绪后再启动 Go 应用
# ============================================================

set -e
 # 任何命令出错就立即退出，不继续执行

echo "=== 等待 MySQL 就绪 (${DB_HOST}:${DB_PORT}) ==="
until nc -z "$DB_HOST" "$DB_PORT"; do
    echo "MySQL 尚未就绪，等待 2 秒..."
    sleep 2
done
echo "MySQL 已就绪"

echo "=== 等待 Redis 就绪 (${REDIS_ADDR}) ==="
# 提取 Redis 地址中的主机和端口
REDIS_HOST=$(echo "$REDIS_ADDR" | cut -d: -f1)
REDIS_PORT=$(echo "$REDIS_ADDR" | cut -d: -f2)
until nc -z "$REDIS_HOST" "$REDIS_PORT"; do
    echo "Redis 尚未就绪，等待 2 秒..."
    sleep 2
done
echo "Redis 已就绪"

echo "=== 启动秒杀服务 ==="
exec ./seckill
