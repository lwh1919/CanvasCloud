package manager

import (
	"backend/config"                  // 项目配置
	"backend/internal/ecode"          // 错误码
	"backend/internal/model/dto/file" // 文件DTO
	"backend/pkg/tcos"                // 腾讯云COS操作封装
	"crypto/md5"                      // MD5哈希
	"encoding/hex"                    // 十六进制编码
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"      // 随机数
	"mime/multipart" // 文件上传处理
	"net/http"       // HTTP请求
	"net/url"        // URL解析
	"os"
	"strconv" // 字符串转换
	"strings"
	"time"

	"github.com/google/uuid" // UUID生成
)

// UploadPicture 处理文件上传图片
// multipartFile: 上传的文件对象
// uploadPrefix: COS存储路径前缀
// 返回: 上传结果信息和错误
func UploadPicture(multipartFile *multipart.FileHeader, uploadPrefix string) (*file.UploadPictureResult, *ecode.ErrorWithCode) {
	// 1. 校验图片文件是否合法
	if err := ValidPicture(multipartFile); err != nil {
		return nil, err
	}

	// 2. 生成唯一文件名和存储路径
	u := uuid.New()
	hash := md5.Sum(u[:])
	id := hex.EncodeToString(hash[:])[:16] // 取前16位作为唯一ID

	// 获取文件后缀
	fileType := multipartFile.Filename[strings.LastIndex(multipartFile.Filename, ".")+1:]

	// 构造文件名: 日期_唯一ID.后缀
	uploadFileName := fmt.Sprintf("%s_%s.%s", time.Now().Format("2006-01-02"), id, fileType)
	fileNameNoType := uploadFileName[:strings.LastIndex(uploadFileName, ".")]

	// COS存储路径: 前缀/文件名
	uploadPath := fmt.Sprintf("%s/%s", uploadPrefix, uploadFileName)

	// 3. 打开文件流并上传到COS
	src, _ := multipartFile.Open()
	defer src.Close()

	// 调用压缩上传函数
	_, err := tcos.PutPictureWithCompress(src, uploadPath)
	if err != nil {
		log.Print(err)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "上传失败")
	}

	// 4. 获取上传后的图片信息
	// 压缩后格式变为webp
	uploadPath = strings.Replace(uploadPath, fileType, "webp", 1)

	// 缩略图路径
	thumbnailUrl := strings.Replace(uploadPath, ".webp", "_thumbnail."+fileType, 1)

	// 获取图片元数据
	picInfo, err := tcos.GetPictureInfo(uploadPath)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片信息失败")
	}

	// 获取图片主色调
	color, err := tcos.GetPictureColor(uploadPath)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片主色调失败")
	}

	// 5. 构造返回结果
	return &file.UploadPictureResult{
		URL:          config.LoadConfig().Tcos.Host + "/" + uploadPath,                     // 完整URL
		ThumbnailURL: config.LoadConfig().Tcos.Host + "/" + thumbnailUrl,                   // 缩略图URL
		PicName:      fileNameNoType,                                                       // 图片名称（不含后缀）
		PicSize:      picInfo.Size,                                                         // 文件大小
		PicWidth:     picInfo.Width,                                                        // 图片宽度
		PicHeight:    picInfo.Height,                                                       // 图片高度
		PicScale:     math.Round(float64(picInfo.Width)/float64(picInfo.Height)*100) / 100, // 宽高比
		PicFormat:    picInfo.Format,                                                       // 图片格式
		PicColor:     color,                                                                // 主色调
	}, nil
}

