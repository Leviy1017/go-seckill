1.原子操作就是不可分割的操作，要么完全执行成功，要么完全不执行，中间不会被其他操作打断。

2.关于docker :
开发的秒杀系统需要三样东西才能跑起来：
  - MySQL（数据库）
  - Redis（缓存）
  - Go 程序（业务逻辑）
Docker 就像给每个服务打了个"集装箱"——把 MySQL、Redis、Go程序分别装进三个独立的箱子，箱子里软件和配置全打包好了。换服务器的时候直接搬箱子就行，不用重新装。说白了就是环境隔离+一键部署。

主流程不用等次要操作，这叫异步。比如下单之后发短信这事可以慢慢来，不用卡着用户等。
MQ就是消息队列，本质就是个排队系统。用了之后发送方和接收方互不依赖，发的人只管扔进去，收的人只管取出来，这叫解耦。

3.关于redis消息队列
现在互联网基本都用分布式架构，MQ已经是系统内部通信的核心了。
主要能力：解耦、可靠投递、广播、削峰、最终一致性这些。
主流MQ：RabbitMQ、Kafka、RocketMQ这些。Redis硬搞也能当MQ用，但消息不持久化，重启就没了，小项目凑合用还行。
核心模式就一句话：生产者往队列扔消息就完事了，消费者自己订阅着取，两边互不干扰，甚至都不用同时在线。

4.即时聊天系统

4.1 为什么用WebSocket不用HTTP？
HTTP是一问一答，客户端问一句服务端答一句，服务端没法主动推数据。
聊天需要服务端主动推消息过来，用HTTP只能轮询（不停问"有新消息吗"），99%的请求白费。
WebSocket先借HTTP握一次手，然后升级成长连接，双方随时互发消息，就像打电话vs发短信的区别。

4.2 核心架构
用户A ──WebSocket──→ 服务器 ──Redis Pub/Sub──→ 服务器 ──WebSocket──→ 用户B
                                    |
                               同时存MySQL

说白了三层：
- WebSocket管实时传输（打电话）
- Redis Pub/Sub管跨服务器转发（邮局寄信）
- MySQL管消息持久化（信留底，不能丢）

4.3 Hub连接池
Hub就是个酒店前台，干三件事：
- Register：入住登记，把用户和WebSocket连接记到登记簿（map）上
- Unregister：退房，从登记簿划掉
- SendToUser：转接电话，查登记簿找到人的连接，推消息过去

key用的是ClientKey{UserID, Role}而不是单纯int，因为buyers表和sellers表的ID各自自增，买家1号和卖家1号会撞，得加角色区分。

4.4 锁
connections这个map会被多个协程同时读写，Go的map不是并发安全的，不加锁会panic。
用的RWMutex读写锁：读可以共享（多人同时查登记簿没问题），写必须独占（改登记簿的时候别人不能看也不能改）。
忘了Unlock就死锁，所以用defer mu.Unlock()保证一定解锁。

4.5 发消息流程
handleChatMessage做的事：
1. 解析JSON拿到接收者和内容
2. FindOrCreateConversation——找或建会话（保证同一对人只有一个会话）
3. SaveMessage——存MySQL，先存库再推，保证消息不丢
4. SendToUser——本机能推就推
5. 推不到就走Redis Publish，让其他服务器推

4.6 离线消息
对方不在线？消息已经在MySQL里了。对方上线时pushOfflineMessages从库里拉未读消息补发，然后标记已读。

4.7 用户下线自动清理
浏览器一关，WebSocket连接断开，conn.ReadMessage()返回err，主循环break退出。
函数退出时defer自动执行：Unregister（从登记簿划掉）+ cancel（关Redis信箱订阅）。
整个清理是自动的，不用手动管。

4.8 浏览器WebSocket没法自定义Header
JWT认证需要Authorization头，但浏览器new WebSocket(url)设不了Header。
解决办法：token从URL参数传 ws://host/api/chat/ws?token=xxx，服务器取出来塞进Header，再走正常认证流程。

4.9 历史消息用HTTP接口
WebSocket只管实时推送，查历史记录走普通HTTP：
- GET /api/chat/conversations  会话列表
- GET /api/chat/messages?conversation_id=1  历史消息
这俩就是正常的增删改查，跟秒杀那些接口一个套路。
