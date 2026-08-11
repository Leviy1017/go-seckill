# 虚拟商品秒杀系统 + AI 运维 Agent

> 一个完整的 Go 高并发秒杀后端 + Python LLM 智能运维监控系统。适合作为 Go Web 后端、高并发、AI Agent 方向的实习作品。

---

## 项目概览

本项目包含两大子系统：

| 子系统 | 语言 | 定位 |
|--------|------|------|
| **go-seckill** | Go 1.21 | 秒杀商品后端服务，支撑买家抢购、卖家管理、实时聊天的完整链路 |
| **seckill-guardian** | Python 3.12 | AI 智能运维 Agent，定时巡检 → LLM 链式推理诊断 → 生成修复建议 |

**业务闭环**：买家注册 → 浏览秒杀 → 抢购下单 → 卖家接单/发货 → 买卖家实时聊天

**运维闭环**：定时探针巡检 → 发现异常 → 收集上下文 → LLM 推理诊断 → 输出报告

---

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                        Client 层                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐  │
│  │ 买家 App  │  │ 卖家 App  │  │ Web 页面  │  │ 管理员面板  │  │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └─────┬──────┘  │
│        │              │              │              │          │
└────────┼──────────────┼──────────────┼──────────────┼──────────┘
         │              │              │              │
         └──────────────┴──────┬───────┴──────────────┘
                               │
                    ┌──────────▼───────────┐
                    │   Go 秒杀服务 :8080   │
                    │  ┌─────────────────┐  │
                    │  │  Middleware 层   │  │
                    │  │ JWT · CORS · Log │  │
                    │  ├─────────────────┤  │
                    │  │   Handler 层     │  │
                    │  │ Auth · Seckill   │  │
                    │  │ Order · Chat     │  │
                    │  ├─────────────────┤  │
                    │  │  WebSocket Hub   │  │
                    │  │ 实时聊天连接池    │  │
                    │  └────────┬────────┘  │
                    └───────────┼───────────┘
                                │
              ┌─────────────────┼─────────────────┐
              │                 │                 │
     ┌────────▼──────┐  ┌───────▼───────┐  ┌──────▼───────┐
     │   MySQL 8.0   │  │    Redis 7    │  │  Redis Pub/Sub│
     │  6 张数据表    │  │  库存 · 限流   │  │  跨进程消息转发  │
     │  持久化存储    │  │  防重 · 缓存   │  │               │
     └────────▲──────┘  └───────▲───────┘  └───────▲───────┘
              │                 │                    │
              │    ┌────────────┴────────────┐       │
              │    │   Python AI Agent       │       │
              │    │  ┌──────────────────┐   │       │
              │    │  │ 4 个探针 (Probe) │   │       │
              │    │  │ 库存 · 超时订单    │   │       │
              │    │  │ 连接池 · 限流攻击  │   │       │
              │    │  ├──────────────────┤   │       │
              │    │  │ 上下文收集器      │   │       │
              │    │  ├──────────────────┤   │       │
              │    │  │ 滑动窗口记忆      │   │       │
              │    │  ├──────────────────┤   │       │
              │    │  │ LLM 链式推理     │   │       │
              │    │  │ 4 步诊断链       │   │       │
              │    │  └──────────────────┘   │       │
              │    └─────────────────────────┘       │
              └──────────────────────────────────────┘

    说明:
    ———→ HTTP 请求         ···→ WebSocket 连接
    ———→ 数据读写           - - → Pub/Sub 订阅
    ———→ AI Agent 只读不写，不做任何修改操作
