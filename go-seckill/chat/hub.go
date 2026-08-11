// 聊天系统：WebSocket + Redis Pub/Sub
// 架构就三层：
//
//	WebSocket管实时传输，Redis Pub/Sub管跨服务器转发，MySQL管消息不丢
//
// 流程：
//
//	用户A发消息 → 存MySQL → 本机能推就推，推不到就走Redis → 对方服务器从Redis取到 → 推给用户B
//	对方不在线？消息在MySQL里存着呢，上线时补发
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"go-seckill/database"
	"go-seckill/handlers"
	"go-seckill/models"
	"go-seckill/response"
)

// 升级器：HTTP→WebSocket，CheckOrigin return true是开发环境放行所有跨域，上线得改
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// 连接池相关

// ClientKey 为什么不直接用int做key？因为buyers表和sellers表的ID各自自增，买家1号和卖家1号会撞
type ClientKey struct {
	UserID int
	Role   string
}

// Hub 就是酒店前台，管谁在线、谁的管子是哪根
type Hub struct {
	connections map[ClientKey]*websocket.Conn // 登记簿：谁→哪根管子
	mu          sync.RWMutex                  // 读写锁，Go的map不是并发安全的，不加锁会panic
}

// 整个程序就这一个Hub，谁都用它
var GlobalHub = &Hub{
	connections: make(map[ClientKey]*websocket.Conn),
}

// Register 入住：把连接写进登记簿。同一账号新连接顶掉旧连接，不然推消息不知道推给哪根管子
func (h *Hub) Register(key ClientKey, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 旧连接还在就先关掉，一个人只留一根管子
	//我故意选了单人单设备因为在抢购场景下不需要多设备同步  也可以进行多设备同时登录的设定
	if old, ok := h.connections[key]; ok {
		old.Close()
	}
	h.connections[key] = conn
	log.Printf("[Chat] 用户上线: %d(%s), 当前在线: %d", key.UserID, key.Role, len(h.connections))
}

// Unregister 退房：划掉登记簿+拔管子
func (h *Hub) Unregister(key ClientKey) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conn, ok := h.connections[key]; ok {
		conn.Close()
		delete(h.connections, key)
		log.Printf("[Chat] 用户下线: %d(%s), 当前在线: %d", key.UserID, key.Role, len(h.connections))
	}
}

// SendToUser 转接电话：查登记簿找到人→往管子里灌数据。找不到return false，调用方走Redis
func (h *Hub) SendToUser(key ClientKey, msg []byte) bool {
	h.mu.RLock()
	conn, ok := h.connections[key]
	h.mu.RUnlock()

	if !ok {
		return false // 不在登记簿上，不在线
	}

	// 写锁是因为并发写同一根管子会乱
	h.mu.Lock()
	defer h.mu.Unlock()
	err := conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		log.Printf("[Chat] 推送失败: %d(%s)", key.UserID, key.Role)
		return false
	}
	return true
}

// 消息格式

// WSMessage 信封：type标明什么消息，data是信的内容
type WSMessage struct {
	Type string          `json:"type"` // 消息类型：chat=聊天, history=历史, conversations=会话列表
	Data json.RawMessage `json:"data"` // 消息体，不同type对应不同结构
}

// ChatData 发消息时的格式
type ChatData struct {
	ReceiverID   int    `json:"receiver_id"`   // 接收者ID
	ReceiverRole string `json:"receiver_role"` // 接收者角色 buyer/seller
	Content      string `json:"content"`       // 消息内容
}

// IncomingPush 收消息时的格式，比ChatData多了message_id、sent_at这些服务端补充的字段
type IncomingPush struct {
	MessageID      int    `json:"message_id"`
	SenderID       int    `json:"sender_id"`
	SenderRole     string `json:"sender_role"`
	Content        string `json:"content"`
	ConversationID int    `json:"conversation_id"`
	SentAt         string `json:"sent_at"`
}

// UnreadSummary 未读消息摘要：上线时只推这个，不逐条轰炸
type UnreadSummary struct {
	Total         int                  `json:"total"`         // 总未读数
	Conversations []UnreadConversation `json:"conversations"` // 每个会话的未读情况
}

