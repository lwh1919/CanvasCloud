package service

import (
	"backend/internal/common"
	"backend/internal/consts"
	"backend/internal/ecode"
	"backend/internal/manager"
	"backend/internal/model/entity"
	reqUser "backend/internal/model/request/user"
	"backend/internal/repository"
	"backend/pkg/argon2"
	"backend/pkg/casbin"
	"backend/pkg/jwt"
	"backend/pkg/mysql"
	"backend/pkg/redis"
	//"backend/pkg/session"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"mime/multipart"
	"strings"
	"time"

	//reqUser "CanvasCloud/internal/models/request/user"
	resUser "backend/internal/model/response/user"
)

type UserService struct {
	UserRepo *repository.UserRepository
}

func NewUserService() *UserService {
	return &UserService{
		UserRepo: repository.NewUserRepository(),
	}
}

func (s *UserService) GetUserById(id uint64) (*entity.User, *ecode.ErrorWithCode) {
	user, err := s.UserRepo.FindById(nil, id)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询错误")
	}
	if user == nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
	}
	return user, nil
}

func (s *UserService) UserRegister(userAccount, userPassword, checkPassword string) (uint64, *ecode.ErrorWithCode) {
	//1校验
	if userAccount == "" || userPassword == "" || checkPassword == "" {
		return 0, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "参数为空")
	}
	if len(userAccount) < 4 {
		return 0, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户账号过短")
	}
	if len(userPassword) < 8 || len(checkPassword) < 8 {
		return 0, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户密码过短")
	}
	if userPassword != checkPassword {
		return 0, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "两次输入的密码不一致")
	}
	//2检查账号是否重复
	var cnt int64
	var err error
	if cnt, err = s.UserRepo.CountByAccount(nil, userAccount); err != nil {
		return 0, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "数据库查询错误")
	}
	if cnt > 0 {
		return 0, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "账号重复")
	}
	//加密
	hashedPassword, err := argon2.GetEncryptPassword(userPassword)
	if err != nil {
		log.Printf("⚠️ 密码加密失败: %v", err)
		return 0, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "系统错误")
	}

	user := &entity.User{
		UserAccount:  userAccount,
		UserPassword: hashedPassword,
		UserName:     "无名",
		UserRole:     "user",
	}
	// 语法分解：
	//entity.User{}   // 1. 创建 User 结构体的实例（零值初始化）
	//&                // 2. 获取该实例的内存地址（指针）

	if err = s.UserRepo.CreateUser(nil, user); err != nil {
		return 0, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误，注册失败")
	}
	//5.添加RBAC权限，新用户为在public域下的viewer
	_ = casbin.UpdateUserRoleInDomain(user.ID, consts.SPACEROLE_VIEWER, consts.DOM_PUBLIC)

	return user.ID, nil
}

// 获取一个链式查询对象
func (s *UserService) GetQueryWrapper(db *gorm.DB, req *reqUser.UserQueryRequest) (*gorm.DB, *ecode.ErrorWithCode) {
	query := db.Session(&gorm.Session{})
	if req == nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "参数为空")
	}
	if req.ID != 0 {
		query = query.Where("id = ?", req.ID)
	}
	if req.UserRole != "" {
		query = query.Where("user_role = ?", req.UserRole)
	}
	//模糊查询
	if req.UserAccount != "" {
		query = query.Where("user_account LIKE ?", "%"+req.UserAccount+"%")
	}
	if req.UserName != "" {
		query = query.Where("user_name LIKE ?", "%"+req.UserName+"%")
	}
	if req.UserProfile != "" {
		query = query.Where("user_profile LIKE ?", "%"+req.UserProfile+"%")
	}
	if req.SortField != "" {
		order := "ASC"
		if strings.ToLower(req.SortOrder) == "descend" {
			order = "DESC"
		}
		query = query.Order(req.SortField + " " + order)
	}
	return query, nil
}
func GetEncryptPassword(userPassword string) string {
	//前四位充当盐值
	return argon2.GetEncryptString(userPassword, userPassword[:5])
}

// 用户登录
func (s *UserService) UserLogin(c *gin.Context, userAccount, userPassword string) (*resUser.UserLoginVO, *ecode.ErrorWithCode) {
	// 1. 参数验证
	if userAccount == "" || userPassword == "" {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "账号或密码为空")
	}

	// 2. 按账号查询用户
	user, err := s.UserRepo.FindByAccount(nil, userAccount)
	if err != nil {
		log.Printf("⚠️ 用户查询失败: %v", err)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "系统错误")
	}

	if user == nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
	}

	// 检查用户是否在黑名单
	if isUserBlacklisted(user.ID) {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户已被删除")
	}

	// 3. 密码验证
	isValid, verr := argon2.VerifyPassword(userPassword, user.UserPassword)
	if verr != nil {
		log.Printf("⚠️ 密码验证错误: %v", verr)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "系统错误")
	}

	if !isValid {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "密码错误")
	}

	// 4. 密码迁移（旧格式自动升级）
	if argon2.IsLegacyPassword(user.UserPassword) {
		if err := s.migrateUserPassword(user.ID, userPassword); err != nil {
			log.Printf("⚠️ 密码迁移失败 (用户ID:%d): %v", user.ID, err)
		} else {
			log.Printf("✅ 密码迁移成功 (用户ID:%d)", user.ID)
		}
	}

	// 5. 生成JWT令牌
	token, err := jwt.GenerateToken(user)
	if err != nil {
		log.Printf("生成令牌失败: %v", err)
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "系统错误")
	}
	// 构建响应
	response := resUser.GetUserLoginVO(*user)
	fmt.Println(token)
	response.Token = token // 添加Token字段

	return response, nil
}

