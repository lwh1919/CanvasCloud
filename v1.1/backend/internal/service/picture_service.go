package service

import (
	aliFetcher "backend/internal/api/old_aliyunai/fetcher"
	"backend/internal/common"
	"backend/internal/consts"
	"backend/internal/ecode"
	"backend/internal/manager"
	"backend/internal/model/dto/file"
	"backend/internal/model/entity"
	reqPicture "backend/internal/model/request/picture"
	resPicture "backend/internal/model/response/picture"
	resUser "backend/internal/model/response/user"
	"backend/internal/repository"
	"backend/internal/utils"
	"backend/pkg/cache"
	"backend/pkg/mysql"
	"backend/pkg/redis"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"log"
	"math"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var listGroup singleflight.Group

type PictureService struct {
	PictureRepo *repository.PictureRepository
}

func NewPictureService() *PictureService {
	return &PictureService{
		PictureRepo: repository.NewPictureRepository(),
	}
}

// 修改或插入图片数据到服务器中
// 修改为接收接口类型，可以是URL地址或者文件（multipartFile）
func (s *PictureService) UploadPicture(picFile interface{}, PictureUploadRequest *reqPicture.PictureUploadRequest, loginUser *entity.User) (*resPicture.PictureVO, *ecode.ErrorWithCode) {
	//判断图片是需要新增还是需要更新
	picId := uint64(0)
	if PictureUploadRequest.ID != 0 {
		picId = PictureUploadRequest.ID
	}
	var space *entity.Space
	//校验空间ID是否存在
	//若存在，则需要校验空间是否存在以及是否有权限上传
	fmt.Println("ok")
	if PictureUploadRequest.SpaceID != 0 {

		var err error
		space, err = repository.NewSpaceRepository().GetSpaceById(nil, PictureUploadRequest.SpaceID)
		if err != nil {
			return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库异常")
		}
		if space == nil {
			return nil, ecode.GetErrWithDetail(ecode.NOT_FOUND_ERROR, "空间不存在")
		}
		//仅允许空间管理员上传图片
		switch space.SpaceType {
		case consts.SPACE_PRIVATE:
			//私有空间，只允许管理员上传图片
			if space.UserID != loginUser.ID {
				return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有空间权限")
			}
		case consts.SPACE_TEAM:
			//公共空间，只允许管理员或者编辑者上传图片
			spaceUserInfo, err := NewSpaceUserService().GetSpaceUserBySpaceIdAndUserId(space.ID, loginUser.ID)
			if err != nil {
				return nil, err
			}
			if spaceUserInfo.SpaceRole != consts.SPACEROLE_EDITOR && spaceUserInfo.SpaceRole != consts.SPACEROLE_ADMIN {
				return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有空间权限")
			}
		}
		//校验额度
		if space.TotalCount >= space.MaxCount {
			return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "空间图片数量已满")
		}
		if space.TotalSize >= space.MaxSize {
			return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "空间图片大小已满")
		}
	}
	//若更新图片，则需要校验图片是否存在，以及空间id是否跟原本的一致
	//第一次空间权限验证确保用户有权限在目标空间上传新图片，第二次空间权限验证确保用户有权限修改特定图片（检查图片所有权和空间一致性），两者分别控制空间准入和资源操作权限。
	if picId != 0 {
		oldpic, err := s.PictureRepo.FindById(nil, picId)
		if err != nil {
			return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库异常")
		}
		if oldpic == nil {
			return nil, ecode.GetErrWithDetail(ecode.NOT_FOUND_ERROR, "图片不存在")
		}
		//权限校验，根据空间的不同区分权限
		if space != nil {
			switch space.SpaceType {
			case consts.SPACE_PRIVATE:
				//私有空间，只允许管理员或者空间创建者上传图片
				if loginUser.UserRole != consts.ADMIN_ROLE && loginUser.ID != oldpic.UserID {
					return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "权限不足")
				}
			case consts.SPACE_TEAM:
				//公共空间，只允许管理员或者编辑者上传图片
				spaceUserInfo, err := NewSpaceUserService().GetSpaceUserBySpaceIdAndUserId(space.ID, loginUser.ID)
				if err != nil {
					return nil, err
				}
				if spaceUserInfo.SpaceRole != consts.SPACEROLE_EDITOR && spaceUserInfo.SpaceRole != consts.SPACEROLE_ADMIN {
					return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有空间权限")
				}
			}
		}
		//校验空间是否一致
		if space != nil && oldpic.SpaceID != PictureUploadRequest.SpaceID {
			return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "空间不一致")
		}
		//没传spaceID，则复用原有图片的spaceID（兼容了公共图库）
		if space == nil {
			PictureUploadRequest.SpaceID = oldpic.SpaceID
		}
	}
	//上传图片，得到信息
	//去要区分上传到公共图库还是私人图库
	var uploadPathPrefix string
	if PictureUploadRequest.SpaceID == 0 {
		uploadPathPrefix = fmt.Sprintf("public/%d", loginUser.ID)
	} else {
		//存在space，则上传到私人图库
		uploadPathPrefix = fmt.Sprintf("space/%d", PictureUploadRequest.SpaceID)
	}

	var info *file.UploadPictureResult
	var err *ecode.ErrorWithCode
	//根据参数的不同类型，调用不同的方法。请保证传入的正确性。
	switch v := picFile.(type) {
	case *multipart.FileHeader:
		info, err = manager.UploadPicture(v, uploadPathPrefix)
	case string:
		info, err = manager.UploadPictureByURL(v, uploadPathPrefix, PictureUploadRequest.PicName)
	default:
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "参数错误")
	}
	if err != nil {
		return nil, err
	}
	//构造插入数据库的实体
	pic := &entity.Picture{
		URL:          info.URL,
		ThumbnailURL: info.ThumbnailURL,
		Name:         info.PicName,
		PicSize:      info.PicSize,
		PicWidth:     info.PicWidth,
		PicHeight:    info.PicHeight,
		PicScale:     info.PicScale,
		PicFormat:    info.PicFormat,
		PicColor:     info.PicColor,
		UserID:       loginUser.ID,
		EditTime:     time.Now(),
		SpaceID:      PictureUploadRequest.SpaceID, //指定空间id
	}
	//补充审核校验参数
	s.FillReviewParamsInPic(pic, loginUser)
	//若是更新，则需要更新ID
	if picId != 0 {
		pic.ID = picId
	}
	//开启事务
	tx := s.PictureRepo.BeginTransaction()
	//进行插入或者更新操作，即save
	originErr := s.PictureRepo.SavePicture(tx, pic)
	if originErr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	//修改空间的额度
	if space != nil {
		//设置更新字段
		updateMap := make(map[string]interface{}, 2)
		updateMap["total_count"] = gorm.Expr("total_count + 1")
		updateMap["total_size"] = gorm.Expr("total_size + ?", pic.PicSize)
		err := NewSpaceService().SpaceRepo.UpdateSpaceById(tx, space.ID, updateMap)
		if err != nil {
			return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
		}
	}
	//提交事务
	originErr = tx.Commit().Error
	if originErr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	userVO := resUser.GetUserVO(*loginUser)
	picVO := resPicture.EntityToVO(*pic, userVO)
	return &picVO, nil
}

