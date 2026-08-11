// 接口鉴权：访问需要登录的接口先走中间件校验 JWT
// 没带 token /token 失效 → 直接拦截返回未登录
// 校验通过才放行进入 buyer/seller 业务函数
// JWT 是 Token 的一种具体实现方式。
package handlers

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"go-seckill/response"
)

type contextKey string //

const (
	ContextUserID   contextKey = "user_id"
	ContextUserRole contextKey = "user_role"
) //

var jwtSecret []byte

func InitJWT() {
	//读取环境变量  os操作系统    os.Getenv 是 Go 语言中读取环境变量的函数
	secret := os.Getenv("JWT_SECRET") //哪来的
	if secret == "" {
		secret = "go-seckill-secret-key-2024"
	}
	jwtSecret = []byte(secret)
}

// GenerateToken 生成 JWT token
func GenerateToken(userID int, role string) (string, error) {
	//MapClaims 是 map[string]interface{} 的类型别名   相当于设置map的接口
	//放键值对数据  取出时需要类型断言
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	//更新日志: 设置了exp防止Token永远不会作废  一次登录有效期一天
	/*创建JWT TOKEN核心操作
	jwt.NewWithClaims 是 github.com/golang-jwt/jwt/v4 包提供的函数，用来创建 JWT Token。
	代表 HS256 签名算法
	claims 是 JWT 的负载（Payload）部分 存放你想要传递的用户信息
	*/
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	//给 Token 签名，然后返回一个字符串。
	return token.SignedString(jwtSecret)
}

// ParseToken 解析 JWT token 就是把客户端传来的 JWT 字符串还原成可读的数据，并验证它是否有效
func ParseToken(tokenStr string) (jwt.MapClaims, error) {
	//jwt.Parse 是 JWT 库提供的解析和验证 Token 的函数
	//func Parse(tokenString string, keyFunc Keyfunc) (*Token, error)
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	//
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// AuthMiddleware JWT 认证中间件
// 这是一个身份验证中间件，用于保护需要登录才能访问的接口。
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Error(w, http.StatusUnauthorized, 1001, "未提供认证Token", nil)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Error(w, http.StatusUnauthorized, 1001, "Token格式错误", nil)
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil {
			response.Error(w, http.StatusUnauthorized, 1001, "Token无效或已过期", nil)
			return
		}

		userID := int(claims["user_id"].(float64))
		role := claims["role"].(string)
		//context.WithValue 就是往 Context 里存数据的函数
		//每一个ctx都是初始的 r.Context 加上存入的数据
		//ctx存储在*http.request对象里
		ctx := context.WithValue(r.Context(), ContextUserID, userID)
		ctx = context.WithValue(ctx, ContextUserRole, role)
		//这行代码是把带着用户信息的请求，传递给下一个处理函数。
		next(w, r.WithContext(ctx))
	}
}

// BuyerOnly 仅允许买家访问
func BuyerOnly(next http.HandlerFunc) http.HandlerFunc {
	return AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		role := r.Context().Value(ContextUserRole).(string)
		if role != "buyer" {
			response.Error(w, http.StatusForbidden, 1002, "需要买家权限", nil)
			return
		}
		next(w, r)
	})
}

// SellerOnly 仅允许卖家访问
func SellerOnly(next http.HandlerFunc) http.HandlerFunc {
	return AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		role := r.Context().Value(ContextUserRole).(string)
		if role != "seller" {
			response.Error(w, http.StatusForbidden, 1002, "需要卖家权限", nil)
			return
		}
		next(w, r)
	})
}

// CORSMiddleware 跨域处理
func CORSMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// LoggingMiddleware 请求日志
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next(w, r)
	}
}