// UnreadConversation 单个会话的未读摘要  类似主流软件的推送机制  杜绝消息轰炸
type UnreadConversation struct {
	ConversationID int    `json:"conversation_id"` // 会话 ID
	OtherUserID    int    `json:"other_user_id"`   // 对方 ID
	OtherUserRole  string `json:"other_user_role"` // 对方角色
	UnreadCount    int    `json:"unread_count"`    // 几条没读
	LastMessage    string `json:"last_message"`    // 最后一条说了啥
}

// Redis订阅：每个用户一个信箱，别的服务器往里投信，这边守着取
func SubscribeUser(ctx context.Context, key ClientKey) {
	channel := fmtChannel(key)
	sub := database.RDB.Subscribe(ctx, channel)
	defer sub.Close()

	ch := sub.Channel()
	log.Printf("[Chat] 订阅Redis频道: %s", channel)

	for {
		select {
		case <-ctx.Done():
			// 用户下线了，ctx发信号，别守了
			log.Printf("[Chat] 取消订阅: %s", channel)
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			// Redis来信了，推给本机这个用户
			GlobalHub.SendToUser(key, []byte(msg.Payload))
		}
	}
}

// fmtChannel 频道名规则：chat:角色:ID，比如chat:buyer:1
func fmtChannel(key ClientKey) string {
	return "chat:" + key.Role + ":" + intToStr(key.UserID)
}

func intToStr(n int) string {
	return fmt.Sprintf("%d", n)
}

// 主流程：用户连上来要干的事
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 1. 认人：JWT中间件已经把用户信息塞进Context了，取出来
	userID := r.Context().Value(handlers.ContextUserID).(int)
	role := r.Context().Value(handlers.ContextUserRole).(string)
	key := ClientKey{UserID: userID, Role: role}

	// 2. HTTP升级成WebSocket，这行执行完conn就诞生了
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Chat] WebSocket升级失败: %v", err)
		return
	}

	// 3. 入住登记
	GlobalHub.Register(key, conn)
	defer GlobalHub.Unregister(key) // 函数退了自动退房

	// 4. 开个信箱，别的服务器发来的消息从这取
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 函数退了自动关信箱
	go SubscribeUser(ctx, key)

	// 5. 补发离线时候没收到的消息
	pushOfflineMessages(key)

	// 6. 主循环：蹲着等用户发消息
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			// 连接断了，跳出循环，下面的defer会自动清理
			break
		}

		// 解析JSON
		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			continue
		}

		// 按type分发，目前只有chat类型
		switch wsMsg.Type {
		case "chat":
			handleChatMessage(key, wsMsg.Data)
		}
	}
}