// 本地上传图片（分片版本）
func ProUploadPicture(multipartFile *multipart.FileHeader, uploadPrefix string) (*file.UploadPictureResult, *ecode.ErrorWithCode) {
	// 1. 校验图片文件是否合法
	if err := ValidPicture(multipartFile); err != nil {
		return nil, err
	}

	// 2. 生成唯一文件名和存储路径
	u := uuid.New()
	hash := md5.Sum(u[:])
	id := hex.EncodeToString(hash[:])[:16] // 取前16位作为唯一ID

	// 获取原始文件名作为基础名称
	picName := multipartFile.Filename

	// 3. 下载图片到本地临时文件
	localFilePath, err := downLoadPictureByFile(multipartFile, &picName)
	if err != nil {
		return nil, err
	}

	// 确保删除临时文件
	defer deleteTempFile(localFilePath)
	// 获取文件后缀
	fileType := multipartFile.Filename[strings.LastIndex(multipartFile.Filename, ".")+1:]

	// 构造文件名: 日期_唯一ID.后缀
	uploadFileName := fmt.Sprintf("%s_%s.%s", time.Now().Format("2006-01-02"), id, fileType)
	fileNameNoType := uploadFileName[:strings.LastIndex(uploadFileName, ".")]

	// COS存储路径: 前缀/文件名
	uploadPath := fmt.Sprintf("%s/%s", uploadPrefix, uploadFileName)

	// 调用压缩上传函数
	nerr := tcos.ProcessPutPictureWithCompress(localFilePath, uploadPath)
	if nerr != nil {
		log.Print(nerr)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "上传失败")
	}

	// 4. 获取上传后的图片信息
	// 压缩后格式变为webp
	uploadPath = strings.Replace(uploadPath, fileType, "webp", 1)

	// 缩略图路径
	thumbnailUrl := strings.Replace(uploadPath, ".webp", "_thumbnail."+fileType, 1)

	// 获取图片元数据
	picInfo, nerr := tcos.GetPictureInfo(uploadPath)
	if nerr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片信息失败")
	}

	// 获取图片主色调
	color, nerr := tcos.GetPictureColor(uploadPath)
	if nerr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片主色调失败")
	}

	// 5. 构造返回结果
	return &file.UploadPictureResult{
		URL:          config.LoadConfig().Tcos.Host + "/" + uploadPath,                     // 完整URL
		ThumbnailURL: config.LoadConfig().Tcos.Host + "/" + thumbnailUrl,                   // 缩略图URL
		PicName:      fileNameNoType,                                                       // 图片名称（不含后缀）
		PicSize:      picInfo.Size,                                                         // 文件大小
		PicWidth:     picInfo.Width,                                                        // 图片宽度
		PicHeight:    picInfo.Height,                                                       // 图片高度
		PicScale:     math.Round(float64(picInfo.Width)/float64(picInfo.Height)*100) / 100, // 宽高比
		PicFormat:    picInfo.Format,                                                       // 图片格式
		PicColor:     color,                                                                // 主色调
	}, nil
}

// ValidPicture 验证上传的图片文件是否合法
func ValidPicture(multipartFile *multipart.FileHeader) *ecode.ErrorWithCode {
	// 1. 检查文件是否为空
	if multipartFile == nil {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件为空")
	}

	// 2. 检查文件大小（最大2MB）
	fileSize := multipartFile.Size
	ONE_MB := int64(1024 * 1024)
	if fileSize > 2*ONE_MB {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件过大，不能超过2MB")
	}

	// 3. 检查文件类型
	lastDotIndex := strings.LastIndex(multipartFile.Filename, ".")
	if lastDotIndex == -1 {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件不是图片")
	}

	// 获取文件后缀（带点）
	fileType := multipartFile.Filename[lastDotIndex:]

	// 允许的文件类型
	allowType := []string{".jpg", ".jpeg", ".png", ".webp"}
	isAllow := false
	for _, v := range allowType {
		if fileType == v {
			isAllow = true
			break
		}
	}

	if !isAllow {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件类型不支持")
	}

	return nil
}

// ProUploadPictureByURL 通过URL上传图片(分片)
func UploadPictureByURL(fileURL string, uploadPrefix string, picName string) (*file.UploadPictureResult, *ecode.ErrorWithCode) {
	// 1. 处理图片名称
	if picName == "" {
		picName = "临时图片"
	}

	// 添加随机后缀防止并发覆盖
	picName = picName + strconv.Itoa(rand.Intn(999999)+1)

	// 2. 校验URL图片
	if err := ValidPictureByURL(fileURL, &picName); err != nil {
		return nil, err
	}

	// 3. 下载图片到本地临时文件
	localFilePath, err := downLoadPictureByURL(fileURL, &picName)
	if err != nil {
		return nil, err
	}

	// 确保删除临时文件
	defer deleteTempFile(localFilePath)

	// 4. 生成唯一文件名和存储路径
	u := uuid.New()
	hash := md5.Sum(u[:])
	id := hex.EncodeToString(hash[:])[:16]

	// 获取文件后缀
	fileType := localFilePath[strings.LastIndex(localFilePath, ".")+1:]

	// 构造文件名: 日期_唯一ID.后缀
	uploadFileName := fmt.Sprintf("%s_%s.%s", time.Now().Format("2006-01-02"), id, fileType)

	// COS存储路径
	uploadPath := fmt.Sprintf("%s/%s", uploadPrefix, uploadFileName)
	errr := tcos.ProcessPutPictureWithCompress(localFilePath, uploadPath)
	if errr != nil {
		log.Print(errr)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "上传失败")
	}
	fmt.Printf(uploadPath)

	// 6. 获取上传后的图片信息
	uploadPath = strings.Replace(uploadPath, fileType, "webp", 1)                   // 压缩后格式变为webp
	thumbnailUrl := strings.Replace(uploadPath, ".webp", "_thumbnail."+fileType, 1) // 缩略图路径

	picInfo, errr := tcos.GetPictureInfo(uploadPath)
	if errr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片信息失败")
	}

	color, errr := tcos.GetPictureColor(uploadPath)
	if errr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片主色调失败")
	}

	// 7. 构造返回结果
	picNameNoType := picName[:strings.LastIndex(picName, ".")] // 去除后缀的名称
	return &file.UploadPictureResult{
		URL:          config.LoadConfig().Tcos.Host + "/" + uploadPath,
		ThumbnailURL: config.LoadConfig().Tcos.Host + "/" + thumbnailUrl,
		PicName:      picNameNoType,
		PicSize:      picInfo.Size,
		PicWidth:     picInfo.Width,
		PicHeight:    picInfo.Height,
		PicScale:     math.Round(float64(picInfo.Width)/float64(picInfo.Height)*100) / 100,
		PicFormat:    picInfo.Format,
		PicColor:     color,
	}, nil
}

