package service

import (
	"backend/internal/consts"
	"backend/internal/ecode"
	"backend/internal/model/entity"
	reqSpaceAnalyze "backend/internal/model/request/space/analyze"
	resSpaceAnalyze "backend/internal/model/response/space/analyze"
	"backend/internal/repository"
	"backend/pkg/mysql"
	"encoding/json"
	"log"
	"math"
	"sync"

	"gorm.io/gorm"
)

type SpaceAnalyzeService struct {
	SpaceAnalyzeRepo *repository.SpaceAnalyzeRepository
}

func NewSpaceAnalyzeService() *SpaceAnalyzeService {
	return &SpaceAnalyzeService{
		SpaceAnalyzeRepo: repository.NewSpaceAnalyzeRepository(),
	}
}

// 校验空间分析权限
func (s *SpaceAnalyzeService) CheckSpaceAnalyzeAuth(SpaceAnalyzeReq *reqSpaceAnalyze.SpaceAnalyzeRequest, loginUser *entity.User) *ecode.ErrorWithCode {
	//校验查询列表
	if SpaceAnalyzeReq.QueryAll || SpaceAnalyzeReq.QueryPublic {
		//需要校验是否是管理员
		if loginUser.UserRole != consts.ADMIN_ROLE {
			return ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有权限")
		}
	} else {
		//私有空间权限校验
		if SpaceAnalyzeReq.SpaceID <= 0 {
			return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "空间ID不能为空")
		}
		space, err := NewSpaceService().GetSpaceById(SpaceAnalyzeReq.SpaceID)
		if err != nil {
			return err
		}
		//仅管理员或空间管理者可以查询空间分析
		if space.UserID != loginUser.ID && loginUser.UserRole != consts.ADMIN_ROLE {
			return ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有权限")
		}
	}
	return nil
}

// 填充空间分析链式查询条件
func (s *SpaceAnalyzeService) FillAnalyzeQueryWrapper(PictureQuery *gorm.DB, req *reqSpaceAnalyze.SpaceAnalyzeRequest) (*gorm.DB, *ecode.ErrorWithCode) {
	query := PictureQuery.Session(&gorm.Session{})
	//全空间分析
	if req.QueryAll {
		//全空间分析不需要任何条件
		return query, nil
	}
	//公共图库分析
	if req.QueryPublic {
		//需要查询spaceId为null的图片
		query = query.Where("space_id IS NULL")
		return query, nil
	}
	//特定空间分析
	if req.SpaceID > 0 {
		query = query.Where("space_id = ?", req.SpaceID)
		return query, nil
	}
	//未指定查询范围
	return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "查询范围不明确")
}

// 空间使用情况分析
func (s *SpaceAnalyzeService) GetSpaceUsageAnalyze(req *reqSpaceAnalyze.SpaceUsageAnalyzeRequest, loginUser *entity.User) (*resSpaceAnalyze.SpaceUsageAnalyzeResponse, *ecode.ErrorWithCode) {
	//校验参数
	//全空间或公共图库需要从picture查询(公共空间没有专门的space)
	res := &resSpaceAnalyze.SpaceUsageAnalyzeResponse{}
	if req.QueryAll || req.QueryPublic {
		//权限校验
		if err := s.CheckSpaceAnalyzeAuth(&req.SpaceAnalyzeRequest, loginUser); err != nil {
			return nil, err
		}
		query := mysql.LoadDB()
		//只获取pic_size字段
		var picSize []int64
		//补充空间字段
		query, err := s.FillAnalyzeQueryWrapper(query, &req.SpaceAnalyzeRequest)
		if err != nil {
			return nil, err
		}
		//查询
		//pluck从 pictures表中查询所有记录的 pic_size列，并将结果存储到 picSize切片中。
		query.Model(&entity.Picture{}).Pluck("pic_size", &picSize)
		//字段填充
		sumSize := int64(0)
		for _, size := range picSize {
			sumSize += size
		}
		res.UsedCount = int64(len(picSize))
		res.UsedSize = sumSize
		return res, nil
	} else {
		//私有空间可以从Space查询
		//参数校验和权限校验
		if err := s.CheckSpaceAnalyzeAuth(&req.SpaceAnalyzeRequest, loginUser); err != nil {
			return nil, err
		}
		space, err := NewSpaceService().GetSpaceById(req.SpaceID)
		if err != nil {
			return nil, err
		}
		res.UsedCount = space.TotalCount
		res.UsedSize = space.TotalSize
		res.MaxCount = space.MaxCount
		res.MaxSize = space.MaxSize
		res.SizeUsageRatio = math.Round(float64(space.TotalSize)/float64(space.MaxSize)*100*100) / 100
		res.CountUsageRatio = math.Round(float64(space.TotalCount)/float64(space.MaxCount)*100*100) / 100
		return res, nil
	}
}