// 密码迁移方法
func (s *UserService) migrateUserPassword(userID uint64, password string) error {
	newHash, err := argon2.GetEncryptPassword(password)
	if err != nil {
		return fmt.Errorf("密码生成失败: %w", err)
	}

	if err := s.UserRepo.UpdatePassword(nil, userID, newHash); err != nil {
		return fmt.Errorf("数据库更新失败: %w", err)
	}

	return nil
}

//
//func (s *UserService) UserLogin(c *gin.Context, userAccount, userPassword string) (*resUser.UserLoginVO, *ecode.ErrorWithCode) {
//	//1校验
//	if userAccount == "" || userPassword == "" {
//		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "账号或密码为空")
//	}
//	//2转化成哈希后查询
//	hashPsw := argon2.GetEncryptString(userPassword, userAccount[:5])
//	user, err := s.UserRepo.FindByAccountAndPassword(nil, userAccount, hashPsw)
//	if err != nil {
//		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询错误")
//	}
//	if user == nil {
//		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在或密码错误")
//	}
//	//3存储用户登录信息,需要用到c
//	userCopy := *user
//	//会话存储
//	session.SetSession(c, consts.USER_LOGIN_STATE, userCopy)
//
//	return resUser.GetUserLoginVO(userCopy), nil
//
//}

//func (s *UserService) UserLogout(c *gin.Context) (bool, *ecode.ErrorWithCode) {
//	//从session中提取用户信息
//	_, ok := session.GetSession(c, consts.USER_LOGIN_STATE).(entity.User)
//	if !ok {
//		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "未登录")
//	}
//	session.DeleteSession(c, consts.USER_LOGIN_STATE)
//	return false, nil
//}

//// 获取当前登录用户，是数据库实体，用于内部可以复用
//// 未获取到用户信息时，返回nil和错误
//func (s *UserService) GetLoginUser(c *gin.Context) (*entity.User, *ecode.ErrorWithCode) {
//	//从session中提取用户信息
//	currentUser, ok := session.GetSession(c, consts.USER_LOGIN_STATE).(entity.User)
//	if !ok {
//		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "用户未登录")
//	}
//	//数据库进行ID查询，避免数据不一致。追求性能可以不查询。
//	curUser, err := s.UserRepo.FindById(nil, currentUser.ID)
//	if err != nil {
//		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询失败")
//	}
//	if curUser == nil {
//		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
//	}
//	return curUser, nil
//}

func (s *UserService) GetUserVOById(id uint64) (*resUser.UserVO, *ecode.ErrorWithCode) {
	user, err := s.GetUserById(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户已删除")
	}
	userVO := resUser.GetUserVO(*user)
	return &userVO, nil
}

// 用户登出
func (s *UserService) UserLogout(c *gin.Context) (bool, *ecode.ErrorWithCode) {
	// 1. 从请求头获取Token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return false, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "未提供认证令牌")
	}

	// 2. 解析Token格式
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "令牌格式错误")
	}
	tokenString := parts[1]

	// 3. 验证Token有效性
	claims, err := jwt.VerifyToken(tokenString)
	if err != nil {
		// 无效Token直接返回成功
		return true, nil
	}

	// 4. 计算Token剩余有效期
	expireTime := claims.ExpiresAt.Time
	remainingTime := time.Until(expireTime)

	// 5. 将Token加入黑名单
	blacklistKey := fmt.Sprintf("jwt:blacklist:token:%s", tokenString)
	if err := redis.GetRedisClient().Set(
		context.Background(),
		blacklistKey,
		"1",
		remainingTime,
	).Err(); err != nil {
		log.Printf("⚠️ 令牌黑名单设置失败: %v", err)
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "系统错误")
	}

	// 6. 记录登出日志
	log.Printf("✅ 用户登出成功: user_id=%d", claims.UserID)
	return true, nil
}

