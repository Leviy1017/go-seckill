# 已完成

- [x] MySQL + Redis 数据库初始化与连接池
- [x] 买家/卖家注册、登录、JWT 认证与角色授权
- [x] 卖家上架秒杀商品、修改库存
- [x] 买家查看秒杀列表、下单（Redis 扣库存 + MySQL 双保险）
- [x] 订单全流程（买家下单 → 卖家接单 → 完成）
- [x] 防刷机制（限流 + 防重 + 时间窗口校验 + 数据库兜底）
- [x] 即时聊天系统（WebSocket + Redis Pub/Sub + MySQL 持久化 + 离线摘要）
- [x] Docker Compose 容器化部署（MySQL + Redis + App + healthcheck）
- [x] 安全加固 6 项（时间校验、JWT exp、订单归属、防重兜底、key 存在性检查、bcrypt）
- [x] Python AI 运维 Agent（4 探针 + LLM 链式推理诊断）

# 待完成

- [ ] 评价系统（商品评分、评论、卖家回复）
- [ ] 结构化日志（logrus + 日志分级）
- [ ] Prometheus /metrics 监控端点
- [ ] 单元测试 + 压测脚本
- [ ] JWT 双 Token 机制（Access + Refresh + 黑名单）
- [ ] 缓存穿透/雪崩防御（布隆过滤器）