// 填充审核参数到指定的Pic中
func (s *PictureService) FillReviewParamsInPic(Pic *entity.Picture, LoginUser *entity.User) {
	if LoginUser.UserRole == consts.ADMIN_ROLE {
		Pic.ReviewStatus = consts.PASS
		Pic.ReviewerID = LoginUser.ID
		now := time.Now()
		Pic.ReviewTime = &now // 使用指针
		Pic.ReviewMessage = "管理员自动过审"
	} else {
		Pic.ReviewStatus = consts.REVIEWING
	}
}

func (s *PictureService) DeletePicture(loginUser *entity.User, deleReq *common.DeleteRequest) *ecode.ErrorWithCode {
	//判断id图片是否存在
	oldPic, err := s.GetPictureById(deleReq.Id)
	if err != nil {
		return err
	}
	var space *entity.Space
	var originErr error
	if oldPic.SpaceID != 0 {
		space, originErr = repository.NewSpaceRepository().GetSpaceById(nil, oldPic.SpaceID)
		if originErr != nil {
			return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
		}
	}
	//权限校验
	if err := s.CheckPictureAuth(loginUser, oldPic, space); err != nil {
		return err
	}
	//开启事务
	tx := s.PictureRepo.BeginTransaction()
	//进行删除图片操作
	originErr = s.PictureRepo.DeleteById(tx, deleReq.Id)
	if originErr != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	//修改空间的额度
	if space != nil {
		//设置更新字段
		updateMap := make(map[string]interface{}, 2)
		updateMap["total_count"] = gorm.Expr("total_count - 1")
		updateMap["total_size"] = gorm.Expr("total_size - ?", oldPic.PicSize)
		err := NewSpaceService().SpaceRepo.UpdateSpaceById(tx, space.ID, updateMap)
		if err != nil {
			return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
		}
	}
	//提交事务
	originErr = tx.Commit().Error
	if err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	return nil
}

// 根据ID获取图片，若图片不存在则返回错误
func (s *PictureService) GetPictureById(id uint64) (*entity.Picture, *ecode.ErrorWithCode) {
	Picture, err := s.PictureRepo.FindById(nil, id)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	if Picture == nil {
		return nil, ecode.GetErrWithDetail(ecode.NOT_FOUND_ERROR, "图片不存在")
	}
	return Picture, nil
}

