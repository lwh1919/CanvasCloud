package common

//记录用于构造和标准化返回给客户端数据的结构体代码，它负责处理“返回给用户什么格式的数据”。

import (
	"net/http"
	"web_app2/internal/ecode"

	"github.com/gin-gonic/gin"
)

// 统一响应
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data" swaggertype:"object"`
}

func BaseResponse(c *gin.Context, data interface{}, msg string, code int) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}
func Success(c *gin.Context, data interface{}) {
	BaseResponse(c, data, "", 0)
}

// 失败响应
func Error(c *gin.Context, code int) {
	BaseResponse(c, nil, ecode.GetErrMsg(code), code)
}