func (s *SpaceAnalyzeService) GetSpaceCategoryAnalyze(req *reqSpaceAnalyze.SpaceCategoryAnalyzeRequest, loginUser *entity.User) ([]resSpaceAnalyze.SpaceCategoryAnalyzeResponse, *ecode.ErrorWithCode) {
	//权限校验
	if err := s.CheckSpaceAnalyzeAuth(&req.SpaceAnalyzeRequest, loginUser); err != nil {
		return nil, err
	}
	//获取查询对象
	query := mysql.LoadDB()
	query = query.Model(&entity.Picture{})
	//补充空间字段
	query, err := s.FillAnalyzeQueryWrapper(query, &req.SpaceAnalyzeRequest)
	if err != nil {
		return nil, err
	}
	//查询分类统计
	var result []resSpaceAnalyze.SpaceCategoryAnalyzeResponse
	//SQL语句，匹配结构体字段昵称的snake_case形式
	if originErr := query.Select("COALESCE(NULLIF(category,''),'未分类') AS category, COUNT(*) AS count, SUM(pic_size) as total_size").
		Group("category").
		Scan(&result).Error; originErr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询失败")
	}
	return result, nil
}

// 获取空间标签统计分析
func (s *SpaceAnalyzeService) GetSpaceTagAnalyze(req *reqSpaceAnalyze.SpaceTagAnalyzeRequest, loginUser *entity.User) ([]resSpaceAnalyze.SpaceTagAnalyzeResponse, *ecode.ErrorWithCode) {
	//权限校验
	if err := s.CheckSpaceAnalyzeAuth(&req.SpaceAnalyzeRequest, loginUser); err != nil {
		return nil, err
	}
	//获取查询对象
	query := mysql.LoadDB()
	query = query.Model(&entity.Picture{})
	//补充空间字段
	query, err := s.FillAnalyzeQueryWrapper(query, &req.SpaceAnalyzeRequest)
	if err != nil {
		return nil, err
	}
	//查询原始标签
	var OriginTags []string
	if originErr := query.
		Where("tags IS NOT NULL").
		Where("tags != ''").
		Pluck("tags", &OriginTags).Error; originErr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询失败")
	}
	//OriginTags = [
	//`["风景","旅游"]`,
	//`["人物","摄影"]`,
	//`["美食"]`
	//]
	TagCount := make(map[string]int64)
	//解析标签
	for _, tags := range OriginTags {
		var tagList []string
		if err := json.Unmarshal([]byte(tags), &tagList); err != nil {
			return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "标签解析失败")
		}
		for _, tag := range tagList {
			TagCount[tag]++
		}
	}
	var result []resSpaceAnalyze.SpaceTagAnalyzeResponse
	for tag, count := range TagCount {
		res := resSpaceAnalyze.SpaceTagAnalyzeResponse{
			Tag:   tag,
			Count: count,
		}
		result = append(result, res)
	}
	return result, nil
}

// 空间大小统计打点器
var spaceSizeMetrics = make(map[uint64]*SpaceSizeMetrics)
var metricsMutex sync.RWMutex