// 校验操作图片权限，公共图库仅本人或管理员可以操作，私人图库仅空间管理员可以操作，团队空间仅空间管理员或者编辑者可以操作
func (s *PictureService) CheckPictureAuth(loginUser *entity.User, picture *entity.Picture, space *entity.Space) *ecode.ErrorWithCode {
	//公共图库，仅本人或管理员可以操作
	if picture != nil && picture.SpaceID == 0 {
		if loginUser.ID != picture.UserID && loginUser.UserRole != consts.ADMIN_ROLE {
			return ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有权限")
		}
	} else {
		//私人图库，仅空间管理员可以操作
		switch space.SpaceType {
		case consts.SPACE_PRIVATE:
			//私有空间，只允许管理员上传图片
			if space.UserID != loginUser.ID {
				return ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有空间权限")
			}
		case consts.SPACE_TEAM:
			//公共空间，只允许管理员或者编辑者上传图片
			spaceUserInfo, err := NewSpaceUserService().GetSpaceUserBySpaceIdAndUserId(space.ID, loginUser.ID)
			if err != nil {
				return err
			}
			if spaceUserInfo.SpaceRole != consts.SPACEROLE_EDITOR && spaceUserInfo.SpaceRole != consts.SPACEROLE_ADMIN {
				return ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有空间权限")
			}
		}
	}
	return nil
}
func (s *PictureService) GetPictureVO(Picture *entity.Picture) *resPicture.PictureVO {
	user, err := repository.NewUserRepository().FindById(nil, Picture.UserID)
	if err != nil {
		return nil
	}
	var picVO resPicture.PictureVO
	if user != nil {
		userVO := resUser.GetUserVO(*user)
		picVO = resPicture.EntityToVO(*Picture, userVO)
	} else {
		picVO = resPicture.EntityToVO(*Picture, resUser.UserVO{})
	}
	return &picVO
}
func (s *PictureService) ListPictureByPage(req *reqPicture.PictureQueryRequest) (*resPicture.ListPictureResponse, *ecode.ErrorWithCode) {
	// 参数校验与默认值
	if req.Current <= 0 {
		req.Current = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 40
	}

	// 获取查询对象（只构建一次）
	query, err := s.GetQueryWrapper(mysql.LoadDB(), req)
	if err != nil {
		return nil, err
	}

	// 查询总数
	var total int64
	if err := query.Model(&entity.Picture{}).Count(&total).Error; err != nil {
		return nil, ecode.GetErrWithDetail(ecode.NOT_FOUND_ERROR, "查询总数失败")
	}

	// 计算分页参数
	offset := (req.Current - 1) * req.PageSize
	totalPages := int(math.Ceil(float64(total) / float64(req.PageSize)))

	// 分页查询
	var Pictures []entity.Picture
	if err := query.Offset(offset).Limit(req.PageSize).Find(&Pictures).Error; err != nil {
		return nil, ecode.GetErrWithDetail(ecode.NOT_FOUND_ERROR, "分页查询失败")
	}

	// 返回结果
	return &resPicture.ListPictureResponse{
		Records: Pictures,
		PageResponse: common.PageResponse{
			Total:   int(total),
			Size:    req.PageSize,
			Pages:   totalPages,
			Current: req.Current,
		},
	}, nil
}

// 获取一个链式查询对象
func (s *PictureService) GetQueryWrapper(db *gorm.DB, req *reqPicture.PictureQueryRequest) (*gorm.DB, *ecode.ErrorWithCode) {
	query := db.Session(&gorm.Session{})
	if req.SearchText != "" {
		query = query.Where("name LIKE ? OR introduction LIKE ?", "%"+req.SearchText+"%", "%"+req.SearchText+"%")
	}
	if req.ID != 0 {
		query = query.Where("id = ?", req.ID)
	}
	if req.UserID != 0 {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+req.Name+"%")
	}
	if req.Introduction != "" {
		query = query.Where("introduction LIKE ?", "%"+req.Introduction+"%")
	}
	if req.PicFormat != "" {
		query = query.Where("pic_format LIKE ?", "%"+req.PicFormat+"%")
	}
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}
	if req.PicWidth != 0 {
		query = query.Where("pic_width = ?", req.PicWidth)
	}
	if req.PicHeight != 0 {
		query = query.Where("pic_height = ?", req.PicHeight)
	}
	if req.PicSize != 0 {
		query = query.Where("pic_size = ?", req.PicSize)
	}
	if req.PicScale != 0 {
		query = query.Where("pic_scale = ?", req.PicScale)
	}
	//补充审核字段条件
	if req.ReviewStatus != nil {
		query = query.Where("review_status = ?", *req.ReviewStatus)
	}
	if req.ReviewMessage != "" {
		query = query.Where("review_message LIKE ?", "%"+req.ReviewMessage+"%")
	}
	if req.ReviewerID != 0 {
		query = query.Where("reviewer_id = ?", req.ReviewerID)
	}
	if req.SpaceID != 0 {
		query = query.Where("space_id = ?", req.SpaceID)
	}
	if req.IsNullSpaceID {
		query = query.Where("space_id IS NULL")
	}
	//补充查询图片的编辑时间，StartEditTime<=查找图片<EndEditTime
	if !req.StartEditTime.IsZero() {
		query = query.Where("edit_time >= ?", req.StartEditTime)
	}
	if !req.EndEditTime.IsZero() {
		query = query.Where("edit_time < ?", req.EndEditTime)
	}
	//tags在数据库中的存储格式：["golang","java","c++"]
	if len(req.Tags) > 0 {
		//and (tags LIKE %"commic" and tags LIKE %"manga"% ...)
		for _, tag := range req.Tags {
			query = query.Where("tags LIKE ?", "%\""+tag+"\"%")
		}
	}
	if req.SortField != "" {
		sortOrder := "ASC"
		if req.SortOrder == "descend" {
			sortOrder = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", req.SortField, sortOrder))
	}
	return query, nil
}