// UploadPictureByURL 通过URL上传图片
func UploadPictureByURLStream(fileURL string, uploadPrefix string, picName string) (*file.UploadPictureResult, *ecode.ErrorWithCode) {
	// 1. 处理图片名称
	if picName == "" {
		picName = "临时图片"
	}

	// 添加随机后缀防止并发覆盖
	picName = picName + strconv.Itoa(rand.Intn(999999)+1)

	// 2. 校验URL图片
	if err := ValidPictureByURL(fileURL, &picName); err != nil {
		return nil, err
	}

	// 4. 生成唯一文件名和存储路径
	u := uuid.New()
	hash := md5.Sum(u[:])
	id := hex.EncodeToString(hash[:])[:16]

	// 获取文件后缀
	fileType := picName[strings.LastIndex(picName, ".")+1:]

	// 构造文件名: 日期_唯一ID.后缀
	uploadFileName := fmt.Sprintf("%s_%s.%s", time.Now().Format("2006-01-02"), id, fileType)

	// COS存储路径
	uploadPath := fmt.Sprintf("%s/%s", uploadPrefix, uploadFileName)

	// 发起HTTP请求获取图片
	resp, err := http.Get(fileURL)
	if err != nil {
		log.Printf("获取图片失败: %v", err)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片失败")
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		log.Printf("获取图片失败，状态码: %d", resp.StatusCode)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片失败")
	}

	// 使用流式上传并压缩
	_, err = tcos.PutPictureWithCompress(resp.Body, uploadPath)
	if err != nil {
		log.Print(err)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "上传失败")
	}

	// 6. 获取上传后的图片信息
	uploadPath = strings.Replace(uploadPath, fileType, "webp", 1)                   // 压缩后格式变为webp
	thumbnailUrl := strings.Replace(uploadPath, ".webp", "_thumbnail."+fileType, 1) // 缩略图路径

	picInfo, err := tcos.GetPictureInfo(uploadPath)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片信息失败")
	}

	color, err := tcos.GetPictureColor(uploadPath)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "获取图片主色调失败")
	}

	// 7. 构造返回结果
	picNameNoType := picName[:strings.LastIndex(picName, ".")] // 去除后缀的名称
	return &file.UploadPictureResult{
		URL:          config.LoadConfig().Tcos.Host + "/" + uploadPath,
		ThumbnailURL: config.LoadConfig().Tcos.Host + "/" + thumbnailUrl,
		PicName:      picNameNoType,
		PicSize:      picInfo.Size,
		PicWidth:     picInfo.Width,
		PicHeight:    picInfo.Height,
		PicScale:     math.Round(float64(picInfo.Width)/float64(picInfo.Height)*100) / 100,
		PicFormat:    picInfo.Format,
		PicColor:     color,
	}, nil
}

// downLoadPictureByURL 下载URL图片到本地临时文件
func downLoadPictureByFile(multipartFile *multipart.FileHeader, picName *string) (string, *ecode.ErrorWithCode) {
	// 1. 打开文件流
	src, err := multipartFile.Open()
	if err != nil {
		return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "打开文件失败")
	}
	defer src.Close()

	// 2. 检查并补充文件后缀
	if lastDotIndex := strings.LastIndex(*picName, "."); lastDotIndex == -1 {
		// 读取文件头部信息来判断文件类型
		buffer := make([]byte, 512)
		_, err := src.Read(buffer)
		if err != nil {
			return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "读取文件头部失败")
		}

		// 重置读取位置
		_, err = src.Seek(0, 0)
		if err != nil {
			return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "重置文件读取位置失败")
		}

		if err := ValidPictureByHeaderBytes(buffer, picName); err != nil {
			return "", err
		}
	}

	// 3. 创建临时目录
	tempDir := "tempfile"
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "创建临时文件夹失败")
	}

	// 4. 创建临时文件
	tempFilePath := tempDir + "/" + *picName
	file, err := os.Create(tempFilePath)
	if err != nil {
		log.Println(err)
		return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "创建临时文件失败")
	}
	defer file.Close()

	// 5. 写入文件
	_, err = io.Copy(file, src)
	if err != nil {
		return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "写入文件失败")
	}

	return tempFilePath, nil
}