// handleChatMessage 发消息的核心逻辑：存库→推送
/* Redis Pub/Sub 的本质：广播。
10 台服务器，用户 2 只连在其中 1 台上。Publish 一发，10 台全收到，9 台发现
connections 里没这个人，白干。
解释了为什么大公司聊天系统不直接用 Redis
Pub/Sub——不是功能做不到，是成本兜不住。
*/
func handleChatMessage(senderKey ClientKey, data json.RawMessage) {
	var chatData ChatData
	if err := json.Unmarshal(data, &chatData); err != nil {
		return
	}
	if chatData.Content == "" || chatData.ReceiverID <= 0 || chatData.ReceiverRole == "" {
		return
	}

	receiverKey := ClientKey{UserID: chatData.ReceiverID, Role: chatData.ReceiverRole}

	// 1. 找或建会话，同一对人只会有一个会话
	conv, err := database.FindOrCreateConversation(senderKey.UserID, senderKey.Role, receiverKey.UserID, receiverKey.Role)
	if err != nil {
		log.Printf("[Chat] 创建会话失败: %v", err)
		return
	}

	// 2. 先存MySQL，推不推得出去再说，反正消息已经落库了不会丢
	msg, err := database.SaveMessage(conv.ConversationID, senderKey.UserID, senderKey.Role, receiverKey.UserID, receiverKey.Role, chatData.Content)
	if err != nil {
		log.Printf("[Chat] 保存消息失败: %v", err)
		return
	}

	// 3. 打包推送内容
	push := IncomingPush{
		MessageID:      msg.MessageID,
		SenderID:       senderKey.UserID,
		SenderRole:     senderKey.Role,
		Content:        chatData.Content,
		ConversationID: conv.ConversationID,
		SentAt:         msg.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	pushBytes, _ := json.Marshal(WSMessage{
		Type: "chat",
		Data: mustMarshal(push),
	})

	// 4. 本机能推就推
	sent := GlobalHub.SendToUser(receiverKey, pushBytes)

	// 5. 推不到说明对方不在这台机器上，扔给Redis让别的服务器推
	if !sent {
		database.RDB.Publish(database.Ctx, fmtChannel(receiverKey), string(pushBytes))
	}
}

// pushOfflineMessages 上线时发未读消息摘要，不逐条推送
// 改前：每个会话拉 50 条消息，逐条推 → 2500 条消息轰炸
// 改后：只发一个摘要 "张三发来 5 条消息，李四发来 12 条消息"
// 用户点进具体会话时通过 HTTP 接口 GetMessages 拉详情
func pushOfflineMessages(key ClientKey) {
	convs, err := database.GetUserConversations(key.UserID, key.Role)
	if err != nil {
		return
	}

	var total int                        //记录几条未读
	var unreadConvs []UnreadConversation //装摘要数据

	for _, conv := range convs {
		count, err := database.GetUnreadCount(key.UserID, key.Role, conv.ConversationID)
		if err != nil || count == 0 {
			continue
		}

		otherID, otherRole := getOtherUser(key, conv)

		unreadConvs = append(unreadConvs, UnreadConversation{
			ConversationID: conv.ConversationID,
			OtherUserID:    otherID,
			OtherUserRole:  otherRole,
			UnreadCount:    count,
			LastMessage:    conv.LastMessage,
		})
		total += count
	}

	if total == 0 {
		return
	}

	pushBytes, _ := json.Marshal(WSMessage{ //WSMessage 是聊天系统里所有 WebSocket 消息的统一格式
		Type: "unread_summary",
		Data: mustMarshal(UnreadSummary{
			Total:         total,
			Conversations: unreadConvs,
		}),
	})
	GlobalHub.SendToUser(key, pushBytes)
	// 注意：不调用 MarkMessagesRead
	// 用户只是收到了通知，并没有真正点开看消息
	// 真正标记已读的时机在 HandleGetMessages（打开聊天页面时）
}

// getOtherUser 从会话里提取"对方"的信息
func getOtherUser(key ClientKey, conv models.Conversation) (int, string) {
	if conv.User1ID == key.UserID && conv.User1Role == key.Role {
		return conv.User2ID, conv.User2Role
	}
	return conv.User1ID, conv.User1Role
}

// HTTP接口，查历史消息用的，不走WebSocket

// HandleGetConversations 查会话列表
func HandleGetConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持GET请求", nil)
		return
	}

	userID := r.Context().Value(handlers.ContextUserID).(int)
	role := r.Context().Value(handlers.ContextUserRole).(string)

	convs, err := database.GetUserConversations(userID, role)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "查询会话失败", err)
		return
	}

	response.Success(w, "成功", convs)
}

// HandleGetMessages 查某个会话的历史消息
func HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持GET请求", nil)
		return
	}

	userID := r.Context().Value(handlers.ContextUserID).(int)
	role := r.Context().Value(handlers.ContextUserRole).(string)
	convID := r.URL.Query().Get("conversation_id")
	if convID == "" {
		response.Error(w, http.StatusBadRequest, 4000, "缺少conversation_id", nil)
		return
	}

	var cid int
	fmt.Sscanf(convID, "%d", &cid)

	// 看了就标已读
	database.MarkMessagesRead(userID, role, cid)

	messages, err := database.GetConversationMessages(cid, 50, 0)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "查询消息失败", err)
		return
	}

	response.Success(w, "成功", messages)
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
