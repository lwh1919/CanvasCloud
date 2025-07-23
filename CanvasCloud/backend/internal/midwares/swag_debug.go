// midwares/swagger_debug.go
package midwares

import (
	"github.com/gin-contrib/sessions"
	"log"
	"os"
	"web_app2/internal/model/entity"

	"github.com/gin-gonic/gin"
)

func SwaggerSessionDebug() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅在开发环境启用
		if os.Getenv("GIN_MODE") != "release" {
			// 打印请求中的 Cookie 信息
			cookie, err := c.Cookie("gsessionid")
			if err == nil {
				log.Printf("[Swagger Debug] Cookie: gsessionid=%s", cookie)
			} else {
				log.Printf("[Swagger Debug] No gsessionid cookie found")
			}

			// 打印 Session 信息
			session := sessions.Default(c)
			user := session.Get("user_login")
			userObj, _ := user.(entity.User)

			log.Printf("[Swagger Debug] Session user_id: %v Session user_account: %s", userObj.ID, userObj.UserAccount)

		}
		c.Next()
	}
}
