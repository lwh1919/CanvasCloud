package tcos

import (
	"backend/config"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// 图片详细数据结构体
type PicInfo struct {
	Format        string `json:"format"`
	Width         int    `json:"width,string"`
	Height        int    `json:"height,string"`
	Size          int64  `json:"size,string"`
	MD5           string `json:"md5"`
	FrameCount    int    `json:"frame_count,string"`
	BitDepth      int    `json:"bit_depth,string"`
	VerticalDPI   int    `json:"vertical_dpi,string"`
	HorizontalDPI int    `json:"horizontal_dpi,string"`
}

var tcos *cos.Client

// 程序启动时自动建立COS连接
func Init() error {
	c := config.Conf.Tcos

	// 1. 构建标准的存储桶URL (符合SDK推荐格式)
	// 格式: https://<bucket-name>-<appid>.cos.<region>.myqcloud.com
	// 如果配置中没有提供appid，需要从host中提取或单独配置
	bucketURL := fmt.Sprintf("https://%s-%s.cos.%s.myqcloud.com",
		c.BucketName, c.AppID, c.Region)

	// 优先使用配置中的host（如果提供）
	if c.Host != "" {
		bucketURL = c.Host
	}

	// 2. 解析URL并创建基础URL
	u, err := url.Parse(bucketURL)
	if err != nil {
		return fmt.Errorf("解析存储桶URL失败: %w", err)
	}

	// 3. 创建COS客户端（遵循SDK标准）
	client := cos.NewClient(
		&cos.BaseURL{BucketURL: u},
		&http.Client{
			Transport: &cos.AuthorizationTransport{
				SecretID:  c.SecretID,
				SecretKey: c.SecretKey,
				// 可选：添加Transport以支持更高级的配置
				Transport: &http.Transport{
					DialContext: (&net.Dialer{
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					MaxIdleConns:        100,
					IdleConnTimeout:     90 * time.Second,
					TLSHandshakeTimeout: 10 * time.Second,
				},
			},
		},
	)

	// 4. 使用更可靠的连接测试方法
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 方法1: 使用HEAD请求检查存储桶是否存在（推荐）
	resp, err := client.Bucket.Head(ctx)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			log.Printf("COS存储桶验证成功: %s (状态码 %d)", bucketURL, resp.StatusCode)
			tcos = client
			return nil
		}
	}

	tcos = client
	return nil
}

func LoadDB() *cos.Client {
	return tcos
}

// 上传本地对象到COS服务器中，key是对象在存储桶中的唯一标识，例如"doc/test.txt"，path是本地文件路径。
// 将服务器本地文件上传到云存储
func PutObjectFromLocal(key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	opt := &cos.ObjectPutOptions{}

	_, err = tcos.Object.Put(context.Background(), key, f, opt)
	return err
}

// 上传实现了io.Reader接口的数据，key是对象在存储桶中的唯一标识，例如"doc/test.txt"。
// 直接上传数据流，无需临时文件
func PutObject(f io.Reader, key string) error {
	opt := &cos.ObjectPutOptions{}
	_, err := tcos.Object.Put(context.Background(), key, f, opt)
	if err != nil {
		return err
	}
	return nil
}

// 从 COS 获取文件流（流式传输）
// 获取COS文件内容流
func GetObject(key string) (io.ReadCloser, error) {
	resp, err := tcos.Object.Get(context.Background(), key, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// 上传图片对象，返回原始响应体
// 上传图片并返回尺寸等基本信息
// key是对象在存储桶中的唯一标识，例如"doc/test.jpg"。
func PutPicture(f io.Reader, key string) (*cos.Response, error) {
	pic := &cos.PicOperations{
		IsPicInfo: 1,
	}
	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			XOptionHeader: &http.Header{},
		},
	}
	opt.XOptionHeader.Add("Pic-Operations", cos.EncodePicOperations(pic))
	opt.XOptionHeader.Add("x-cos-return-response", "true")
	res, err := tcos.Object.Put(context.Background(), key, f, opt)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// 上传图片对象并且进行上传时压缩，保存成webp格式，返回原始响应体。
// 原图不会被覆盖。
// key是对象在存储桶中的唯一标识，例如"doc/test.png"。
// 同时，会添加一个缩略图，会缩略原图至宽高至多为256，参考网址为doc/test_thumbnail.png。
// 智能图片处理，一次上传生成三种版本
func PutPictureWithCompress(f io.Reader, key string) (*cos.Response, error) {
	//取出key的后缀，修改为webp
	lastIdx := strings.LastIndex(key, ".")
	var newKey string
	var thumbnailKey string
	//确保安全性
	if lastIdx != -1 {
		keyNoType := key[:lastIdx]
		keyType := key[lastIdx:]
		newKey = keyNoType + ".webp"
		thumbnailKey = keyNoType + "_thumbnail" + keyType
	}
	pic := &cos.PicOperations{
		IsPicInfo: 1,
		Rules: []cos.PicOperationsRules{
			{
				Rule:   "imageMogr2/format/webp",
				FileId: "/" + newKey,
			},
			{
				Rule:   fmt.Sprintf("imageMogr2/thumbnail/%dx%d>", 256, 256),
				FileId: "/" + thumbnailKey,
			},
		},
	}
	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			XOptionHeader: &http.Header{},
		},
	}
	opt.XOptionHeader.Add("Pic-Operations", cos.EncodePicOperations(pic))
	res, err := tcos.Object.Put(context.Background(), key, f, opt)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// 高级上传(整体读+分块传)
func ChunkPutObjectFromLocal(key, path string) error {
	var err error
	//高级上传设置(分块)
	opt := &cos.MultiUploadOptions{
		ThreadPoolSize: 5,
		CheckPoint:     true, // 显式开启断点续传功能
	}
	_, _, err = tcos.Object.Upload(
		context.Background(), key, path, opt,
	)

	return err
}

// 处理本地路径图片并进行压缩和生成缩略图 (分步策略：先分块上传，后处理图片)
func ProcessPutPictureWithCompress(path, key string) error {
	// 1. 使用分块上传将本地文件上传到COS
	err := ChunkPutObjectFromLocal(key, path)
	if err != nil {
		return fmt.Errorf("分块上传失败: %v", err)
	}

	// 2. 准备图片处理规则：生成WebP格式和缩略图
	lastIdx := strings.LastIndex(key, ".")
	var newKey string
	var thumbnailKey string

	if lastIdx != -1 {
		keyNoType := key[:lastIdx]
		keyType := key[lastIdx:]
		newKey = keyNoType + ".webp"                      // WebP格式输出文件
		thumbnailKey = keyNoType + "_thumbnail" + keyType // 缩略图输出文件
	}

	// 3. 构建图片处理操作
	picOps := &cos.PicOperations{
		IsPicInfo: 1, // 返回原图信息
		Rules: []cos.PicOperationsRules{
			{
				Rule:   "imageMogr2/format/webp/quality/85", // 转换为WebP格式并压缩
				FileId: newKey,                              // 输出文件路径
			},
			{
				Rule:   "imageMogr2/thumbnail/!256x256r/interlace/1/format/jpg/quality/85", // 生成缩略图
				FileId: thumbnailKey,                                                       // 缩略图输出路径
			},
		},
	}
	// 5. 调用图像处理API处理已上传的图片[1,3](@ref)
	_, _, err = tcos.CI.ImageProcess(context.Background(), key, picOps)

	return err
}

// 获取图片详细信息，返回详细信息的结构体
// key是对象在存储桶中的唯一标识，例如"doc/test.jpg"。
// 获取COS文件内容流
func GetPictureInfo(key string) (*PicInfo, error) {
	operation := "imageInfo"
	resp, err := tcos.CI.Get(context.Background(), key, operation, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	info, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var picInfo PicInfo
	err = json.Unmarshal(info, &picInfo)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	return &picInfo, nil
}

// 获取图片的主色调，返回十六进制的图片主色调，例如：0x736246
// 提取图片主要颜色值
func GetPictureColor(key string) (string, error) {
	operation := "imageAve"
	resp, err := tcos.CI.Get(context.Background(), key, operation, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	//获取响应体
	var result map[string]string
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	//解析JSON
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	//获取RGB值
	rgb := result["RGB"]
	if rgb == "" {
		return "", fmt.Errorf("获取图片主色调失败")
	}
	return rgb, nil
}

// 删除对象，key为唯一标识
// 删除指定路径的文件
func DeleteObject(key string) error {
	_, err := tcos.Object.Delete(context.Background(), key)
	if err != nil {
		return err
	}
	return nil
}