// downLoadPictureByURL 下载URL图片到本地临时文件
func downLoadPictureByURL(fileURL string, picName *string) (string, *ecode.ErrorWithCode) {
	// 1. 下载图片
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "下载图片失败")
	}
	defer resp.Body.Close()

	// 2. 检查并补充文件后缀
	if lastDotIndex := strings.LastIndex(*picName, "."); lastDotIndex == -1 {
		if err := ValidPictureByHeader(resp, picName); err != nil {
			return "", err
		}
	}

	// 3. 创建临时目录
	tempDir := "tempfile"
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "创建临时文件夹失败")
	}

	// 4. 创建临时文件
	tempFilePath := tempDir + "/" + *picName
	file, err := os.Create(tempFilePath)
	if err != nil {
		log.Println(err)
		return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "链接格式不支持")
	}
	defer file.Close()

	// 5. 写入文件
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return "", ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "写入文件失败")
	}

	return tempFilePath, nil
}

// deleteTempFile 删除临时文件
func deleteTempFile(tempFilePath string) {
	if tempFilePath != "" {
		os.Remove(tempFilePath)
	}
}

// ValidPictureByURL 验证URL图片是否合法
func ValidPictureByURL(fileURL string, picName *string) *ecode.ErrorWithCode {
	// 1. 检查URL是否为空
	if fileURL == "" {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "URL为空")
	}

	// 2. 检查URL格式
	_, err := url.ParseRequestURI(fileURL)
	if err != nil {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "URL格式错误")
	}

	// 3. 检查URL协议
	if !strings.HasPrefix(fileURL, "http") && !strings.HasPrefix(fileURL, "https") {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "仅支持 HTTP 或 HTTPS 协议的文件地址")
	}

	// 4. 发送HEAD请求验证文件
	resp, err := http.Head(fileURL)
	if err != nil {
		return nil // 无法连接时跳过其他检查
	}
	defer resp.Body.Close()

	// 5. 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// 6. 检查文件类型
	if err := ValidPictureByHeader(resp, picName); err != nil {
		return err
	}

	// 7. 检查文件大小
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		size, err := strconv.ParseUint(contentLength, 10, 64)
		if err != nil {
			return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件大小格式异常")
		}

		ONE_M := uint64(1024 * 1024)
		if size > 2*ONE_M {
			return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件过大，不能超过2MB")
		}
	}

	return nil
}

// ValidPictureByHeader 通过HTTP响应头验证图片类型
func ValidPictureByHeader(resp *http.Response, picName *string) *ecode.ErrorWithCode {
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		// 允许的MIME类型
		allowType := []string{"image/jpeg", "image/jpg", "image/png", "image/webp"}
		isAllow := false

		for _, v := range allowType {
			if contentType == v {
				// 添加文件后缀
				*picName = *picName + "." + strings.Split(contentType, "/")[1]
				isAllow = true
				break
			}
		}

		if !isAllow {
			return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件类型不支持")
		}
	}

	return nil
}

// ValidPictureByHeaderBytes 通过文件头部字节验证图片类型
func ValidPictureByHeaderBytes(buffer []byte, picName *string) *ecode.ErrorWithCode {
	// 通过文件头部魔数判断文件类型
	var fileExt string

	// 检查文件头魔数
	if len(buffer) >= 8 {
		switch {
		case buffer[0] == 0xFF && buffer[1] == 0xD8 && buffer[2] == 0xFF:
			fileExt = ".jpg"
		case buffer[0] == 0x89 && buffer[1] == 0x50 && buffer[2] == 0x4E && buffer[3] == 0x47:
			fileExt = ".png"
		case buffer[0] == 0x52 && buffer[1] == 0x49 && buffer[2] == 0x46 && buffer[3] == 0x46 && buffer[8] == 0x57 && buffer[9] == 0x45 && buffer[10] == 0x42:
			fileExt = ".webp"
		default:
			return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件类型不支持")
		}

		// 添加文件后缀
		*picName = *picName + fileExt
		return nil
	}

	return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "无法识别文件类型")
}
