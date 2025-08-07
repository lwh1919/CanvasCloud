package controller

import (
	"backend/internal/common"
	"backend/internal/ecode"
	iTaskReq "backend/internal/model/request/iTask"
	iTaskRes "backend/internal/model/response/iTask"
	"backend/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

func dumb5() {
	_ = iTaskRes.ITaskVO{}
}

var sITask *service.ITaskService

// ProCreatePictureOutPaintingTask
// @Summary      上传ai扩图图片任务PRO「需要登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param        request body iTask.TaskRequest true "任务"
// @Success      200  {object}  common.Response{data=bool} "上传成功，返回任务信息视图"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/out_painting/procreate_task [POST]
// @Security BearerAuth
func ProCreatePictureOutPaintingTask(c *gin.Context) {
	var req iTaskReq.TaskRequest
	if err := c.ShouldBind(&req); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}

	// 获取当前用户ID
	loginUser, _ := sUser.GetLoginUser(c)

	err := sITask.ProCreatePictureOutPaintingTask(&req, loginUser.ID)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}

	common.Success(c, true)
}

// GetOutPaintingTaskListResponse
// @Summary      获取ai请求任务视图「需要登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Success      200  {object}  common.Response{data=[]iTaskRes.ITaskVO} "获取成功，返回图片信息视图"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/out_painting/list [GET]
// @Security BearerAuth
func GetOutPaintingTaskListResponse(c *gin.Context) {
	// 获取当前用户ID
	loginUser, _ := sUser.GetLoginUser(c)

	res, err := sITask.GetITaskVOList(loginUser.ID)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}

	common.Success(c, res)
}

// DeleteImageExpandTask
// @Summary     删除任务视图「需要登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Success      200  {object}  common.Response{data=bool} "上传成功，返回图片信息视图"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/out_painting/delete_task/:id [POST]
// @Security BearerAuth
func DeleteImageExpandTask(c *gin.Context) {
	taskId := c.Param("id")
	id, oerr := strconv.ParseUint(taskId, 10, 64)
	if oerr != nil {
		common.BaseResponse(c, nil, "无效的任务ID", ecode.PARAMS_ERROR)
		return
	}

	// 获取当前用户ID
	loginUser, _ := sUser.GetLoginUser(c)

	// 验证任务归属
	task, oerr := sITask.ITaskRepo.FindById(nil, id)
	if oerr != nil || task == nil || task.UserID != loginUser.ID {
		common.BaseResponse(c, nil, oerr.Error(), ecode.OPERATION_ERROR)
		return
	}

	if err := sITask.DeleteImageExpandTask(id); err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}

	common.Success(c, true)
}
