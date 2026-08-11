# go-seckill 秒杀系统 — 学习指南

> 前置知识：MySQL + Redis 理论 + Go 基础语法

---

## 学习路线（1~3 天）

```
Day 1: 看懂每个文件 → 理解项目结构 → 跑起来
Day 2: 用 curl 试完所有接口 → 理解下单 7 步流程
Day 3: 自己动手改 → 加一个功能（如防重复下单）
```

---

## 第一步：先跑起来

```bash
# 1. 确保 MySQL 和 Redis 在运行

# 2. 建数据库
mysql -u root -p -e "CREATE DATABASE seckill"

# 3. 编译运行
GOTOOLCHAIN=local GONOSUMCHECK='*' GONOSUMDB='*' /usr/local/go/bin/go build -o seckill .
./seckill

# 4. 服务跑在 :8080
```

---

## 第二步：用 curl 逐个测试接口

### 1. 买家注册
```bash
curl -X POST http://localhost:8080/api/auth/buyer/register \
  -H "Content-Type: application/json" \
  -d '{"username":"张三","password":"123456","phone":"13800000001","address":"北京"}'
```

### 2. 买家登录
```bash
curl -X POST http://localhost:8080/api/auth/buyer/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000001","password":"123456"}'
```
拿到返回的 token，后面用。

### 3. 卖家注册 + 登录
```bash
curl -X POST http://localhost:8080/api/auth/seller/register \
  -H "Content-Type: application/json" \
  -d '{"shop_name":"数码旗舰店","password":"123456","phone":"13900000001","shop_addr":"上海"}'

curl -X POST http://localhost:8080/api/auth/seller/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13900000001","password":"123456"}'
```
拿到卖家 token。

### 4. 卖家上架秒杀商品
```bash
curl -X POST http://localhost:8080/api/seller/product \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <卖家的token>" \
  -d '{
    "name":"视频会员年卡",
    "description":"原价99，秒杀价9.9",
    "original_price":99,
    "seckill_price":9.9,
    "stock":100,
    "seckill_start":"2025-01-01 00:00:00",
    "seckill_end":"2026-12-31 23:59:59"
  }'
```

### 5. 买家查看秒杀列表
```bash
curl http://localhost:8080/api/buyer/seckill \
  -H "Authorization: Bearer <买家的token>"
```

### 6. 买家下单（秒杀核心）
```bash
curl -X POST http://localhost:8080/api/buyer/order \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <买家的token>" \
  -d '{"product_id":1}'
```

### 7. 买家查询订单
```bash
curl "http://localhost:8080/api/buyer/order/status?id=<订单ID>" \
  -H "Authorization: Bearer <买家的token>"
```

### 8. 卖家接单 → 完成
```bash
curl -X POST http://localhost:8080/api/seller/order/accept \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <卖家的token>" \
  -d '{"order_id":"<订单ID>"}'

curl -X POST http://localhost:8080/api/seller/order/complete \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <卖家的token>" \
  -d '{"order_id":"<订单ID>"}'
```

---

## 第三步：看懂下单的 7 步流程

这是整个项目最有价值的代码，在 `handlers/buyer.go` 的 `BuyerPlaceOrder`：

```
1. 查商品信息      → 商品是否存在
2. 检查秒杀状态    → 是否为 active（进行中）
3. 查买家信息      → 用于订单快照
4. 查卖家信息      → 用于订单快照
5. Redis DECR 扣库存  → 核心！原子操作，返回 < 0 代表售罄
6. MySQL UPDATE 同步库存 → 持久化
7. INSERT INTO seckill_orders → 创建订单
```

**Redis 扣库存为什么能防超卖**：

```
假设库存=100，3个人同时抢：

Redis 单线程处理：
  请求A → DECR → 返回 99 ✓
  请求B → DECR → 返回 98 ✓
  请求C → DECR → 返回 97 ✓

最后一轮：
  请求X → DECR → 返回 0  ✓（刚好最后一个）
  请求Y → DECR → 返回 -1 ✗（remain<0，回滚 Incr +1，返回"已售罄"）
  请求Z → DECR → 返回 -1 ✗
```

---

## 第四步：自己动手改

### 练习 1：防重复下单
在 Redis 里记录"哪个买家已经抢过哪个商品"：
```go
// 下单前检查
key := fmt.Sprintf("seckill:bought:%d:%d", buyerID, productID)
exists, _ := RDB.Exists(ctx, key).Result()
if exists > 0 { return "你已经抢过了" }
// 下单成功后记录
RDB.Set(ctx, key, 1, 24*time.Hour)
```

### 练习 2：实现秒杀倒计时 API
返回每个商品的 `seckill_start - now()`，前端可以展示倒计时。

### 练习 3：客户端并发测试
写个脚本同时发起 200 个下单请求，看库存是否精确扣到 0 且不超卖。

---

## 项目分层速查

```
main.go                    → 入口：初始化DB/Redis、注册路由、启动服务
│
├── handlers/              → HTTP 层：接收请求、调 database 层、返回 JSON
│   ├── middleware.go      → JWT 认证、CORS、日志
│   ├── auth.go            → 注册/登录
│   ├── buyer.go           → 买家：秒杀列表、下单、订单查询
│   └── seller.go          → 卖家：上架商品、接单、完成订单
│
├── database/              → 数据层：所有 SQL 和 Redis 操作
│   ├── mysql.go           → MySQL 连接池 + 自动建表
│   ├── redis.go           → Redis 连接 + 库存预热/扣减
│   └── queries.go         → 买家、卖家、商品、订单的 CRUD
│
├── models/                → 数据结构（结构体 + JSON tag）
│   └── struct.go
│
└── response/              → 统一返回格式 {code, message, data}
    └── response.go
```

---

## Go 关键知识点清单

| 知识点 | 在哪里用到 |
|--------|-----------|
| `struct` + JSON tag | `models/struct.go` 每个结构体 |
| `json:"-"` 隐藏字段 | `Password string \`json:"-"\`` |
| 匿名导入 `_ "driver"` | `database/mysql.go` 第 10 行 |
| `context.WithValue` | `handlers/middleware.go` 传 userID |
| `interface{}` 泛型 | `response/response.go` Data 字段 |
| `json.NewDecoder` | `handlers/auth.go` 解析请求体 |
| `sql.DB` 连接池 | `database/mysql.go` |
| JWT `jwt.MapClaims` | `handlers/middleware.go` |
| bcrypt 密码哈希 | `database/queries.go` |
| Redis `DECR` 原子操作 | `database/redis.go` |
| `type alias` 自定义类型 | `type contextKey string` |
| `strings.SplitN` | `main.go` 解析路由 pattern |
| `http.HandlerFunc` 类型 | 每个 handler 函数的签名 |
