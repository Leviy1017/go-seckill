package models

import "time"

// Buyer 买家
type Buyer struct {
	BuyerID    int       `json:"buyer_id"`
	Username   string    `json:"username"`
	Password   string    `json:"-"`   //json:"-" 表示这个字段不会出现在 API 返回结果中（比如密码 Password string \json:"-"`） 
	Phone      string    `json:"phone"`
	Address    string    `json:"address"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Seller 卖家
type Seller struct {
	SellerID   int       `json:"seller_id"`
	ShopName   string    `json:"shop_name"`
	Password   string    `json:"-"`
	Phone      string    `json:"phone"`
	ShopAddr   string    `json:"shop_addr"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SeckillProduct 秒杀商品
type SeckillProduct struct {
	ProductID       int       `json:"product_id"`
	SellerID        int       `json:"seller_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	OriginalPrice   float64   `json:"original_price"`
	SeckillPrice    float64   `json:"seckill_price"`
	Stock           int       `json:"stock"`
	SeckillStart    time.Time `json:"seckill_start"`
	SeckillEnd      time.Time `json:"seckill_end"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SeckillOrder 秒杀订单
type SeckillOrder struct {
	OrderID      string    `json:"order_id"`
	BuyerID      int       `json:"buyer_id"`
	SellerID     int       `json:"seller_id"`
	ProductID    int       `json:"product_id"`
	ProductName  string    `json:"product_name"`
	SeckillPrice float64   `json:"seckill_price"`
	OrderStatus  string    `json:"order_status"`
	BuyerName    string    `json:"buyer_name"`
	SellerName   string    `json:"seller_name"`
	OrderTime    time.Time `json:"order_time"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// 订单状态常量
const (
	OrderStatusPaid      = "paid"
	OrderStatusAccepted  = "accepted"
	OrderStatusCompleted = "completed"
)

// 商品状态常量
const (
	ProductStatusPending = "pending"
	ProductStatusActive  = "active"
	ProductStatusEnded   = "ended"
	ProductStatusSoldOut = "sold_out"
)

// Conversation 会话：记录两个人之间的聊天关系
type Conversation struct {
	ConversationID int       `json:"conversation_id"`
	User1ID        int       `json:"user1_id"`         // 较小的用户ID
	User1Role      string    `json:"user1_role"`       // "buyer" 或 "seller"
	User2ID        int       `json:"user2_id"`         // 较大的用户ID
	User2Role      string    `json:"user2_role"`       // "buyer" 或 "seller"
	LastMessage    string    `json:"last_message"`     // 最后一条消息内容
	LastMessageAt  time.Time `json:"last_message_at"`  // 最后消息时间
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	MessageID      int       `json:"message_id"`
	ConversationID int       `json:"conversation_id"`
	SenderID       int       `json:"sender_id"`
	SenderRole     string    `json:"sender_role"`      // "buyer" 或 "seller"
	ReceiverID     int       `json:"receiver_id"`
	ReceiverRole   string    `json:"receiver_role"`    // "buyer" 或 "seller"
	Content        string    `json:"content"`
	ReadAt         *time.Time `json:"read_at"`         // nil = 未读
	CreatedAt      time.Time `json:"created_at"`
}
