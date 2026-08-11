/*
auth功能总结:  (authentication  认证)
1.身份认证的"安检站"，所有接口都在做各种核验。

	买家注册 → 核验：方法、参数、手机号是否已被注册
	买家登录 → 核验：方法、参数、手机号是否存在、密码是否正确
	卖家注册 → 核验：方法、参数、手机号是否已被注册
	卖家登录 → 核验：方法、参数、手机号是否存在、密码是否正确
*/
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"go-seckill/database"
	"go-seckill/response"
)

// ---- 买家注册 ----

type buyerRegisterReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	Address  string `json:"address"`
}

func BuyerRegister(w http.ResponseWriter, r *http.Request) {
	//http.ResponseWriter 是一个接口，用于构建HTTP响应
	//HTTP响应是服务器返回给客户端的回复数据
	//*http.Request 包含客户端请求的所有信息
	//流程:客户端发送HTTP请求 服务端有HTTP接口层
	//再到业务逻辑层 再到数据访问层 数据库
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持POST请求", nil)
		return
	}
	//这个接口要求 r.Method 必须是 POST(提交/发送数据)
	//"用户请求的方法" != "服务器期望/要求的方法"
	//HTTP包自带方法 方法常量Method  状态码Status
	//响应w 请求r 上下文 Context
	// 这不是最终代码
	// 这是一个"模板/模式"
	// 实际开发中会替换成具体的方法
	var req buyerRegisterReq
	//创建一个空盒子，准备装客户端发来的数据
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		//json.NewDecoder(r.Body)  // 1. 创建一个JSON解码器，从 r.Body 读取数据
		//.Decode(&req) 2. 解码JSON数据，填充到 req 中
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		//http.StatusBadRequest - HTTP状态码
		//4000 - 业务错误码
		//"请求参数错误" - 错误信息
		//err - 错误详情
		return
	}
	if req.Username == "" || req.Password == "" || req.Phone == "" {
		response.Error(w, http.StatusBadRequest, 4000, "用户名、密码、手机号不能为空", nil)
		return
	}
	//检查客户端是否提供了必填数据

	buyer, err := database.CreateBuyer(req.Username, req.Password, req.Phone, req.Address)
	if err != nil {
		response.Error(w, http.StatusConflict, 4001, "注册失败，手机号可能已存在", err)
		return
	}

	token, _ := GenerateToken(buyer.BuyerID, "buyer")
	//生成一个JWT令牌，用于后续的身份认证
	//JWT（JSON Web Token）是身份认证的"通行证"
	//middleware中定义了GenerateToken函数
	response.Created(w, "买家注册成功", map[string]interface{}{
		"buyer": buyer,
		"token": token,
	})

}

// ---- 买家登录 ----

type loginReq struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

func BuyerLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持POST请求", nil)
		//response 包是你们项目自定义的工具包，用于统一HTTP响应的格式。
		// 它封装了常用的响应方法，让代码更简洁、更规范。
		return
	}

	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		return
	}

	buyer, err := database.GetBuyerByPhone(req.Phone) //queries中的函数
	if err != nil {
		if err == sql.ErrNoRows {
			response.Error(w, http.StatusUnauthorized, 4002, "手机号或密码错误", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, 5000, "服务器错误", err)
		return
	}

	if !database.CheckPassword(buyer.Password, req.Password) {
		response.Error(w, http.StatusUnauthorized, 4002, "手机号或密码错误", nil)
		return
	}

	token, _ := GenerateToken(buyer.BuyerID, "buyer")
	response.Success(w, "买家登录成功", map[string]interface{}{
		"buyer": buyer,
		"token": token,
	})
}

// ---- 卖家注册 ----

type sellerRegisterReq struct {
	ShopName string `json:"shop_name"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
	ShopAddr string `json:"shop_addr"`
}

func SellerRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		//Go Web开发中的标准方法检查模式 原理同上标识
		//如果没有检查  那么GET请求 即用户点击可能就会进行注册  也会影响安全问题
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持POST请求", nil)
		//方法不允许
		return
	}

	var req sellerRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		return
	}
	if req.ShopName == "" || req.Password == "" || req.Phone == "" {
		response.Error(w, http.StatusBadRequest, 4000, "店名、密码、手机号不能为空", nil)
		//请求错误
		return
	}

	seller, err := database.CreateSeller(req.ShopName, req.Password, req.Phone, req.ShopAddr)
	if err != nil {
		response.Error(w, http.StatusConflict, 4001, "注册失败，手机号可能已存在", err)
		return
	}

	token, _ := GenerateToken(seller.SellerID, "seller")
	response.Created(w, "卖家注册成功", map[string]interface{}{
		"seller": seller,
		"token":  token,
	})
}

// ---- 卖家登录 ----

func SellerLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, 4000, "仅支持POST请求", nil)
		return
	}

	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, 4000, "请求参数错误", err)
		return
	}
	//登录接口接收客户端数据的标准写法

	seller, err := database.GetSellerByPhone(req.Phone)
	if err != nil {
		if err == sql.ErrNoRows {
			response.Error(w, http.StatusUnauthorized, 4002, "手机号或密码错误", nil)
			return
		}
		response.Error(w, http.StatusInternalServerError, 5000, "服务器错误", err)
		return
	}

	if !database.CheckPassword(seller.Password, req.Password) {
		response.Error(w, http.StatusUnauthorized, 4002, "手机号或密码错误", nil)
		return
	}

	token, _ := GenerateToken(seller.SellerID, "seller")
	response.Success(w, "卖家登录成功", map[string]interface{}{
		"seller": seller,
		"token":  token,
	})
}
