package midwares

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"io"
	"log"
	"strconv"
	"strings"
	"backend/internal/common"
	"backend/internal/ecode"
	"backend/internal/service"
	"backend/pkg/casbin"
)

// CasbinAuthCheck 中间件鉴权函数，必须需要登录
// Dom: 访问的资源域，对于公共图库是public，对于特定的空间是space，具体的空间ID会从请求中提取
// Obj: 访问的资源对象，目前有picture和spaceUser两种
// Act: 访问的行为，对于picture有upload/delete/view/edit，对于spaceUser有manage
func CasbinAuthCheck(Dom, Obj, Act string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始鉴权日志
		log.Printf("[CasbinAuth] 开始鉴权: Dom=%s, Obj=%s, Act=%s", Dom, Obj, Act)
		log.Printf("[CasbinAuth] 请求路径: %s %s", c.Request.Method, c.Request.URL.Path)

		// 获取用户服务
		userService := service.NewUserService()

		// 获取当前登录对象
		loginUser, err := userService.GetLoginUser(c)
		if err != nil {
			log.Printf("[CasbinAuth] 获取登录用户失败: %s (代码: %d)", err.Msg, err.Code)
			common.BaseResponse(c, nil, err.Msg, err.Code)
			c.Abort()
			return
		}
		log.Printf("[CasbinAuth] 登录用户: ID=%d, 用户名=%s", loginUser.ID, loginUser.UserAccount)

		// 获取sub，用户ID
		sub := "user_" + fmt.Sprintf("%d", loginUser.ID)
		log.Printf("[CasbinAuth] 构建sub: %s", sub)

		// 若Dom是space，则需要从请求中获取空间ID
		if Dom == "space" {
			log.Printf("[CasbinAuth] 处理space域鉴权")

			// 复制请求体
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// 解析请求体
			var OriginBodyMap map[string]interface{}
			if OriginErr := json.Unmarshal(bodyBytes, &OriginBodyMap); OriginErr != nil {
				log.Printf("[CasbinAuth] 请求体解析失败: %v", OriginErr)
				common.BaseResponse(c, nil, "请求体解析失败", ecode.SYSTEM_ERROR)
				c.Abort()
				return
			}

			// 记录原始请求体
			log.Printf("[CasbinAuth] 原始请求体: %s", string(bodyBytes))

			// 将ID字段转换为uint64
			bodyMap := make(map[string]interface{})
			for k, v := range OriginBodyMap {
				// 使用小写匹配字段名
				keyLower := strings.ToLower(k)
				if keyLower == "id" || keyLower == "spaceid" {
					// 处理不同类型的ID值
					var idStr string
					switch val := v.(type) {
					case string:
						idStr = val
					case float64: // JSON数字默认是float64
						idStr = strconv.FormatUint(uint64(val), 10)
					case int:
						idStr = strconv.Itoa(val)
					default:
						log.Printf("[CasbinAuth] 无法处理的ID类型: %T", v)
						continue
					}

					idUint64, err := strconv.ParseUint(idStr, 10, 64)
					if err == nil {
						bodyMap[k] = idUint64
						log.Printf("[CasbinAuth] 转换ID: %s -> %d", idStr, idUint64)
					} else {
						log.Printf("[CasbinAuth] ID转换失败: %s -> %v", idStr, err)
					}
				}
			}

			// 尝试直接获取spaceId
			spaceId, ok := bodyMap["spaceId"]
			if !ok {
				// 尝试小写字段名
				spaceId, ok = bodyMap["spaceid"]
			}

			if ok {
				Dom = fmt.Sprintf("%s_%v", Dom, spaceId)
				log.Printf("[CasbinAuth] 从请求体获取spaceId: %v, 构建Dom: %s", spaceId, Dom)
			} else {
				// 若Obj是spaceUser，那么需要从数据库根据Id找到对应的记录获取spaceId
				if Obj == "spaceUser" {
					log.Printf("[CasbinAuth] 处理spaceUser对象")

					// 获取空间成员ID
					spaceUserId, ok := bodyMap["id"]
					if !ok {
						// 尝试大写字段名
						spaceUserId, ok = bodyMap["Id"]
					}

					if !ok {
						log.Printf("[CasbinAuth] 请求体中缺少spaceUser ID")
						common.BaseResponse(c, nil, "请求体中缺少spaceUser ID", ecode.SYSTEM_ERROR)
						c.Abort()
						return
					}

					// 获取空间成员Service
					spaceUserService := service.NewSpaceUserService()

					// 根据ID查询空间成员信息
					spaceUserInfo, err := spaceUserService.GetSpaceUserById(spaceUserId.(uint64))
					if err != nil {
						log.Printf("[CasbinAuth] 获取空间成员信息失败: %s (代码: %d)", err.Msg, err.Code)
						common.BaseResponse(c, nil, "获取空间成员信息失败", ecode.SYSTEM_ERROR)
						c.Abort()
						return
					}

					// 获取空间ID
					Dom = fmt.Sprintf("%s_%d", Dom, spaceUserInfo.SpaceID)
					log.Printf("[CasbinAuth] 从数据库获取spaceId: %d, 构建Dom: %s", spaceUserInfo.SpaceID, Dom)
				} else {
					log.Printf("[CasbinAuth] 请求体中缺少space ID")
					common.BaseResponse(c, nil, "请求体中缺少space ID", ecode.SYSTEM_ERROR)
					c.Abort()
					return
				}
			}
		}

		// 获取casbin鉴权中间件
		casMethod := casbin.LoadCasbinMethod()

		// 记录最终鉴权参数
		log.Printf("[CasbinAuth] 最终鉴权参数: sub=%s, dom=%s, obj=%s, act=%s", sub, Dom, Obj, Act)

		// 判断是否有权限
		ok, originErr := casMethod.Enforcer.Enforce(sub, Dom, Obj, Act)
		if originErr != nil {
			log.Printf("[CasbinAuth] 权限校验出错: %v", originErr)
			common.BaseResponse(c, nil, "权限校验出错", ecode.SYSTEM_ERROR)
			c.Abort()
			return
		}

		if !ok {
			log.Printf("[CasbinAuth] 没有权限: sub=%s, dom=%s, obj=%s, act=%s", sub, Dom, Obj, Act)
			common.BaseResponse(c, nil, "没有权限", ecode.NO_AUTH_ERROR)
			c.Abort()
			return
		}

		log.Printf("[CasbinAuth] 权限通过: sub=%s, dom=%s, obj=%s, act=%s", sub, Dom, Obj, Act)

		// 权限通过，放行
		c.Next()
	}
}
