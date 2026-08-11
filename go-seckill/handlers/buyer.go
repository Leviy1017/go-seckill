// 收到秒杀请求 → 调用 Redis 扣库存 → 成功再改 MySQL 库存。
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go-seckill/database"
	"go-seckill/response"
)

// ---- 查看秒杀商品列表 ----
// TODO: 可以使用连接池优化  短连接场景
func BuyerSeckillList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		//GET方法  查询数据
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持GET请求", nil)
		return
	}

	products, err := database.GetSeckillProducts() //调用数据库查询函数
	if err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "查询秒杀商品失败", err)
		return
	}

	response.Success(w, "成功", products)
}

// ---- 买家订单列表 ----

func BuyerOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持GET请求", nil)
		return
	}

	buyerID := r.Context().Value(ContextUserID).(int)

	orders, err := database.GetBuyerOrders(buyerID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "查询订单失败", err)
		return
	}

	response.Success(w, "成功", orders)
}

// ---- 查询订单状态 ----

func BuyerOrderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持GET请求", nil)
		return
	}

	orderID := r.URL.Query().Get("id")
	/*URL = 告诉计算机"去哪找"和"找什么"的地址
	id是URL地址中参数的值
	URL地址格式:https://www.example.com:8080/api/order/status?id=12345&page=1#section
	*/
	if orderID == "" {
		response.Error(w, http.StatusBadRequest, 4000, "缺少订单ID参数", nil)
		return
	}

	order, err := database.GetOrderByID(orderID)
	if err != nil {
		response.Error(w, http.StatusNotFound, 4004, "订单不存在", err)
		return
	}

	response.Success(w, "成功", order)
}

// ---- 下单（秒杀核心接口） ----

type orderReq struct {
	ProductID int `json:"product_id"`
}

func BuyerPlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持POST请求", nil)
		return
	}

	var req orderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		return
	}
	if req.ProductID <= 0 {
		response.Error(w, http.StatusBadRequest, 4000, "商品ID无效", nil)
		return
	}

	buyerID := r.Context().Value(ContextUserID).(int)
	stockKey := fmt.Sprintf("seckill:stock:%d", req.ProductID)

	// ---- 0. 限流：每秒最多 3 次 ----
	if !database.CheckRateLimit(buyerID, 3) {
		response.Error(w, http.StatusTooManyRequests, 4103, "请求过于频繁，请稍后再试", nil)
		return
	}

	// ---- 1. 查商品信息 ----
	product, err := database.GetSeckillProductByID(req.ProductID)
	if err != nil {
		response.Error(w, http.StatusNotFound, 4004, "秒杀商品不存在", err)
		return
	}

	// ---- 2. 检查秒杀状态 ----
	//更新日志:进行了对于时间边界性的验证 防止在时间外只靠状态来判断下单可行性
	now := time.Now()
	if product.Status != "active" || now.Before(product.SeckillStart) ||
		now.After(product.SeckillEnd) {
		// 顺便把状态纠正一下
		response.Error(w, http.StatusBadRequest, 4100, "当前不在秒杀时间内", nil)
		return
	}

	// ---- 3. 防重：同一买家同一商品只能抢一次 ----
	if !database.CheckDuplicateOrder(buyerID, req.ProductID) {
		response.Error(w, http.StatusOK, 4104, "你已提交过抢购请求，请勿重复操作", nil)
		return
	}
	defer database.ClearDuplicateFlag(buyerID, req.ProductID) // 不管成败都清理

	// ---- 4. 查买家、卖家信息 ----
	buyer, err := database.GetBuyerByID(buyerID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "获取买家信息失败", err)
		return
	}
	seller, err := database.GetSellerByID(product.SellerID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "获取卖家信息失败", err)
		return
	}

	// ---- 5. Redis 原子扣库存（第一道防线，最快的过滤） ----
	remain, err := database.DeductStock(req.ProductID, product.Stock)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "扣库存失败", err)
		return
	}
	if remain < 0 {
		database.RDB.Incr(database.Ctx, stockKey) // 扣多了，加回来
		response.Error(w, http.StatusOK, 4101, "已售罄", nil)
		return
	}

	// ---- 6. 数据库事务：扣库存 + 创建订单（第二道防线，强一致性） ----
	order, err := database.ProcessSeckillOrder(
		buyerID, req.ProductID, product.Name, product.SeckillPrice,
		buyer.Username, seller.ShopName, product.SellerID,
	)
	if err != nil {
		// 事务失败，回滚 Redis
		database.RDB.Incr(database.Ctx, stockKey)
		response.Error(w, http.StatusOK, 4101, "下单失败，请重试", err)
		return
	}

	response.Created(w, "下单成功", order)
}