// 分页查询图片视图
func (s *PictureService) ListPictureVOByPage(req *reqPicture.PictureQueryRequest) (*resPicture.ListPictureVOResponse, *ecode.ErrorWithCode) {
	//调用PictureList
	list, err := s.ListPictureByPage(req)
	if err != nil {
		return nil, err
	}
	//获取VO对象
	listVO := &resPicture.ListPictureVOResponse{
		PageResponse: list.PageResponse,
		Records:      s.GetPictureVOList(list.Records),
	}
	return listVO, nil
}

// 获取PictureVO列表
func (s *PictureService) GetPictureVOList(Pictures []entity.Picture) []resPicture.PictureVO {
	var picVOList []resPicture.PictureVO
	// 保存所有需要的user对象
	userMap := make(map[uint64]resUser.UserVO)

	// 创建默认用户VO（用于用户不存在的情况）
	defaultUserVO := resUser.UserVO{
		ID:          0,
		UserAccount: "已删除用户",
	}

	for _, Picture := range Pictures {
		// 如果用户还未被查询，则进行查询
		if _, ok := userMap[Picture.UserID]; !ok {
			user, err := repository.NewUserRepository().FindById(nil, Picture.UserID)
			if err != nil {
				//log.Printf("GetPictureVOList: 查询用户失败 (ID=%d), 错误: %v", Picture.UserID, err)

				// 使用默认用户信息
				userMap[Picture.UserID] = defaultUserVO
				continue
			}

			// 检查用户是否存在
			if user == nil {
				//log.Printf("GetPictureVOList: 用户不存在 (ID=%d)", Picture.UserID)
				userMap[Picture.UserID] = defaultUserVO
				continue
			}

			userVO := resUser.GetUserVO(*user)
			userMap[Picture.UserID] = userVO
		}
	}

	// 转换所有图片为VO对象
	for _, v := range Pictures {
		// 确保用户信息存在
		userVO, exists := userMap[v.UserID]
		if !exists {
			userVO = defaultUserVO
		}

		picVOList = append(picVOList, resPicture.EntityToVO(v, userVO))
	}

	return picVOList
}

// // 分页查询图片视图（带缓存、多级缓存模式ristretto + redis）
// // 该函数实现了带多级缓存的分页查询功能，通过本地缓存和Redis缓存减少数据库压力
// // 参数: req - 图片查询请求结构体，包含分页参数和过滤条件
// // 返回值: 图片列表视图响应体指针 或 错误信息
func (s *PictureService) ListPictureVOByPageWithCache(req *reqPicture.PictureQueryRequest) (*resPicture.ListPictureVOResponse, *ecode.ErrorWithCode) {
	//获取redis客户端，和本地缓存
	redisClient := redis.GetRedisClient()
	localCache := cache.GetCache()
	// 将req转为json字符串，并用md5进行压缩
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "参数序列化失败")
	}
	//进一步将请求转化为缓存key
	hash := md5.Sum(reqBytes)
	m5Str := hex.EncodeToString(hash[:])
	cacheKey := fmt.Sprintf("chg:ListPictureVOByPage:%s", m5Str)
	//尝试获取缓存
	//1.本地缓存获取
	dataInterface, found := localCache.Get(cacheKey)
	if found && dataInterface != nil {
		//断言，保证数据为Byte数组
		dataBytes, _ := dataInterface.([]byte)
		//本地缓存命中，直接返回
		var cachedList resPicture.ListPictureVOResponse
		if err := json.Unmarshal(dataBytes, &cachedList); err == nil {
			log.Println("本地缓存命中，数据成功返回")
			return &cachedList, nil
		}
	}
	//2.分布式缓存获取
	cachedData, err := redisClient.Get(context.Background(), cacheKey).Result()
	if redis.IsNilErr(err) {
		log.Println("缓存未命中，查询数据库...")
	} else if err != nil {
		log.Printf("Redis 读取失败: %v\n", err) // 仅做警告，不中断流程
	} else if cachedData != "" {
		var cachedList resPicture.ListPictureVOResponse
		if err := json.Unmarshal([]byte(cachedData), &cachedList); err == nil {
			log.Println("缓存命中，数据成功返回")
			return &cachedList, nil
		} else {
			log.Println("缓存解析失败，重新查询数据库")
		}
	}

	//缓存未击中，正常流程，并将结果放入缓存
	v, err, _ := listGroup.Do(cacheKey, func() (interface{}, error) {
		data, businessErr := s.ListPictureVOByPage(req)
		if businessErr != nil {
			return data, errors.New(businessErr.Msg)
		}
		return data, nil
	})
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, err.Error())
	}
	//拿到真正的数据
	data := v.(*resPicture.ListPictureVOResponse)

	//数据序列化，加入缓存中，允许存储空值
	dataBytes, err := json.Marshal(data)
	if err != nil {
		//序列化失败，不影响正常流程
		log.Println("数据序列化失败，错误为", err)
		return data, nil
	}
	//设置过期时间，为5分钟~10分钟
	expireTime := time.Duration(rand.IntN(300)+300) * time.Second
	expireTime2 := time.Duration(rand.IntN(200)+300) * time.Second
	go func() {
		// Redis
		if _, err := redis.GetRedisClient().
			Set(context.Background(), cacheKey, dataBytes, expireTime).
			Result(); err != nil {
			log.Println("写 Redis 缓存失败：", err)
		}
		// 本地
		cache.GetCache().SetWithTTL(cacheKey, data, 1, expireTime2)
	}()
	//返回数据
	return data, nil
}

