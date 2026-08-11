package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"go-seckill/chat"
	"go-seckill/database"
	"go-seckill/handlers"
)

func main() {
	handlers.InitJWT()

	// 初始化数据库
	if err := database.InitMySQL(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		log.Fatalf("数据表迁移失败: %v", err)
	}

	// 初始化 Redis
	if err := database.InitRedis(); err != nil {
		log.Fatalf("Redis 初始化失败: %v", err)
	}

	// 注册路由
	mux := http.NewServeMux()

	// ---- 公开端点 ----
	route(mux, "POST /api/auth/buyer/register", handlers.BuyerRegister, "")
	route(mux, "POST /api/auth/buyer/login", handlers.BuyerLogin, "")
	route(mux, "POST /api/auth/seller/register", handlers.SellerRegister, "")
	route(mux, "POST /api/auth/seller/login", handlers.SellerLogin, "")

	// ---- 买家端点 ----
	route(mux, "GET /api/buyer/seckill", handlers.BuyerSeckillList, "buyer")
	route(mux, "GET /api/buyer/orders", handlers.BuyerOrders, "buyer")
	route(mux, "GET /api/buyer/order/status", handlers.BuyerOrderStatus, "buyer")
	route(mux, "POST /api/buyer/order", handlers.BuyerPlaceOrder, "buyer")

	// ---- 卖家端点 ----
	route(mux, "POST /api/seller/product", handlers.SellerCreateProduct, "seller")
	route(mux, "PUT /api/seller/product/stock", handlers.SellerUpdateStock, "seller")
	route(mux, "POST /api/seller/order/accept", handlers.SellerAcceptOrder, "seller")
	route(mux, "POST /api/seller/order/complete", handlers.SellerCompleteOrder, "seller")
	route(mux, "GET /api/seller/orders", handlers.SellerOrders, "seller")

	// ---- 聊天端点（买家和卖家都能用，所以用 "any" 表示任意已登录用户）----
	// WebSocket 连接：客户端通过 ws://host/api/chat/ws 连接
	wsRoute(mux, "/api/chat/ws", chat.HandleWebSocket)
	// HTTP 接口：获取会话列表
	route(mux, "GET /api/chat/conversations", chat.HandleGetConversations, "any")
	// HTTP 接口：获取会话历史消息
	route(mux, "GET /api/chat/messages", chat.HandleGetMessages, "any")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("服务启动于端口 :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// route 统一路由注册
// role: "" = 无需登录, "buyer" = 买家认证, "seller" = 卖家认证, "any" = 任意已登录用户
func route(mux *http.ServeMux, pattern string, handler http.HandlerFunc, role string) {
	parts := strings.SplitN(pattern, " ", 2)
	method := parts[0]
	path := parts[1]

	// 根据角色包装认证中间件
	var h http.HandlerFunc
	switch role {
	case "buyer":
		h = handlers.BuyerOnly(handler)
	case "seller":
		h = handlers.SellerOnly(handler)
	case "any":
		h = handlers.AuthMiddleware(handler) // 任意已登录用户，不限制角色
	default:
		h = handler
	}

	// 包装通用中间件
	h = handlers.CORSMiddleware(handlers.LoggingMiddleware(h))

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte(`{"code":405,"message":"请求方法不允许"}`))
			return
		}
		h(w, r)
	})
}

// wsRoute WebSocket 路由注册（不走 method 检查，WebSocket 用 GET 升级）
// 浏览器 WebSocket 不能自定义 Header，所以 token 通过 URL 参数传递：
//   ws://host/api/chat/ws?token=xxx
func wsRoute(mux *http.ServeMux, path string, handler http.HandlerFunc) {
	h := func(w http.ResponseWriter, r *http.Request) {
		// 从 query 参数中提取 token
		token := r.URL.Query().Get("token")
		if token != "" {
			// 把 token 放到 Header 里，让 AuthMiddleware 正常工作
			r.Header.Set("Authorization", "Bearer "+token)
		}
		handlers.AuthMiddleware(handler)(w, r)
	}
	h = handlers.CORSMiddleware(h)
	mux.HandleFunc(path, h)
}