// InitSpaceSizeMetrics 初始化历史数据到打点器
func InitSpaceSizeMetrics() {
	// 使用一条SQL查询直接获取所有空间的图片大小分布统计
	var results []struct {
		SpaceID uint64 `gorm:"column:space_id"`
		Range   string `gorm:"column:range"`
		Count   int64  `gorm:"column:count"`
	}

	err := mysql.LoadDB().Raw(`
		SELECT 
			space_id,
			CASE 
				WHEN pic_size < 102400 THEN '<100KB'
				WHEN pic_size < 512000 THEN '100KB-500KB'
				WHEN pic_size < 1048576 THEN '500KB-1MB'
				ELSE '>1MB'
			END as range,
			COUNT(*) as count
		FROM pictures 
		WHERE space_id IS NOT NULL
		GROUP BY space_id, range
	`).Scan(&results).Error

	if err != nil {
		log.Printf("初始化空间大小统计失败: %v", err)
		return
	}

	// 按空间ID分组处理结果
	spaceMetrics := make(map[uint64]*SpaceSizeMetrics)

	for _, result := range results {
		if _, exists := spaceMetrics[result.SpaceID]; !exists {
			spaceMetrics[result.SpaceID] = &SpaceSizeMetrics{}
		}

		metrics := spaceMetrics[result.SpaceID]
		switch result.Range {
		case "<100KB":
			metrics.Bucket100K = result.Count
		case "100KB-500KB":
			metrics.Bucket500K = result.Count
		case "500KB-1MB":
			metrics.Bucket1M = result.Count
		case ">1MB":
			metrics.Bucket1MPlus = result.Count
		}
	}

	// 批量更新内存中的指标
	metricsMutex.Lock()
	for spaceID, metrics := range spaceMetrics {
		spaceSizeMetrics[spaceID] = metrics
	}
	metricsMutex.Unlock()
}

type SpaceSizeMetrics struct {
	Bucket100K   int64
	Bucket500K   int64
	Bucket1M     int64
	Bucket1MPlus int64
}

// 更新空间大小统计
func updateSpaceSizeMetrics(spaceID uint64, picSize int64, op ...int64) {
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	if _, exists := spaceSizeMetrics[spaceID]; !exists {
		spaceSizeMetrics[spaceID] = &SpaceSizeMetrics{}
	}

	// 默认操作为增加(1)，如果传入参数则使用传入的值
	operation := int64(1)
	if len(op) > 0 {
		operation = op[0]
	}

	metrics := spaceSizeMetrics[spaceID]
	switch {
	case picSize < 100*1024:
		metrics.Bucket100K += operation
	case picSize < 500*1024:
		metrics.Bucket500K += operation
	case picSize < 1024*1024:
		metrics.Bucket1M += operation
	default:
		metrics.Bucket1MPlus += operation
	}
}

// 获取空间大小统计分析（打点版本）
func (s *SpaceAnalyzeService) GetSpaceSizeAnalyze(req *reqSpaceAnalyze.SpaceSizeAnalyzeRequest, loginUser *entity.User) ([]resSpaceAnalyze.SpaceSizeAnalyzeResponse, *ecode.ErrorWithCode) {
	// 权限校验
	if err := s.CheckSpaceAnalyzeAuth(&req.SpaceAnalyzeRequest, loginUser); err != nil {
		return nil, err
	}

	// 初始化打点数据（首次查询时）
	if err := initMetricsIfNeeded(req.SpaceID); err != nil {
		return nil, err
	}

	// 直接读取打点数据
	metricsMutex.RLock()
	metrics := spaceSizeMetrics[req.SpaceID]
	metricsMutex.RUnlock()

	return []resSpaceAnalyze.SpaceSizeAnalyzeResponse{
		{SizeRange: "<100KB", Count: metrics.Bucket100K},
		{SizeRange: "100KB-500KB", Count: metrics.Bucket500K},
		{SizeRange: "500KB-1MB", Count: metrics.Bucket1M},
		{SizeRange: ">1MB", Count: metrics.Bucket1MPlus},
	}, nil
}

