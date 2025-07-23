package service

import (
	"fmt"
	"gorm.io/gorm"
	"web_app2/internal/consts"
	"web_app2/internal/ecode"
	"web_app2/internal/model/entity"
	reqSpaceUser "web_app2/internal/model/request/spaceuser"
	resSpace "web_app2/internal/model/response/space"
	resSpaceUser "web_app2/internal/model/response/spaceuser"
	resUser "web_app2/internal/model/response/user"
	"web_app2/internal/repository"
	"web_app2/pkg/casbin"
	"web_app2/pkg/mysql"
)

type SpaceUserService struct {
	SpaceUserRepo *repository.SpaceUserRepository
}

func NewSpaceUserService() *SpaceUserService {
	return &SpaceUserService{
		SpaceUserRepo: repository.NewSpaceUserRepository(),
	}
}
func (s *SpaceUserService) AddSpaceUser(req reqSpaceUser.SpaceUserAddRequest) (uint64, *ecode.ErrorWithCode) {
	//参数校验
	spaceUser := &entity.SpaceUser{
		SpaceID:   req.SpaceID,
		UserID:    req.UserID,
		SpaceRole: req.SpaceRole,
	}
	if req.SpaceRole == "" {
		//默认
		spaceUser.SpaceRole = consts.SPACEROLE_VIEWER
	}
	if err := ValidSpaceUser(spaceUser, true); err != nil {
		return 0, err
	}
	//数据库添加新成员,建议放到req层
	query := mysql.LoadDB()
	originErr := query.Save(spaceUser).Error
	if originErr != nil {
		return 0, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库操作失败")
	}
	//更新RBAC权限
	dom := fmt.Sprintf("space_%d", req.SpaceID)
	casbin.UpdateUserRoleInDomain(casbin.Casbin, req.UserID, consts.SPACEROLE_VIEWER, dom)
	return spaceUser.ID, nil
}

// 校验空间成员对象，区分是编辑校验还是增加成员校验
func ValidSpaceUser(spaceUser *entity.SpaceUser, add bool) *ecode.ErrorWithCode {
	//若创建，需要=检验是否填写了空间ID和用户ID
	if add {
		_, err := NewSpaceService().GetSpaceById(spaceUser.SpaceID)
		if err != nil {
			return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "空间不存在")
		}
		_, err = NewUserService().GetUserById(spaceUser.UserID)
		if err != nil {
			return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "用户不存在")
		}
	}
	//检验空间角色
	if exist := consts.IsSpaceUserRoleExist(spaceUser.SpaceRole); !exist {
		spaceUser.SpaceRole = consts.SPACEROLE_VIEWER
		//return ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "空间角色不存在")
	}
	return nil
}

// 根据ID移除空间成员
func (s *SpaceUserService) RemoveSpaceUserById(id uint64) *ecode.ErrorWithCode {
	db := mysql.LoadDB()
	if !CheckIsCreater(id) {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "自检失败")
	}

	// 5. 执行删除
	if err := db.Where("id = ?", id).Delete(&entity.SpaceUser{}).Error; err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "删除空间成员失败")
	}

	return nil
}
func (s *SpaceUserService) EditSpaceUser(req *reqSpaceUser.SpaceUserEditRequest) (bool, *ecode.ErrorWithCode) {
	//参数校验
	if req.ID <= 0 {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "ID不能为空")
	}
	if req.SpaceRole != "" && !consts.IsSpaceUserRoleExist(req.SpaceRole) {
		return false, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "空间角色不存在")
	}
	if !CheckIsCreater(req.ID) {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "自检失败")
	}
	//记录校验,建议放在req层
	oldSpaceUser := &entity.SpaceUser{}
	query := mysql.LoadDB()
	originErr := query.Model(&entity.SpaceUser{}).Where("id = ?", req.ID).First(oldSpaceUser).Error
	if originErr != nil {
		return false, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "没有找到成员空间")
	}
	if oldSpaceUser.SpaceRole == req.SpaceRole {
		return true, nil
	}
	if err := ValidSpaceUser(oldSpaceUser, false); err != nil {
		return false, err
	}
	//更新空间成员，还是建议放在req层
	query = mysql.LoadDB()
	query.Model(&entity.SpaceUser{}).Where("id = ?", req.ID).Updates(map[string]interface{}{
		"space_role": req.SpaceRole,
	})
	//更新这个空间成员的权限
	casClient := casbin.LoadCasbinMethod()
	domain := fmt.Sprintf("space_%d", oldSpaceUser.SpaceID)
	casbin.UpdateUserRoleInDomain(casClient, oldSpaceUser.UserID, req.SpaceRole, domain)
	return true, nil
}

// 不能对自己的权限进行更改
func CheckIsCreater(id uint64) bool {
	// 1. 获取数据库连接
	db := mysql.LoadDB()
	// 2. 查询空间成员记录
	var spaceUser entity.SpaceUser
	if err := db.Where("id = ?", id).First(&spaceUser).Error; err != nil {
		return false
	}

	// 3. 查询空间信息
	var space entity.Space
	if err := db.Where("id = ?", spaceUser.SpaceID).First(&space).Error; err != nil {
		return false
	}
	// 4. 检查权限：不能删除自己
	if space.UserID == spaceUser.UserID {
		return false
	}
	return true
}
func (s *SpaceUserService) ListSpaceUserVO(req *reqSpaceUser.SpaceUserQueryRequest) ([]resSpaceUser.SpaceUserVO, *ecode.ErrorWithCode) {
	if exist := consts.IsSpaceUserRoleExist(req.SpaceRole); exist {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "空间角色不存在")
	}
	query, err := s.GetQueryWrapper(mysql.LoadDB(), req)
	if err != nil {
		return nil, err
	}
	//建议放在req层
	var spaceUserList []entity.SpaceUser
	query.Model(&entity.SpaceUser{}).Find(&spaceUserList)

	//获取空间成员视图列表
	voList := s.GetSpaceUserVOList(spaceUserList)
	return voList, nil

}

