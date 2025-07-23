package ecode

import (
	"fmt"
)

// HTTP状态码
// 常见：
// 400 BAD REQUEST
// 401 UNAUTHORIZED
// 403 FORBIDDEN
// 404 NOT FOUND
// 408 REQUEST TIME OUT
// 500 INTERNAL ERROR
// 五位数使得状态码可扩展
const (
	SUCCESS         = 0     //成功
	PARAMS_ERROR    = 40000 //请求参数错误
	NOT_LOGIN_ERROR = 40100 //未登录
	NO_AUTH_ERROR   = 40101 //无权限
	NOT_FOUND_ERROR = 40400 //请求数据不存在
	FORBIDDEN_ERROR = 40300 //禁止访问
	SYSTEM_ERROR    = 50000 //系统内部异常
	OPERATION_ERROR = 50001 //操作失败

)

// errMsgMap 错误码与错误信息映射
var errMsgMap = map[int]string{
	SUCCESS:         "成功",
	PARAMS_ERROR:    "请求参数错误",
	NOT_LOGIN_ERROR: "未登录",
	NO_AUTH_ERROR:   "无权限",
	NOT_FOUND_ERROR: "请求数据不存在",
	FORBIDDEN_ERROR: "禁止访问",
	SYSTEM_ERROR:    "系统内部异常",
	OPERATION_ERROR: "操作失败",
}

// 错误返回结构体，避免信息重复
type ErrorWithCode struct {
	Code int
	Msg  string
}

// GetErrMsg 获取错误信息
func GetErrMsg(code int) string {
	if msg, ok := errMsgMap[code]; ok {
		return msg
	}
	return "未知错误"
}

// GetErrWithDetail 返回带状态码的错误
func GetErrWithDetail(code int, msg string) *ErrorWithCode {
	return &ErrorWithCode{
		Code: code,
		Msg:  fmt.Sprintf("%s", msg),
	}
}