// 一层缓存代码的流程：
// 先查本地缓存（无锁）
// 再查Redis（无锁）
// 如果都没命中，则使用SingleFlight保护数据库查询
// 优化后的代码：
// 先查本地缓存（无锁）
// 如果没命中，则进入SingleFlight保护区域
// 在SingleFlight内部：
// 再次检查本地缓存（避免在等待锁期间已有其他协程更新）
// 查Redis
// 如果Redis没命中，再查数据库
// 更新缓存
// 本质就是在本地缓存和分布式缓存间再做一道sl
// 效果就是单机多次相同请求合一，多机相同请求合一（在redis层）
func (s *PictureService) ProListPictureVOByPageWithCache(req *reqPicture.PictureQueryRequest) (*resPicture.ListPictureVOResponse, *ecode.ErrorWithCode) {
	// 1. 生成缓存键
	cacheKey := generateCacheKey(req)

	// 2. 尝试从本地缓存获取（指针存储，避免序列化）
	if data := getFromLocalCache(cacheKey); data != nil {
		log.Println("✅ 本地缓存命中")
		return data, nil
	}

	// 3. 使用 SingleFlight 保护整个缓存层和数据库层
	v, err, _ := listGroup.Do(cacheKey, func() (interface{}, error) {
		// 3.1 再次检查本地缓存（避免等待期间已有数据）
		if data := getFromLocalCache(cacheKey); data != nil {
			return data, nil
		}

		// 3.2 尝试从 Redis 获取
		if data := getFromRedisCache(cacheKey); data != nil {
			// 回填本地缓存
			expire := time.Duration(rand.IntN(200)+300) * time.Second
			cache.GetCache().SetWithTTL(cacheKey, data, 1, expire)
			return data, nil
		}

		log.Println("🔄 缓存未命中，查询数据库...")

		// 3.3 查询数据库
		data, businessErr := s.ListPictureVOByPage(req)
		if businessErr != nil {
			return nil, errors.New(businessErr.Msg)
		}

		// 3.4 更新缓存（异步）
		go updateMultiCache(cacheKey, data)

		return data, nil
	})

	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, err.Error())
	}

	return v.(*resPicture.ListPictureVOResponse), nil
}

// 生成缓存键
func generateCacheKey(req *reqPicture.PictureQueryRequest) string {
	reqBytes, _ := json.Marshal(req)
	hash := md5.Sum(reqBytes)
	return fmt.Sprintf("chg:ListPictureVOByPage:%x", hash[:])
}

// 从本地缓存获取（直接存储结构体指针）
func getFromLocalCache(cacheKey string) *resPicture.ListPictureVOResponse {
	data, found := cache.GetCache().Get(cacheKey)
	if !found {
		return nil
	}

	if result, ok := data.(*resPicture.ListPictureVOResponse); ok {
		return result
	}
	return nil
}

// 从 Redis 缓存获取
func getFromRedisCache(cacheKey string) *resPicture.ListPictureVOResponse {
	cachedData, err := redis.GetRedisClient().Get(context.Background(), cacheKey).Result()
	if err != nil || cachedData == "" {
		return nil
	}

	var result resPicture.ListPictureVOResponse
	if err := json.Unmarshal([]byte(cachedData), &result); err != nil {
		log.Printf("⚠️ Redis缓存解析失败: %v", err)
		return nil
	}

	log.Println("✅ Redis缓存命中")
	return &result
}

