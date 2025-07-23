package service

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"mime/multipart"
	"strings"
	"web_app2/internal/common"
	"web_app2/internal/consts"
	"web_app2/internal/ecode"
	"web_app2/internal/manager"
	"web_app2/internal/model/entity"
	reqUser "web_app2/internal/model/request/user"
	"web_app2/internal/repository"
	"web_app2/pkg/argon2"
	"web_app2/pkg/casbin"
	"web_app2/pkg/mysql"
	"web_app2/pkg/session"

	//reqUser "CanvasCloud/internal/models/request/user"
	resUser "web_app2/internal/model/response/user"
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
	encryptPassword := GetEncryptPassword(userPassword)
	//插入数据
	user := &entity.User{
		UserAccount:  userAccount,
		UserPassword: encryptPassword,
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
	casClient := casbin.LoadCasbinMethod()
	_ = casbin.UpdateUserRoleInDomain(casClient, user.ID, consts.SPACEROLE_VIEWER, consts.DOM_PUBLIC)

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

func (s *UserService) UserLogin(c *gin.Context, userAccount, userPassword string) (*resUser.UserLoginVO, *ecode.ErrorWithCode) {
	//1校验
	if userAccount == "" || userPassword == "" {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "账号或密码为空")
	}
	//2转化成哈希后查询
	hashPsw := argon2.GetEncryptString(userPassword, userAccount[:5])
	user, err := s.UserRepo.FindByAccountAndPassword(nil, userAccount, hashPsw)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询错误")
	}
	if user == nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在或密码错误")
	}
	//3存储用户登录信息,需要用到c
	userCopy := *user
	//会话存储
	session.SetSession(c, consts.USER_LOGIN_STATE, userCopy)

	return resUser.GetUserLoginVO(userCopy), nil

}
func (s *UserService) UserLogout(c *gin.Context) (bool, *ecode.ErrorWithCode) {
	//从session中提取用户信息
	_, ok := session.GetSession(c, consts.USER_LOGIN_STATE).(entity.User)
	if !ok {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "未登录")
	}
	session.DeleteSession(c, consts.USER_LOGIN_STATE)
	return false, nil
}

// 获取当前登录用户，是数据库实体，用于内部可以复用
// 未获取到用户信息时，返回nil和错误
func (s *UserService) GetLoginUser(c *gin.Context) (*entity.User, *ecode.ErrorWithCode) {
	//从session中提取用户信息
	currentUser, ok := session.GetSession(c, consts.USER_LOGIN_STATE).(entity.User)
	if !ok {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "用户未登录")
	}
	//数据库进行ID查询，避免数据不一致。追求性能可以不查询。
	curUser, err := s.UserRepo.FindById(nil, currentUser.ID)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库查询失败")
	}
	if curUser == nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
	}
	return curUser, nil
}

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

// 根据ID软删除用户
func (s *UserService) RemoveById(id uint64) (bool, *ecode.ErrorWithCode) {
	if suc, err := s.UserRepo.RemoveById(nil, id); err != nil {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	} else {
		if !suc {
			return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "用户不存在")
		}
		return true, nil
	}
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
