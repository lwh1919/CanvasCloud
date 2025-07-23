package controller

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"log"
	"web_app2/internal/common"
	"web_app2/internal/ecode"
	"web_app2/internal/model/entity"
	reqSpaceUser "web_app2/internal/model/request/spaceuser"
	resSpaceUser "web_app2/internal/model/response/spaceuser"
	"web_app2/internal/service"
	"web_app2/pkg/mysql"
)

func dump3() {
	temp := resSpaceUser.SpaceUserVO{}
	_ = temp
}

var sSpaceUser *service.SpaceUserService

// AddSpaceUser godoc
// @Summary      增加成员到空间
// @Tags         spaceUser
// @Accept       json
// @Produce      json
// @Param		request body reqSpaceUser.SpaceUserAddRequest true "成员的ID和空间ID，以及添加的成员角色"
// @Success      200  {object}  common.Response{data=string} "返回空间成员表的数据ID，字符串格式"
// @Failure      400  {object}  common.Response "查询失败，详情见响应中的code"
// @Router       /v1/spaceUser/add [POST]
func AddSpaceUser(c *gin.Context) {
	req := reqSpaceUser.SpaceUserAddRequest{}
	if err := c.ShouldBind(&req); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}
	id, err := sSpaceUser.AddSpaceUser(req)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, fmt.Sprintf("%d", id))
}

// DeleteSpaceUser godoc
// @Summary      从空间移除成员
// @Tags         spaceUser
// @Accept       json
// @Produce      json
// @Param		request body reqSpaceUser.SpaceUserRemoveRequest true "空间成员表中数据的ID"
// @Success      200  {object}  common.Response{data=bool} "返回成功与否"
// @Failure      400  {object}  common.Response "查询失败，详情见响应中的code"
// @Router       /v1/spaceUser/delete [POST]
func DeleteSpaceUser(c *gin.Context) {

	req := reqSpaceUser.SpaceUserRemoveRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.BaseResponse(c, false, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}

	// 调用服务
	errResult := sSpaceUser.RemoveSpaceUserById(req.ID)
	if errResult != nil {
		log.Printf("删除失败: %s (代码: %d)", errResult.Msg, errResult.Code)
		common.BaseResponse(c, false, errResult.Msg, errResult.Code)
		return
	}

	log.Println("删除成功")
	common.Success(c, true)
}

// EditSpaceUser godoc
// @Summary      编辑成员权限
// @Tags         spaceUser
// @Accept       json
// @Produce      json
// @Param		request body reqSpaceUser.SpaceUserEditRequest true "记录的ID和需要调整的权限"
// @Success      200  {object}  common.Response{data=bool} "编辑成功与否"
// @Failure      400  {object}  common.Response "查询失败，详情见响应中的code"
// @Router       /v1/spaceUser/edit [POST]
func EditSpaceUser(c *gin.Context) {
	req := reqSpaceUser.SpaceUserEditRequest{}
	if err := c.ShouldBind(&req); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}
	suc, err := sSpaceUser.EditSpaceUser(&req)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, suc)
}

// ListSpaceUser godoc
// @Summary      查询成员信息列表
// @Tags         spaceUser
// @Accept       json
// @Produce      json
// @Param		request body reqSpaceUser.SpaceUserQueryRequest true "可以携带的参数"
// @Success      200  {object}  common.Response{data=[]resSpaceUser.SpaceUserVO} "返回详细数据"
// @Failure      400  {object}  common.Response "查询失败，详情见响应中的code"
// @Router       /v1/spaceUser/list [POST]
func ListSpaceUser(c *gin.Context) {
	req := reqSpaceUser.SpaceUserQueryRequest{}
	if err := c.ShouldBind(&req); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}
	spaceVOList, err := sSpaceUser.ListSpaceUserVO(&req)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, spaceVOList)
}

// GetSpaceUser godoc
// @Summary      查询某个成员在某个空间的信息
// @Tags         spaceUser
// @Accept       json
// @Produce      json
// @Param		request body reqSpaceUser.SpaceUserQueryRequest true "必须携带spaceID和userID"
// @Success      200  {object}  common.Response{data=entity.SpaceUser} "返回成功与否"
// @Failure      400  {object}  common.Response "查询失败，详情见响应中的code"
// @Router       /v1/spaceUser/get [POST]
func GetSpaceUser(c *gin.Context) {
	req := reqSpaceUser.SpaceUserQueryRequest{}
	if err := c.ShouldBind(&req); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}
	query, err := sSpaceUser.GetQueryWrapper(mysql.LoadDB(), &req)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	spaceUser := &entity.SpaceUser{}
	originErr := query.First(spaceUser).Error
	if originErr != nil {
		if originErr == gorm.ErrRecordNotFound {
			common.BaseResponse(c, nil, "没有找到该空间成员", ecode.PARAMS_ERROR)
			return
		}
		common.BaseResponse(c, nil, "查询失败", ecode.SYSTEM_ERROR)
		return
	}
	common.Success(c, *spaceUser)
}

// ListMyTeamSpace godoc
// @Summary      查询我加入的团队空间列表
// @Tags         spaceUser
// @Accept       json
// @Produce      json
// @Success      200  {object}  common.Response{data=[]resSpaceUser.SpaceUserVO} "返回详细数据"
// @Failure      400  {object}  common.Response "查询失败，详情见响应中的code"
// @Router       /v1/spaceUser/list/my [POST]
func ListMyTeamSpace(c *gin.Context) {
	loginUser, err := sUser.GetLoginUser(c)
	if err != nil {
		common.BaseResponse(c, nil, "获取登录用户失败", ecode.NOT_LOGIN_ERROR)
		return
	}
	req := &reqSpaceUser.SpaceUserQueryRequest{
		UserID: loginUser.ID,
	}
	spaceVOList, err := sSpaceUser.ListSpaceUserVO(req)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, spaceVOList)
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