// 更新多级缓存
func updateMultiCache(cacheKey string, data *resPicture.ListPictureVOResponse) {
	// 1. 序列化数据
	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("⚠️ 数据序列化失败: %v", err)
		return
	}

	// 2. 设置随机过期时间（防雪崩）
	redisExpire := time.Duration(rand.IntN(300)+300) * time.Second
	localExpire := time.Duration(rand.IntN(200)+300) * time.Second

	// 3. 并发更新缓存
	var wg sync.WaitGroup
	wg.Add(2)

	// Redis 更新
	go func() {
		defer wg.Done()
		if _, err := redis.GetRedisClient().Set(
			context.Background(),
			cacheKey,
			dataBytes,
			redisExpire,
		).Result(); err != nil {
			log.Printf("⚠️ Redis写入失败: %v", err)
		}
	}()

	// 本地缓存更新
	go func() {
		defer wg.Done()
		cache.GetCache().SetWithTTL(cacheKey, data, 1, localExpire)
	}()

	wg.Wait()
}

// 跟新图片，会进行权限校验
func (s *PictureService) UpdatePicture(updateReq *reqPicture.PictureUpdateRequest, loginUser *entity.User) *ecode.ErrorWithCode {
	//查询图片是否存在
	oldPic, err := s.GetPictureById(updateReq.ID)
	if err != nil {
		return err
	}
	space, _ := NewSpaceService().GetSpaceById(oldPic.SpaceID)
	//权限校验
	if err := s.CheckPictureAuth(loginUser, oldPic, space); err != nil {
		return err
	}
	//权限校验
	oldPic.Name = updateReq.Name
	oldPic.Introduction = updateReq.Introduction
	oldPic.Category = updateReq.Category
	if err := s.ValidPicture(oldPic); err != nil {
		return err
	}
	//保留更新字段
	updateMap := make(map[string]interface{}, 8)
	updateMap["name"] = oldPic.Name
	updateMap["introduction"] = oldPic.Introduction
	updateMap["category"] = oldPic.Category
	tags, _ := json.Marshal(updateReq.Tags)
	updateMap["tags"] = string(tags)
	updateMap["edit_time"] = time.Now()
	//填充审核参数
	s.FillReviewParamsInMap(oldPic, loginUser, updateMap)
	//更新
	if err := s.PictureRepo.UpdateById(nil, updateReq.ID, updateMap); err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	return nil
}

// 填充审核参数到指定的map中
func (s *PictureService) FillReviewParamsInMap(Pic *entity.Picture, LoginUser *entity.User, UpdateMap map[string]interface{}) {
	if LoginUser.UserRole == consts.ADMIN_ROLE {
		UpdateMap["review_status"] = consts.PASS
		UpdateMap["reviewer_id"] = LoginUser.ID
		UpdateMap["review_time"] = time.Now()
		UpdateMap["review_message"] = "管理员自动过审"
	} else {
		UpdateMap["review_status"] = consts.REVIEWING
	}
}

// 图片参数校验，在更新和修改图片前进行判断
func (s *PictureService) ValidPicture(Picture *entity.Picture) *ecode.ErrorWithCode {
	if Picture.ID == 0 {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "图片ID不能为空")
	}
	if len(Picture.URL) > 1024 {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "图片URL过长")
	}
	if len(Picture.Introduction) > 800 {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "图片简介过长")
	}
	if Picture.Name == "" || utf8.RuneCountInString(Picture.Name) > 20 {
		fmt.Println(Picture.Name)
		fmt.Println(len(Picture.Name))
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "图片名不能为空或不能超过20个字符")
	}
	return nil
}

