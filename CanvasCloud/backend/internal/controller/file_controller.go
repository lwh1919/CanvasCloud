package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"path"
	"web_app2/internal/common"
	"web_app2/internal/ecode"
	"web_app2/internal/manager"
	"web_app2/pkg/tcos"
)

func TestUploadPicture(c *gin.Context) {
	file, _ := c.FormFile("file")
	manager.UploadPicture(file, "test")

}

// 文件测试上传

// TestUploadFile godoc
// @Summary      测试文件上传接口「管理员」
// @Tags         file
// @Accept       mpfd
// @Produce      json
// @Param        file formData file true "文件"
// @Success      200  {object}  common.Response{data=string} "响应文件存储在COS的KEY"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/file/test/upload [POST]
func TestUploadFile(c *gin.Context) {

	file, _ := c.FormFile("file") // 忽略错误，后面会检查

	manager.UploadPicture(file, "test") // 调用业务管理器处理图片（如果有特殊处理）

	src, err := file.Open()
	if err != nil {
		// 为什么返回参数错误：文件无法打开属于用户输入问题
		common.BaseResponse(c, nil, "文件打开失败", ecode.PARAMS_ERROR)
		return
	}
	defer src.Close() // 确保关闭文件描述符，防止资源泄漏

	// 4. 构建COS存储路径

	key := fmt.Sprintf("test/%s", file.Filename) // 格式：test/文件名

	// 5. 上传到腾讯云COS
	// 为什么使用PutObject：直接流式上传，避免本地存储
	err = tcos.PutObject(src, key)
	if err != nil {
		log.Print(err) // 记录详细错误供排查
		// 为什么返回系统错误：COS操作失败属于后端问题
		common.BaseResponse(c, nil, "上传失败", ecode.SYSTEM_ERROR)
		return
	}

	// 6. 返回成功响应
	common.Success(c, key) // 返回文件在COS的存储路径。保证上传完成
}

// 下载测试接口（流式下载，无显式响应），是的，如果客户端不需要知道文件存储位置，第一个上传接口完全可以改为不显式响应存储路径

// TestDownloadFile godoc
// @Summary      测试文件下载接口「管理员」
// @Tags         file
// @Produce      octet-stream
// @Param        key query string true "文件存储在 COS 的 KEY"
// @Success      200 {file} file "返回文件流"
// @Failure      400 {object} common.Response "下载失败，详情见响应中的 code"
// @Router       /v1/file/test/download [GET]
func TestDownloadFile(c *gin.Context) {
	// 1. 获取查询参数中的文件key
	// 为什么用Query获取：GET请求参数在URL中
	key := c.Query("key")
	if key == "" {
		// 为什么参数错误：缺少必要参数
		common.BaseResponse(c, nil, "缺少 key 参数", ecode.PARAMS_ERROR)
		return
	}

	// 2. 从腾讯云COS获取文件
	// 为什么返回Reader：流式处理，避免内存中加载大文件
	reader, err := tcos.GetObject(key)
	if err != nil {
		log.Printf("文件下载失败: %v", err) // 记录详细错误
		common.BaseResponse(c, nil, "文件下载失败", ecode.SYSTEM_ERROR)
		return
	}
	defer reader.Close() // 确保关闭网络连接

	// 3. 设置HTTP响应头
	// Content-Disposition：告诉浏览器下载文件而非展示
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", path.Base(key)))

	// Content-Type：二进制流类型
	c.Header("Content-Type", "application/octet-stream")

	// Transfer-Encoding：启用分块传输
	c.Header("Transfer-Encoding", "chunked")

	// 4. 流式传输文件内容
	// 为什么使用io.Copy：高效的内存拷贝，支持大文件
	_, err = io.Copy(c.Writer, reader)
	if err != nil {
		log.Printf("流式传输失败: %v", err)
		// 注意：此时HTTP头已发送，不能再修改状态码
	}
}