```

---

## 技术栈

### Go 秒杀后端

| 领域 | 选型 | 用途 |
|------|------|------|
| HTTP 路由 | `net/http` 标准库 | 手写路由 + 方法检查 + 中间件 |
| 数据库 | MySQL 8.0 | 6 张表，持久化存储 |
| 缓存 | Redis 7 | 库存预热 / 原子扣减 / 限流 / 防重 |
| 认证 | JWT (HS256) | 24h 过期 Token |
| 密码安全 | bcrypt | 加盐哈希 |
| ID 生成 | UUID v4 | 订单号唯一标识 |
| 实时通信 | WebSocket | gorilla/websocket |
| 消息转发 | Redis Pub/Sub | 跨进程消息广播 |
| 部署 | Docker + Compose | 三服务编排 + healthcheck |

### Python AI 运维 Agent

| 领域 | 选型 | 用途 |
|------|------|------|
| MySQL 连接 | pymysql | 查询连接池状态、订单数据 |
| Redis 连接 | redis-py | 查询库存、限流 key |
| LLM | OpenAI 兼容 API | 链式推理诊断 |
| 配置管理 | python-dotenv | 环境变量管理 |

---

## 快速开始

### 环境要求

- Docker & Docker Compose（推荐一键启动）
- Go 1.21+（仅本地开发需要）
- Python 3.12+（仅运行 AI Agent 需要）

### 方式一：Docker Compose 一键启动（推荐）

```bash
# 启动所有服务（MySQL + Redis + Go 秒杀服务）
cd go-seckill
docker compose -f docker/docker-compose.yml up -d

# 查看日志
docker compose -f docker/docker-compose.yml logs -f app

# 停止服务
docker compose -f docker/docker-compose.yml down
```

### 方式二：本地运行

```bash
# 1. 启动 MySQL 和 Redis
cd go-seckill
docker compose -f docker/docker-compose.yml up -d mysql redis

# 2. 配置环境变量
cp config.env.example config.env
# 编辑 config.env，修改数据库连接地址（本地用 localhost）

# 3. 启动 Go 服务
source config.env && go run main.go
```

### 接口测试

```bash
# 1. 注册买家
curl -X POST http://localhost:8080/api/auth/buyer/register \
  -H "Content-Type: application/json" \
  -d '{"username":"张三","password":"123456","phone":"13800000001","address":"北京市朝阳区"}'

# 2. 注册卖家
curl -X POST http://localhost:8080/api/auth/seller/register \
  -H "Content-Type: application/json" \
  -d '{"shop_name":"话费充值旗舰店","password":"123456","phone":"13900000001","shop_addr":"上海市浦东新区"}'

# 3. 买家登录（获取 Token）
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/buyer/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"13800000001","password":"123456"}' | jq -r '.data.token')

# 4. 卖家登录 & 上架秒杀商品
curl -X POST http://localhost:8080/api/seller/product \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <卖家TOKEN>" \
  -d '{"name":"50元话费卡","description":"秒杀价仅需5元！","original_price":50,"seckill_price":5,"stock":100,"seckill_start":"2026-01-01T00:00:00Z","seckill_end":"2027-12-31T23:59:59Z"}'

# 5. 买家查看秒杀列表
curl http://localhost:8080/api/buyer/seckill \
  -H "Authorization: Bearer $TOKEN"

# 6. 买家下单抢购
curl -X POST http://localhost:8080/api/buyer/order \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"product_id":1}'
```

---

## API 文档

### 公开接口（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/buyer/register` | 买家注册 |
| POST | `/api/auth/buyer/login` | 买家登录（返回 JWT Token） |
| POST | `/api/auth/seller/register` | 卖家注册 |
| POST | `/api/auth/seller/login` | 卖家登录（返回 JWT Token） |

### 买家接口（需要买家 Token）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/buyer/seckill` | 查看秒杀商品列表 |
| GET | `/api/buyer/orders` | 查询我的订单 |
| GET | `/api/buyer/order/status` | 查询单个订单状态 |
| POST | `/api/buyer/order` | **下单抢购**（核心接口） |

### 卖家接口（需要卖家 Token）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/seller/product` | 上架秒杀商品 |
| PUT | `/api/seller/product/stock` | 修改商品库存 |
| GET | `/api/seller/orders` | 查看本店订单 |
| POST | `/api/seller/order/accept` | 接单 |
| POST | `/api/seller/order/complete` | 完成订单 |