func (s *PictureService) DoPictureReview(req *reqPicture.PictureReviewRequest, user *entity.User) *ecode.ErrorWithCode {
	//参数检验
	if req.ID == 0 || req.ReviewStatus == nil || !consts.ReviewValueExist(*req.ReviewStatus) {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "参数错误")
	}
	//判断图片是否存在
	oldPic, err := s.PictureRepo.FindById(nil, req.ID)
	if err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	if oldPic == nil {
		return ecode.GetErrWithDetail(ecode.NOT_FOUND_ERROR, "图片不存在")
	}
	//校验审核状态是否重复
	if oldPic.ReviewStatus == *req.ReviewStatus {
		return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "请勿重复审核")
	}
	//数据库操作

	//记录要更新的值，防止全部更新效率过低
	updateMap := make(map[string]interface{}, 8)
	updateMap["review_status"] = *req.ReviewStatus
	updateMap["reviewer_id"] = user.ID
	updateMap["review_time"] = time.Now()
	updateMap["review_message"] = req.ReviewMessage
	//执行更新
	if err := s.PictureRepo.UpdateById(nil, req.ID, updateMap); err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	return nil
}
func (s *PictureService) SearchPictureByColor(loginUser *entity.User, picColor string, spaceId uint64) ([]resPicture.PictureVO, *ecode.ErrorWithCode) {
	//1.参数校验
	if spaceId <= 0 || picColor == "" {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "参数错误")
	}
	if loginUser == nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户未登录")
	}

	//获取空间
	space, err := repository.NewSpaceRepository().GetSpaceById(nil, spaceId)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库异常")
	}
	if space == nil {
		return nil, ecode.GetErrWithDetail(ecode.NOT_FOUND_ERROR, "空间不存在")
	}
	//空间权限校验
	if err := s.CheckPictureAuth(loginUser, nil, space); err != nil {
		return nil, err
	}
	//3.查询该空间下的所有图片，必须拥有主色调
	//构造一个查询请求，调用QueryWrapper
	queryRequest := &reqPicture.PictureQueryRequest{
		SpaceID: spaceId,
	}
	query, _ := s.GetQueryWrapper(mysql.LoadDB(), queryRequest)
	//添加条件，查询所有拥有主色调的图片
	query = query.Where("pic_color IS NOT NULL AND pic_color != ''")
	//执行查询
	var pictures []entity.Picture
	err = query.Find(&pictures).Error
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库异常")
	}
	if len(pictures) == 0 {
		return nil, nil //若无图片，返回空列表
	}
	//4.计算相似度并且排序
	sort.Slice(pictures, func(i, j int) bool {
		//计算相似度，使用utils包中的函数
		similarityI := utils.ColorSimilarity(pictures[i].PicColor, picColor)
		similarityJ := utils.ColorSimilarity(pictures[j].PicColor, picColor)
		return similarityI > similarityJ // 降序排列
	})
	//5.返回结果
	//因为不需要用户信息，所以调用响应里自带的方法，减少用户的查询
	var picVOList []resPicture.PictureVO
	for _, picture := range pictures {
		picVOList = append(picVOList, resPicture.EntityToVO(picture, resUser.UserVO{}))
	}
	return picVOList, nil
}

func (s *PictureService) PictureEditByBatch(req *reqPicture.PictureEditByBatchRequest, loginUser *entity.User) (bool, *ecode.ErrorWithCode) {
	//1.参数校验
	if loginUser == nil {
		return false, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "未登录")
	}
	if req.SpaceID <= 0 {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "空间ID不能为空")
	}
	if len([]rune(req.NameRule)) > 20 {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "名称规则过长")
	}
	//2.空间权限校验
	space, err := NewSpaceService().GetSpaceById(req.SpaceID)
	if err != nil {
		return false, err
	}
	if space == nil {
		return false, ecode.GetErrWithDetail(ecode.NOT_FOUND_ERROR, "空间不存在")
	}
	//3.获取图片列表
	var picList []entity.Picture
	db := mysql.LoadDB()
	db.Where(req.PictureIdList).Where("space_id = ?", req.SpaceID).Find(&picList)
	if len(picList) == 0 {
		return true, nil
	}
	//进一步权限校验
	if err := s.CheckPictureAuth(loginUser, &picList[0], space); err != nil {
		return false, err
	}
	//4.更新分类和标签
	//填充名称字段
	s.fillPictureNameWithRule(picList, req.NameRule)
	//设置更新字段
	tags, originErr := json.Marshal(&req.Tags)
	if originErr != nil {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "标签参数错误")
	}
	//批量更新
	originErr = s.PictureRepo.UpdatePicturesByBatch(nil, picList, string(tags), req.Category)
	if originErr != nil {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库批量更新图片异常")
	}
	return true, nil
}

// 填充图片的昵称，传入的昵称规则如“名称{序号}”，序号从1开始递增
func (s *PictureService) fillPictureNameWithRule(pic []entity.Picture, nameRule string) {
	index := 1
	for i := range pic {
		pic[i].Name = strings.Replace(nameRule, "{序号}", fmt.Sprintf("%d", index), -1)
		index++
	}
}

func (s *PictureService) CreatePictureOutPaintingTask(req *reqPicture.CreateOutPaintingTaskRequest, loginUser *entity.User) (*resPicture.CreateOutPaintingTaskResponse, *ecode.ErrorWithCode) {
	//1.参数校验
	if req.PictureID <= 0 {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "图片ID不能为空")
	}
	pic, err := s.GetPictureById(req.PictureID)
	if err != nil {
		return nil, err
	}
	space, _ := repository.NewSpaceRepository().GetSpaceById(nil, pic.SpaceID)
	//2.权限校验
	err = s.CheckPictureAuth(loginUser, pic, space)
	if err != nil {
		return nil, err
	}
	//3.创建任务
	//将前端请求转化为阿里云API请求
	createOutPaintReq := req.ToAliAiRequest(pic.URL)
	//发送任务
	res, err := aliFetcher.CreateOutPaintingTask(createOutPaintReq)
	if err != nil {
		return nil, err
	}
	//4.返回结果
	return resPicture.AOutPaintResToF(res), nil
}
func (s *PictureService) GetOutPaintingTaskResponse(taskId string) (*resPicture.GetOutPaintingResponse, *ecode.ErrorWithCode) {
	//1.参数校验
	if taskId == "" {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "任务ID不能为空")
	}
	//2.获取任务状态
	res, err := aliFetcher.GetOutPaintingTaskResponse(taskId)
	if err != nil {
		return nil, err
	}
	//3.返回结果
	return resPicture.AGetOutPaintResToF(res), nil
}

