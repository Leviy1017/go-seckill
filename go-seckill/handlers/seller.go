// 创建秒杀商品（CreateSeckillProduct）、修改库存。
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"go-seckill/database"
	"go-seckill/models"
	"go-seckill/response"
)

// ---- 上架秒杀商品 ----

type createProductReq struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	OriginalPrice float64 `json:"original_price"`
	SeckillPrice  float64 `json:"seckill_price"`
	Stock         int     `json:"stock"`
	SeckillStart  string  `json:"seckill_start"`
	SeckillEnd    string  `json:"seckill_end"`
}

// 登录请求认证
func SellerCreateProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持POST请求", nil)
		return
	}

	sellerID := r.Context().Value(ContextUserID).(int)

	var req createProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		return
	}
	if req.Name == "" || req.Stock <= 0 || req.SeckillPrice <= 0 {
		response.Error(w, http.StatusBadRequest, 4000, "商品名称、库存、秒杀价不能为空", nil)
		return
	}

	startTime, err := time.Parse("2006-01-02 15:04:05", req.SeckillStart)
	if err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "开始时间格式错误(需: 2006-01-02 15:04:05)", err)
		return
	}
	endTime, err := time.Parse("2006-01-02 15:04:05", req.SeckillEnd)
	if err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "结束时间格式错误", err)
		return
	}

	p := &models.SeckillProduct{
		SellerID:      sellerID,
		Name:          req.Name,
		Description:   req.Description,
		OriginalPrice: req.OriginalPrice,
		SeckillPrice:  req.SeckillPrice,
		Stock:         req.Stock,
		SeckillStart:  startTime,
		SeckillEnd:    endTime,
	}

	if err := database.CreateSeckillProduct(p); err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "创建商品失败", err)
		return
	}

	// 库存预热到 Redis
	database.WarmUpStock(p.ProductID, p.Stock)

	response.Created(w, "秒杀商品创建成功", p)
}

// ---- 调整库存 ----

type updateStockReq struct {
	ProductID int `json:"product_id"`
	NewStock  int `json:"new_stock"`
}

func SellerUpdateStock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持PUT请求", nil)
		return
	}

	sellerID := r.Context().Value(ContextUserID).(int)

	var req updateStockReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		return
	}
	if req.ProductID <= 0 || req.NewStock < 0 {
		response.Error(w, http.StatusBadRequest, 4000, "参数无效", nil)
		return
	}

	product, err := database.GetSeckillProductByID(req.ProductID)
	if err != nil {
		response.Error(w, http.StatusNotFound, 4004, "商品不存在", err)
		return
	}
	//更新日志: 卖家修改库存前校验商品是否属于自己，防止跨店铺操作
	if product.SellerID != sellerID {
		response.Error(w, http.StatusForbidden, 4003, "无权操作此商品", nil)
		return
	}

	if err := database.UpdateStock(req.ProductID, req.NewStock); err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "更新库存失败", err)
		return
	}

	// 同步 Redis
	database.WarmUpStock(req.ProductID, req.NewStock)

	response.Success(w, "库存更新成功", nil)
}

// ---- 确认接单 ----

type acceptOrderReq struct {
	OrderID string `json:"order_id"`
}

func SellerAcceptOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持POST请求", nil)
		return
	}

	sellerID := r.Context().Value(ContextUserID).(int)

	var req acceptOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		return
	}

	order, err := database.GetOrderByID(req.OrderID)
	if err != nil {
		response.Error(w, http.StatusNotFound, 4004, "订单不存在", err)
		return
	}
	if order.SellerID != sellerID {
		response.Error(w, http.StatusForbidden, 4003, "无权操作此订单", nil)
		return
	}
	//更新日志:防止卖家接到不属于自己家的订单  检查权限
	if order.OrderStatus != models.OrderStatusPaid {
		response.Error(w, http.StatusBadRequest, 4102, "订单状态不允许接单", nil)
		return
	}

	if err := database.UpdateOrderStatus(req.OrderID, models.OrderStatusAccepted); err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "更新订单状态失败", err)
		return
	}

	response.Success(w, "接单成功", nil)
}

// ---- 确认完成 ----

func SellerCompleteOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持POST请求", nil)
		return
	}

	sellerID := r.Context().Value(ContextUserID).(int)

	var req acceptOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		return
	}

	order, err := database.GetOrderByID(req.OrderID)
	if err != nil {
		response.Error(w, http.StatusNotFound, 4004, "订单不存在", err)
		return
	}
	//更新日志: 卖家完成订单前校验订单是否属于自己，防止跨店铺操作
	if order.SellerID != sellerID {
		response.Error(w, http.StatusForbidden, 4003, "无权操作此订单", nil)
		return
	}
	if order.OrderStatus != models.OrderStatusAccepted {
		response.Error(w, http.StatusBadRequest, 4102, "订单状态不允许完成", nil)
		return
	}

	if err := database.UpdateOrderStatus(req.OrderID, models.OrderStatusCompleted); err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "更新订单状态失败", err)
		return
	}

	response.Success(w, "订单已完成", nil)
}

// ---- 查看订单列表 ----

func SellerOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持GET请求", nil)
		return
	}

	sellerID := r.Context().Value(ContextUserID).(int)

	orders, err := database.GetSellerOrders(sellerID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, 5000, "查询订单失败", err)
		return
	}

	response.Success(w, "成功", orders)
}
