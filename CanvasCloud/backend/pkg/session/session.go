package session

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

//Cookie+Session 在服务端存储状态（Session 数据），通过 Cookie 传递 Session ID 进行关联，是有状态的
//储物柜比喻
//经典 Web 应用： 需要服务端维护丰富用户会话状态（购物车、复杂权限、频繁交互）。
//需要严格会话控制： 容易实现服务端主动使会话失效（删除 Session 存储即可）。
//同源策略内： Cookie 自动管理方便。

// 封装session函数
// 设置 Session 数据
func SetSession(c *gin.Context, key string, value interface{}) error {
	session := sessions.Default(c)
	session.Set(key, value)
	return session.Save()
}

// 获取 Session 数据
func GetSession(c *gin.Context, key string) interface{} {
	session := sessions.Default(c)
	return session.Get(key)
}

// 删除 Session 数据
func DeleteSession(c *gin.Context, key string) error {
	session := sessions.Default(c)
	session.Delete(key)
	return session.Save()
}

// 清空 Session 数据
func ClearSession(c *gin.Context) error {
	session := sessions.Default(c)
	session.Clear()
	return session.Save()
}