// 初始化打点数据（仅首次）
func initMetricsIfNeeded(spaceID uint64) *ecode.ErrorWithCode {
	// 使用双重检查锁定模式，减少锁竞争
	metricsMutex.RLock()
	_, exists := spaceSizeMetrics[spaceID]
	metricsMutex.RUnlock()

	if exists {
		return nil
	}

	// 使用一次性查询获取所有区间的统计数据
	var counts []struct {
		Range string `gorm:"column:range"`
		Count int64  `gorm:"column:count"`
	}

	err := mysql.LoadDB().Raw(`
		SELECT 
			CASE 
				WHEN pic_size < 102400 THEN '<100KB'
				WHEN pic_size < 512000 THEN '100KB-500KB'
				WHEN pic_size < 1048576 THEN '500KB-1MB'
				ELSE '>1MB'
			END as range,
			COUNT(*) as count
		FROM pictures 
		WHERE space_id = ?
		GROUP BY range
	`, spaceID).Scan(&counts).Error

	if err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "初始化统计失败")
	}

	// 初始化计数
	metrics := &SpaceSizeMetrics{}
	for _, c := range counts {
		switch c.Range {
		case "<100KB":
			metrics.Bucket100K = c.Count
		case "100KB-500KB":
			metrics.Bucket500K = c.Count
		case "500KB-1MB":
			metrics.Bucket1M = c.Count
		case ">1MB":
			metrics.Bucket1MPlus = c.Count
		}
	}

	// 再次检查并更新，避免并发初始化
	metricsMutex.Lock()
	defer metricsMutex.Unlock()

	// 再次检查是否已被其他协程初始化
	if _, exists := spaceSizeMetrics[spaceID]; !exists {
		spaceSizeMetrics[spaceID] = metrics
	}

	return nil
}

// 获取规定时间周期内，用户上传图片的情况
func (s *SpaceAnalyzeService) GetSpaceUserAnalyze(req *reqSpaceAnalyze.SpaceUserAnalyzeRequest, loginUser *entity.User) ([]resSpaceAnalyze.SpaceUserAnalyzeResponse, *ecode.ErrorWithCode) {
	//权限校验
	if err := s.CheckSpaceAnalyzeAuth(&req.SpaceAnalyzeRequest, loginUser); err != nil {
		return nil, err
	}
	//获取查询对象
	query := mysql.LoadDB()
	query = query.Model(&entity.Picture{})
	//补充空间字段
	query, err := s.FillAnalyzeQueryWrapper(query, &req.SpaceAnalyzeRequest)
	if err != nil {
		return nil, err
	}
	if req.UserID != 0 {
		query = query.Where("user_id = ?", req.UserID)
	}
	//根据需要分析的时间维度，进行分组
	switch req.TimeDimension {
	case "day":
		//DATE_FORMAT将时间格式化为YYYY-MM-DD
		query = query.Select("DATE_FORMAT(create_time, '%Y-%m-%d') AS period, COUNT(*) AS count")
	case "week":
		//YEARWEEK将时间格式化为第几年的第几周，如202511
		query = query.Select("YEARWEEK(create_time) AS period, COUNT(*) AS count")
	case "month":
		query = query.Select("DATE_FORMAT(create_time, '%Y-%m') AS period, COUNT(*) AS count")
	default:
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "时间维度不合法")
	}
	var result []resSpaceAnalyze.SpaceUserAnalyzeResponse
	if originErr := query.Group("period").Order("period").Scan(&result).Error; originErr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询失败")
	}
	return result, nil
}

// 查询空间使用排名前topN的用户
func (s *SpaceAnalyzeService) GetSpaceRankAnalyze(req *reqSpaceAnalyze.SpaceRankAnalyzeRequest, loginUser *entity.User) ([]entity.Space, *ecode.ErrorWithCode) {
	if loginUser.UserRole != consts.ADMIN_ROLE {
		return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "没有权限")
	}
	if req.TopN == 0 {
		req.TopN = 10
	}
	//获取查询对象
	query := mysql.LoadDB()
	var result []entity.Space
	if originErr := query.Model(&entity.Space{}).Select("id", "space_name", "user_id", "total_size").Order("total_size DESC").Limit(req.TopN).Scan(&result).Error; originErr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询失败")
	}
	return result, nil
}