### 聊天接口（需要登录）

| 方法 | 路径 | 说明 |
|------|------|------|
| WS | `/api/chat/ws?token=xxx` | WebSocket 实时聊天 |
| GET | `/api/chat/conversations` | 获取会话列表 |
| GET | `/api/chat/messages` | 获取历史消息 |

---

## 核心技术亮点

### 1. 秒杀下单：Redis + MySQL 双保险防超卖

```
买家请求 → 限流检查(每秒3次)
        → 查询商品 & 校验秒杀时间窗口
        → Redis SETNX 防重(10s过期)
        → Redis DECR 原子扣库存 ←── 第一道防线(快速过滤)
        → MySQL 事务 BEGIN
            → 数据库层防重检查
            → UPDATE ... WHERE stock > 0   ←── 第二道防线(行级锁强一致)
            → INSERT 订单
            → 库存为 0 时自动标记 sold_out
            → COMMIT
        → 事务失败则 Redis INCR 回滚库存
```

**设计要点**：
- Redis 先扣：O(1) 时间复杂度，几十万 QPS 无压力
- MySQL 兜底：`WHERE stock > 0` + InnoDB 行级锁保证强一致性
- 事务失败自动回滚 Redis：保证库存不丢

### 2. 三层防刷机制

| 层级 | 机制 | 实现 |
|------|------|------|
| 频率控制 | 限流 | Redis INCR + 1秒滑动窗口，每用户每秒最多 3 次请求 |
| 重复请求 | 防重 | Redis SETNX，同一买家同一商品 10s 内仅允许一次 |
| 时间窗口 | 验证 | 服务端校验 `now >= start && now <= end`，同时校验 status 和时间 |
| 数据库兜底 | 事务检查 | MySQL 事务内 SELECT COUNT 防重，防止 Redis 挂了也能拦截 |

### 3. 即时聊天：三层架构保证可靠性

```
买家发消息
  → 存入 MySQL (持久化，保证不丢)
  → 检查对方是否在线
     ├── 在线 → 直接通过 WebSocket 推送
     └── 不在线
         → 本机连接池找不到
         → 通过 Redis Pub/Sub 发布到 "chat" 频道
         → 对方服务器监听到 → 推给目标用户

对方上线时：
  → 查询 MySQL 未读消息
  → 推送"未读摘要"（避免大量消息轰炸）
  → 点开会话时标记已读
```

**设计要点**：
- WebSocket 连接池用 `(UserID + Role)` 作为 Key，区分买家和卖家的 ID 冲突
- RWMutex 保护并发读写
- 离线消息摘要化推送，而非逐条轰炸

### 4. AI 运维 Agent：4 探针 + 4 步链式推理

**4 个探针，定时巡检**：

| 探针 | 检查目标 | 告警条件 |
|------|---------|---------|
| 库存一致性 | Redis vs MySQL 库存对比 | key 丢失 / 数据不一致 / Redis 0 但 MySQL > 0 |
| 超时订单 | paid 状态订单 | 超过 30 分钟未接单 |
| 连接池 | MySQL PROCESSLIST | 连接池使用率 > 80% |
| 限流攻击 | Redis rate key 扫描 | 买家被频繁限流（疑似刷接口） |

**4 步链式推理诊断**：

```
发现异常 → 步骤1 紧急度排序 → 步骤2 原因分析 → 步骤3 验证计划 → 步骤4 综合诊断报告
```

每步有独立的 System Prompt，上一步输出喂给下一步，形成完整的推理链。

**设计原则**：Agent **只读不写** — 查 Redis 用 GET/EXISTS，查 MySQL 用 SELECT，永不做修改操作，确保生产环境安全。

### 5. 安全加固（6 项）

