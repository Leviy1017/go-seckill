package database

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

var RDB *redis.Client
var Ctx = context.Background() //空白上下文 常用默认空环境

func InitRedis() error {
	dbNum, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	RDB = redis.NewClient(&redis.Options{
		Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       dbNum,
	})

	if err := RDB.Ping(Ctx).Err(); err != nil {
		return fmt.Errorf("Redis 连接失败: %w", err)
	}

	log.Println("[Redis] 连接成功")
	return nil
}

// WarmUpStock 库存预热：将商品库存同步到 Redis
// /RDB.Set 将库存写入 Redis，0 表示永不过期
func WarmUpStock(productID int, stock int) error { //stock 库存数值
	key := fmt.Sprintf("seckill:stock:%d", productID)
	return RDB.Set(Ctx, key, stock, 0).Err()
}

// DeductStock Redis 原子扣库存，返回扣减后的值
// fallbackStock: Redis key 不存在时从 MySQL 取库存值预热进去
func DeductStock(productID int, fallbackStock int) (int64, error) {
	key := fmt.Sprintf("seckill:stock:%d", productID)
	// 检查 Redis key 是否存在，不存在就从 MySQL 预热
	//更新日志:首先检查key是否存在再进行操作
	exists, _ := RDB.Exists(Ctx, key).Result()
	if exists == 0 && fallbackStock > 0 {
		RDB.Set(Ctx, key, fallbackStock, 0)
	}
	return RDB.Decr(Ctx, key).Result()
}

// GetStock 获取 Redis 中的剩余库存
func GetStock(productID int) (int, error) {
	key := fmt.Sprintf("seckill:stock:%d", productID)
	val, err := RDB.Get(Ctx, key).Result()
	if err != nil {
		return 0, err
	}
	stock, _ := strconv.Atoi(val)
	return stock, nil
}

// CheckDuplicateOrder 防重复下单：用 SETNX 原子标记"某个买家正在抢某个商品"
// 返回 true 表示可以继续下单（首次），false 表示重复请求
func CheckDuplicateOrder(buyerID, productID int) bool {
	key := fmt.Sprintf("seckill:ordering:%d:%d", buyerID, productID)
	// SETNX：key 不存在才设置成功。10 秒过期防止死锁
	ok, _ := RDB.SetNX(Ctx, key, 1, 10*time.Second).Result()
	return ok
}

// ClearDuplicateFlag 下单完成后清除标记（成功或失败都清，让用户下次能重试）
func ClearDuplicateFlag(buyerID, productID int) {
	key := fmt.Sprintf("seckill:ordering:%d:%d", buyerID, productID)
	RDB.Del(Ctx, key)
}

// CheckRateLimit 限流：同一买家每秒最多 N 次下单请求
// 返回 true 表示未超限，false 表示被限流
func CheckRateLimit(buyerID int, limitPerSec int) bool {
	key := fmt.Sprintf("seckill:rate:%d", buyerID)
	// INCR 是原子操作，第一次调用会从 0 加到 1
	count, _ := RDB.Incr(Ctx, key).Result()
	// 第一次设置这个 key 时加过期时间（1 秒窗口）
	if count == 1 {
		RDB.Expire(Ctx, key, time.Second)
	}
	return count <= int64(limitPerSec)
}