// 放在req
func (s *SpaceUserService) GetQueryWrapper(db *gorm.DB, req *reqSpaceUser.SpaceUserQueryRequest) (*gorm.DB, *ecode.ErrorWithCode) {
	query := db.Session(&gorm.Session{})
	if req.ID > 0 {
		query = query.Where("id = ?", req.ID)
	}
	if req.SpaceID > 0 {
		query = query.Where("space_id = ?", req.SpaceID)
	}
	if req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.SpaceRole != "" {
		query = query.Where("space_role = ?", req.SpaceRole)
	}
	return query, nil
}

// 应该放在req层
// GetSpaceUserVOList 将空间成员实体列表转换为视图对象列表
// 参数 spaceUsers: 空间成员实体列表
// 返回值: 空间成员视图对象列表
func (s *SpaceUserService) GetSpaceUserVOList(spaceUsers []entity.SpaceUser) []resSpaceUser.SpaceUserVO {
	recordUserVO := make(map[uint64]resUser.UserVO)
	recordSpaceVO := make(map[uint64]resSpace.SpaceVO)
	for _, spaceUser := range spaceUsers {
		if _, ok := recordUserVO[spaceUser.UserID]; !ok {
			//该用户没有被查询过，进行查询
			user, _ := NewUserService().GetUserById(spaceUser.UserID)
			//保证用户的存在
			userVO := resUser.GetUserVO(*user)
			recordUserVO[spaceUser.UserID] = userVO
		}
		if _, ok := recordSpaceVO[spaceUser.SpaceID]; !ok {
			//该空间没有被查询过，进行查询
			space, _ := NewSpaceService().GetSpaceById(spaceUser.SpaceID)
			//保证空间的存在
			spaceVO := resSpace.EntityToVO(*space, recordUserVO[spaceUser.UserID])
			recordSpaceVO[spaceUser.SpaceID] = spaceVO
		}
	}
	//封装返回
	voList := make([]resSpaceUser.SpaceUserVO, 0, len(spaceUsers))
	for _, spaceUser := range spaceUsers {
		vo := resSpaceUser.SpaceUserVO{
			ID:        spaceUser.ID,
			SpaceID:   spaceUser.SpaceID,
			UserID:    spaceUser.UserID,
			SpaceRole: spaceUser.SpaceRole,
		}
		vo.User = recordUserVO[spaceUser.UserID]
		vo.SpaceVO = recordSpaceVO[spaceUser.SpaceID]
		voList = append(voList, vo)
	}
	return voList
}

// 根据记录的Id获取空间成员记录
func (s *SpaceUserService) GetSpaceUserById(id uint64) (*entity.SpaceUser, *ecode.ErrorWithCode) {
	if id <= 0 {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "ID不能为空")
	}
	//查询空间成员记录
	query := mysql.LoadDB()
	spaceUser := &entity.SpaceUser{}
	originErr := query.Model(&entity.SpaceUser{}).Where("id = ?", id).First(spaceUser).Error
	if originErr != nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "没有找到该空间成员")
	}
	return spaceUser, nil
}

func GetPermissionList(space *entity.Space, loginUser *entity.User) []string {
	// 初始化空权限列表
	permissionList := []string{}

	// 未登录用户无权限
	if loginUser == nil {
		return permissionList
	}

	// 定义权限角色
	adminPermission := []string{
		"picture:view",
		"picture:edit",
		"picture:delete",
		"picture:upload",
		"spaceUser:manage",
	}

	editorPermission := []string{
		"picture:view",
		"picture:edit",
		"picture:delete",
		"picture:upload",
	}

	viewerPermission := []string{"picture:view"}

	// === 公共图库处理 ===
	if space == nil {
		if loginUser.UserRole == consts.ADMIN_ROLE {
			return adminPermission // 管理员有所有权限
		}
		return viewerPermission // 普通用户有查看权限
	}

	// === 私人空间处理 ===
	if space.SpaceType == consts.SPACE_PRIVATE {
		// 空间所有者或系统管理员有所有权限
		if space.UserID == loginUser.ID || loginUser.UserRole == consts.ADMIN_ROLE {
			return adminPermission
		}
		return permissionList // 其他人无权限
	}

	// === 团队空间处理 ===
	if space.SpaceType == consts.SPACE_TEAM {
		// 查询用户在空间中的角色
		spaceUserInfo, err := NewSpaceUserService().GetSpaceUserBySpaceIdAndUserId(space.ID, loginUser.ID)
		if err != nil || spaceUserInfo == nil {
			return permissionList // 查询失败或非成员无权限
		}

		// 根据角色分配权限
		switch spaceUserInfo.SpaceRole {
		case consts.SPACEROLE_ADMIN:
			return adminPermission
		case consts.SPACEROLE_EDITOR:
			return editorPermission
		default: // 默认为查看者
			return viewerPermission
		}
	}

	return permissionList
}
func (s *SpaceUserService) GetSpaceUserBySpaceIdAndUserId(spaceId uint64, userId uint64) (*entity.SpaceUser, *ecode.ErrorWithCode) {
	if spaceId <= 0 {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "ID不能为空")
	}
	//查询空间成员记录
	query := mysql.LoadDB()
	spaceUser := &entity.SpaceUser{}
	originErr := query.Model(&entity.SpaceUser{}).Where("space_id = ? and user_id = ?", spaceId, userId).First(spaceUser).Error
	if originErr != nil {
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "没有找到该空间成员")
	}
	return spaceUser, nil
}

//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
//回车