// 批量爬取图片并上传到系统，返回成功上传的数量
// 批量爬取图片并上传到系统，返回成功上传的数量
func (s *PictureService) UploadPictureByBatch(req *reqPicture.PictureUploadByBatchRequest, loginUser *entity.User) (int, *ecode.ErrorWithCode) {
	// 1. 参数校验与预处理 - 确保系统稳定性和资源保护
	// 为什么需要数量限制：防止恶意请求导致系统资源耗尽
	if req.Count > 30 {
		return 0, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "一次最多上传30张图片")
	}

	// 为什么需要默认名称前缀：提供合理的默认值，增强用户体验
	if req.NamePrefix == "" {
		req.NamePrefix = req.SearchText
	}

	// 2. 构建并发送图片搜索请求 - 处理特殊字符和反爬虫策略
	// URL编码为什么重要：确保中文等特殊字符在URL中正确传输
	encodedSearchText := url.QueryEscape(req.SearchText)

	// 为什么需要随机偏移量：避免被目标网站识别为爬虫，获取更多样化的图片结果
	randInt := rand.IntN(100)

	// 为什么选择Bing异步接口：相比其他搜索引擎，Bing的异步接口更稳定且易于解析
	fetchUrl := fmt.Sprintf("https://cn.bing.com/images/async?q=%s&mmasync=1&first=%d",
		encodedSearchText, randInt)

	// 3. 发送HTTP请求 - 基础网络通信
	// 为什么使用标准库http.Get：简单场景下的最佳选择
	res, err := http.Get(fetchUrl)
	if err != nil {
		// 为什么返回系统错误：区分网络错误和业务错误
		return 0, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "网络请求失败")
	}
	// 为什么需要defer关闭：确保网络资源正确释放，防止内存泄漏
	defer res.Body.Close()

	// 4. 解析HTML内容 - 从HTML中提取结构化数据
	// 为什么选择goquery：提供jQuery风格的API，简化HTML解析
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		// 为什么记录详细错误：便于生产环境问题排查
		log.Println("解析失败，错误为", err)
		return 0, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "解析失败")
	}

	// 5. 定位目标元素 - 精确获取图片容器
	// 为什么使用.dgControl：Bing图片搜索结果特有的容器class
	div := doc.Find(".dgControl").First()
	// 为什么检查元素存在：防止后续操作空对象导致panic
	if div.Length() == 0 {
		log.Println("未找到图片容器元素")
		return 0, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "解析失败")
	}

	// 6. 遍历并处理图片 - 核心业务逻辑
	uploadCount := 0 // 成功计数器

	// 为什么使用EachWithBreak：允许在达到数量时提前终止遍历
	div.Find("img.mimg").EachWithBreak(func(i int, img *goquery.Selection) bool {
		// 7. 获取并处理图片URL
		// 为什么检查src属性：不是所有img标签都有有效src
		fileUrl, exists := img.Attr("src")
		if !exists || strings.TrimSpace(fileUrl) == "" {
			log.Println("当前链接为空，已跳过")
			return true // 继续处理下一张图片
		}

		// 为什么清理URL参数：获取原始图片而非缩略图
		if idx := strings.Index(fileUrl, "?"); idx != -1 {
			fileUrl = fileUrl[:idx]
		}

		// 8. 上传单张图片 - 复用现有服务
		// 为什么构建独立请求：符合服务接口契约，确保参数一致性
		uploadReq := &reqPicture.PictureUploadRequest{
			FileUrl: fileUrl,        // 原始图片URL
			PicName: req.NamePrefix, // 使用统一名称前缀
		}

		// 为什么忽略单图上传错误：批量操作中单点失败不应中断整体流程
		if _, err := s.UploadPicture(fileUrl, uploadReq, loginUser); err != nil {
			// 为什么记录错误但不中断：保证最大程度完成任务
			log.Println("上传失败，错误为", err)
		} else {
			log.Println("上传成功")
			uploadCount++ // 成功时增加计数
		}

		// 9. 提前终止条件 - 性能优化
		// 为什么需要此检查：避免不必要的图片处理
		return uploadCount < req.Count
	})

	// 10. 返回结果 - 业务响应
	return uploadCount, nil
}

//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
//回城