| 加固项 | 说明 |
|--------|------|
| 时间窗口双重校验 | 同时检查 status 和时间边界，防止过期商品被下单 |
| JWT 加 exp 声明 | Token 24h 过期，防止泄露后长期有效 |
| 订单归属校验 | 查订单时校验 buyer_id 或 seller_id，防止越权 |
| 防重 + 数据库兜底 | Redis SETNX + MySQL 事务内 COUNT 双重防重 |
| Redis 库存 key 存在性检查 | 扣库存前检查 key 是否存在，不存在从 MySQL 自动预热 |
| bcrypt 密码哈希 | 不存储明文密码 |

---

## 项目结构

```
GO/
├── README.md                          # 项目主文档（你正在看）
├── TODO.md                            # 待办清单
│
├── go-seckill/                        # Go 秒杀服务
│   ├── main.go                        #   入口：路由注册 + 中间件 + 启动
│   ├── go.mod / go.sum                #   Go 依赖
│   ├── config.env.example             #   配置模板
│   ├── handlers/                      #   HTTP 处理层
│   │   ├── auth.go                    #     买家/卖家注册登录
│   │   ├── buyer.go                   #     秒杀列表、下单、订单查询
│   │   ├── seller.go                  #     上架、改库存、接单、完成
│   │   └── middleware.go              #     JWT 认证/授权、CORS、日志
│   ├── database/                      #   数据操作层
│   │   ├── mysql.go                   #     MySQL 连接池 + 自动建表
│   │   ├── redis.go                   #     Redis 连接 + 库存/限流/防重
│   │   ├── queries.go                 #     CRUD + 事务下单
│   │   └── chat.go                    #     聊天数据操作
│   ├── chat/                          #   即时聊天
│   │   └── hub.go                     #     WebSocket Hub + Pub/Sub
│   ├── models/                        #   数据模型
│   │   └── struct.go                  #     所有结构体定义
│   ├── response/                      #   统一返回格式
│   │   └── response.go
│   └── docker/                        #   容器化部署
│       ├── Dockerfile                 #     多阶段构建
│       ├── docker-compose.yml         #     MySQL + Redis + App
│       ├── entrypoint.sh              #     健康检查等待脚本
│       └── mysql/conf.d/              #     MySQL 配置
│
└── seckill-guardian/                  # Python AI 运维 Agent
    ├── main.py                        #   主调度循环：注册探针 → 定时检查
    ├── requirements.txt               #   redis / pymysql / openai
    ├── config.env.example             #   配置模板（含 LLM API Key）
    ├── probe/                         #   探针模块（4 个）
    │   ├── base.py                    #     Alert 结构 + 探针基类
    │   ├── stock.py                   #     库存一致性探针
    │   ├── order_timeout.py           #     超时订单探针
    │   ├── connection_pool.py         #     连接池探针
    │   └── rate_limit.py             #     限流攻击探针
    ├── collector/                     #   上下文收集
    │   └── context.py                 #     系统快照补充
    ├── memory/                        #   滑动窗口记忆
    │   └── history.py                 #     10 轮历史 + 趋势分析
    ├── diagnoser/                     #   LLM 诊断
    │   └── llm.py                     #     4 步链式推理
    └── report/                        #   报告输出
        └── writer.py
```

---

## 运行 AI 运维 Agent

```bash
cd seckill-guardian

# 安装依赖
pip install -r requirements.txt

# 配置
cp config.env.example config.env
# 编辑 config.env：填入 MySQL/Redis 连接信息 + LLM API Key

# 运行（默认每 30s 巡检一次）
python main.py
```

---

## 后续规划

- [ ] 评价系统（用户打分、评论、卖家回复）
- [ ] 结构化日志（logrus + 日志分级）
- [ ] Prometheus + Grafana 监控面板
- [ ] 单元测试 + 压测脚本（wrk/vegeta）
- [ ] JWT 双 Token（Access + Refresh + 黑名单）
- [ ] 缓存穿透/雪崩防御（布隆过滤器 / 互斥锁）