func (s *UserService) GetLoginUser(c *gin.Context) (*entity.User, *ecode.ErrorWithCode) {
	// 直接从上下文中获取用户信息
	//c.Get("jwtClaims")
	claims, exists := c.Get("jwtClaims")
	if !exists {
		fmt.Println(exists)
		return nil, ecode.GetErrWithDetail(ecode.NO_AUTH_ERROR, "未认证")
	}

	jwtClaims, ok := claims.(*jwt.Claims)
	if !ok {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "令牌解析错误")
	}

	// 构建用户对象
	user := &entity.User{
		ID:          jwtClaims.UserID,
		UserAccount: jwtClaims.UserAccount,
		UserName:    jwtClaims.UserName,
		UserAvatar:  jwtClaims.UserAvatar,
		UserRole:    jwtClaims.UserRole,
	}
	fmt.Println(user)
	return user, nil
}

// 删除用户（踢人）
func (s *UserService) RemoveById(id uint64) (bool, *ecode.ErrorWithCode) {
	// 1. 先获取用户信息（用于黑名单设置）
	user, err := s.UserRepo.FindById(nil, id)
	if err != nil {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "用户查询失败")
	}
	if user == nil {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
	}

	// 2. 删除用户
	success, err := s.UserRepo.RemoveById(nil, id)
	if err != nil {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	if !success {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
	}

	// 3. 将该用户加入黑名单
	blacklistKey := fmt.Sprintf("jwt:blacklist:%d", id)
	expireTime := 24 * time.Hour
	if err := redis.GetRedisClient().Set(
		context.Background(),
		blacklistKey,
		"1",
		expireTime,
	).Err(); err != nil {
		log.Printf("⚠️ 用户黑名单设置失败: user_id=%d, error=%v", id, err)
	} else {
		log.Printf("✅ 用户加入黑名单: user_id=%d, account=%s", id, user.UserAccount)
	}

	return true, nil
}

// 检查用户是否在黑名单
func isUserBlacklisted(userID uint64) bool {
	blacklistKey := fmt.Sprintf("jwt:blacklist:%d", userID)
	exists, err := redis.GetRedisClient().Exists(context.Background(), blacklistKey).Result()
	return err == nil && exists > 0
}

// 更新用户信息，不存在则返回错误
func (s *UserService) UpdateUser(u *entity.User) *ecode.ErrorWithCode {
	if suc, err := s.UserRepo.UpdateUser(nil, u); err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	} else {
		if !suc {
			return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
		}
		return nil
	}
}

// 更新用户信息的特定字段，不存在则返回错误
func (s *UserService) UpdateUserByMap(req *reqUser.UserEditRequest) *ecode.ErrorWithCode {
	updateMap := map[string]interface{}{
		"user_name":    req.UserName,
		"user_profile": req.UserProfile,
	}
	if suc, err := s.UserRepo.UpdateUserByMap(nil, req.ID, updateMap); err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	} else {
		if !suc {
			return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
		}
		return nil
	}
}

// 获取用户列表
func (s *UserService) ListUserByPage(queryReq *reqUser.UserQueryRequest) (*resUser.ListUserVOResponse, *ecode.ErrorWithCode) {
	query, err := s.GetQueryWrapper(mysql.LoadDB(), queryReq)
	if err != nil {
		return nil, err
	}
	total, _ := s.UserRepo.GetQueryUsersNum(nil, query)
	//拼接分页
	if queryReq.Current == 0 {
		queryReq.Current = 1
	}
	//重置query
	query, _ = s.GetQueryWrapper(mysql.LoadDB(), queryReq)
	query = query.Offset((queryReq.Current - 1) * queryReq.PageSize).Limit(queryReq.PageSize)
	users, errr := s.UserRepo.ListUserByPage(nil, query)
	if errr != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	usersVO := resUser.GetUserVOList(users)
	p := (total + queryReq.PageSize - 1) / queryReq.PageSize
	return &resUser.ListUserVOResponse{
		Records: usersVO,
		PageResponse: common.PageResponse{
			Total:   total,
			Size:    queryReq.PageSize,
			Pages:   p,
			Current: queryReq.Current,
		},
	}, nil
}

// 上传头像接口
func (s *UserService) UploadAvatar(file *multipart.FileHeader, userId uint64) (bool, *ecode.ErrorWithCode) {
	//1.校验
	if file == nil {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件为空")
	}
	if file.Size > 5*1024*1024 {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件过大")
	}
	if !strings.HasPrefix(file.Header.Get("Content-Type"), "image/") {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件格式错误")
	}
	//校验文件大小
	fileSize := file.Size
	ONE_MB := int64(1024 * 1024)
	if fileSize > 5*ONE_MB {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "文件过大，不能超过2MB")
	}
	//2.获取文件路径
	//定义前缀
	uploadPrefix := fmt.Sprintf("avatar/%d", userId)
	result, err := manager.UploadPicture(file, uploadPrefix)
	if err != nil {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "文件上传失败")
	}
	//3.更新数据库
	updateMap := map[string]interface{}{"user_avatar": fmt.Sprintf("%s", result.URL)}
	query := mysql.LoadDB()
	query.Model(&entity.User{}).Where("id = ?", userId).Updates(updateMap)
	if err := query.Error; err != nil {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库更新失败")
	}
	return true, nil
}

//
