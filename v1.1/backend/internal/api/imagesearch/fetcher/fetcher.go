package fetcher

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"resty.dev/v3"
	"strings"
	"time"
	"backend/internal/api/imagesearch/model"
	"backend/internal/ecode"
)

type BaiduResponse struct {
	Status int    `json:"status"` // 0表示成功，非0表示错误
	Msg    string `json:"msg"`    // 错误信息
	Data   struct {
		URL  string `json:"url"`  // 搜索结果的页面URL
		Sign string `json:"sign"` // 签名参数(用于后续请求)
	} `json:"data"`
}

// 调用百度以图搜图接口获取原始搜索结果URL
// 上传原始图片地址
// 仅支持20M以下jpg，jpeg，png，bmp，gif等格式的图片
// 请传入webp图片，会添加解析成为PNG格式
func GetImagePageURL(imageURL string) (string, *ecode.ErrorWithCode) {
	// 1. 处理图片URL - 添加格式转换参数
	imageURL = imageURL + "?imageMogr2/format/png"
	// URL编码处理特殊字符
	imageURL = url.QueryEscape(imageURL)

	// 2. 准备表单数据
	formData := map[string]string{
		"image":        imageURL,        // 图片URL
		"tn":           "pc",            // 平台类型(PC端)
		"from":         "pc",            // 来源(PC端)
		"image_source": "PC_UPLOAD_URL", // 图片来源(URL上传)
	}

	// 3. 生成时间戳(防止缓存)
	uptime := fmt.Sprintf("%d", time.Now().UnixMilli())
	// 构造请求URL
	reqUrl := "https://graph.baidu.com/upload?uptime=" + uptime

	// 4. 发送POST请求
	client := resty.New() // 创建HTTP客户端
	resp, err := client.R().
		SetHeader("Acs-Token", fmt.Sprintf("%d", rand.IntN(1000))). // 随机请求头(防反爬)
		SetFormData(formData). // 设置表单数据
		SetTimeout(5 * time.Second). // 设置超时时间
		Post(reqUrl) // 发送POST请求

	// 5. 处理请求错误
	if err != nil || resp.StatusCode() != http.StatusOK {
		return "", ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "请求搜图接口失败")
	}

	// 6. 解析JSON响应
	var baiduResp BaiduResponse
	if err := json.Unmarshal(resp.Bytes(), &baiduResp); err != nil {
		return "", ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "解析响应结果失败")
	}

	// 7. 验证响应状态
	if baiduResp.Status != 0 || baiduResp.Data.URL == "" {
		return "", ecode.GetErrWithDetail(ecode.OPERATION_ERROR,
			"获取图片页面失败，可能是图片格式不支持")
	}

	// 8. 返回搜索结果页面URL
	return baiduResp.Data.URL, nil
}

// 通过GetImagePageURL获取的URL，来获取简略图片信息的请求接口，即FirstURL(负责从搜索结果页面提取获取图片列表的API URL（称为"FirstURL"）)
func GetImageFirstURL(searchResultURL string) (string, *ecode.ErrorWithCode) {
	// 1. 发送HTTP GET请求
	resp, err := http.Get(searchResultURL)
	if err != nil {
		return "", ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "请求图片页面失败")
	}
	defer resp.Body.Close() // 确保关闭响应体

	// 2. 解析HTML文档
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "解析图片页面失败")
	}

	// 3. 查找所有<script>标签
	scriptElements := doc.Find("script")

	// 4. 编译正则表达式 - 匹配"firstUrl"字段
	reg := regexp.MustCompile(`"firstUrl"\s*:\s*"(.*?)"`)

	var firstURL string

	// 5. 遍历所有script标签
	scriptElements.EachWithBreak(func(i int, s *goquery.Selection) bool {
		scriptContent := s.Text() // 获取脚本内容

		// 6. 使用正则表达式匹配
		matches := reg.FindStringSubmatch(scriptContent)
		if len(matches) > 1 {
			// 7. 处理URL中的转义字符(\/ -> /)
			firstURL = strings.ReplaceAll(matches[1], "\\/", "/")
			return false // 找到后退出循环
		}
		return true // 继续查找
	})

	// 8. 验证是否找到firstUrl
	if firstURL == "" {
		return "", ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "搜索失败")
	}

	return firstURL, nil
}

// firstURL，它本质上是包含完整搜索上下文的API端点
func GetImageList(firstURL string) ([]model.ImageSearchResult, *ecode.ErrorWithCode) {
	// 1. 发送HTTP GET请求
	resp, err := http.Get(firstURL)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "请求图片列表失败")
	}
	defer resp.Body.Close() // 确保关闭响应体

	// 2. 读取响应数据
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "读取响应数据失败")
	}

	// 3. 解析JSON数据
	var apiResp model.APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "解析JSON数据失败")
	}

	// 4. 返回图片列表
	return apiResp.Data.List, nil
}
