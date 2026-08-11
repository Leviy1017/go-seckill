package response

import (
	"encoding/json"
	"net/http"
	"time"
)

// APIResponse 统一 API 返回格式
type APIResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// 把"业务数据"打包成"统一格式"的JSON响应
func Success(w http.ResponseWriter, message string, data interface{}) {
	writeJSON(w, http.StatusOK, APIResponse{
		Code:      0,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
	})
}

func Created(w http.ResponseWriter, message string, data interface{}) {
	writeJSON(w, http.StatusCreated, APIResponse{
		Code:      0,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
	})
}

func Error(w http.ResponseWriter, statusCode int, code int, message string, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	writeJSON(w, statusCode, APIResponse{
		Code:      code,
		Message:   message,
		Error:     errMsg,
		Timestamp: time.Now().Unix(),
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}
