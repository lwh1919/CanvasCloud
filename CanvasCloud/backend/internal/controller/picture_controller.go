package controller

import (
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
	"web_app2/internal/api/imagesearch"
	"web_app2/internal/common"
	"web_app2/internal/consts"
	"web_app2/internal/ecode"
	"web_app2/internal/model/entity"
	reqPicture "web_app2/internal/model/request/picture"
	resPicture "web_app2/internal/model/response/picture"
	"web_app2/internal/service"
	//resPicture "CanvasCloud/internal/models/response/picture"
	imgSearchModel "web_app2/internal/api/imagesearch/model"
)

func dumb2() {
	_ = imgSearchModel.ImageSearchResult{}
}

var sPicture *service.PictureService

// 给忘记了，wc
// Query String (查询参数)	URL ? 后拼接的键值对	c.Query("key")	搜索过滤、分页参数	/api/users?page=2&limit=10
// Path Parameter (路径参数)	URL 路径中的变量段	c.Param("key")	RESTful API 资源标识	/api/users/:id
// Form Data (表单数据)	POST 请求体中的键值对（包括文件）	c.PostForm("key")/c.FormFile()	HTML 表单提交、文件上传	<form enctype="multipart/form-data">
// // JSON Body (JSON 主体)	POST/PUT 请求体中的 JSON 结构	c.ShouldBindJSON(&obj)	API 交互、复杂数据结构	{"name":"John", "age":30}
// ShouldBind：智能检测内容类型（JSON/XML/表单等）
// ShouldBindQuery：仅绑定查询字符串
// ShouldBindJSON：绑定JSON请求体
// ShouldBindUri：绑定路径参数
// ShouldBindWith：根据指定绑定器绑定

//文件必须使用c.FormFile（单个文件）或c.MultipartForm（多个文件）来处理。

// UploadPicture godoc
// @Summary      上传图片接口「需要登录校验」
// @Description  根据是否存在ID来上传图片或者修改图片信息，返回图片信息视图
// @Tags         picture
// @Accept       mpfd
// @Produce      json
// @Param        file formData file true "图片"
// @Param        id formData string false "图片的ID，非必需"
// @Param        spaceId formData string false "图片的上传空间ID，非必需"
// @Success      200  {object}  common.Response{data=resPicture.PictureVO} "上传成功，返回图片信息视图"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/upload [POST]
func UploadPicture(c *gin.Context) {
	file, _ := c.FormFile("file")
	// 手动解析表单参数
	id, _ := strconv.ParseUint(c.PostForm("id"), 10, 64)
	spaceId, _ := strconv.ParseUint(c.PostForm("spaceId"), 10, 64)
	picReq := &reqPicture.PictureUploadRequest{
		ID:      id,      // 获取 id
		SpaceID: spaceId, // 获取 spaceId
	}
	loginUser, err := sUser.GetLoginUser(c)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	picVO, err := sPicture.UploadPicture(file, picReq, loginUser)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, *picVO)
}

// UploadPictureByUrl godoc
// @Summary      根据URL上传图片接口「需要登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param        request body reqPicture.PictureUploadRequest true "图片URL"
// @Success      200  {object}  common.Response{data=resPicture.PictureVO} "上传成功，返回图片信息视图"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/upload/url [POST]
func UploadPictureByUrl(c *gin.Context) {
	picReq := &reqPicture.PictureUploadRequest{}
	c.ShouldBind(picReq)
	loginUser, err := sUser.GetLoginUser(c)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	//对于AI扩图，携带的query参数需要保留，因为一些具有时效性
	remove := strings.Contains(picReq.FileUrl, "OSSAccess")
	//对于一般网站，若picUrl包含了?解析参数，需要去掉
	if idx := strings.LastIndex(picReq.FileUrl, "?"); idx != -1 && !remove {
		picReq.FileUrl = picReq.FileUrl[:idx]
	}
	picVO, err := sPicture.UploadPicture(picReq.FileUrl, picReq, loginUser)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, *picVO)
}

// UploadPictureByBatch godoc
// @Summary      批量抓取图片「管理员」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param        request body reqPicture.PictureUploadByBatchRequest true "图片的关键词"
// @Success      200  {object}  common.Response{data=int} "返回抓取图片数量"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/upload/batch [POST]

func UploadPictureByBatch(c *gin.Context) {
	picReq := &reqPicture.PictureUploadByBatchRequest{}
	c.ShouldBind(picReq)
	//一定能获取
	loginUser, _ := sUser.GetLoginUser(c)
	cnt, err := sPicture.UploadPictureByBatch(picReq, loginUser)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, cnt)
}

// UpdatePicture godoc
// @Summary      更新图片「登录校验」
// @Description  若图片不存在，则返回false
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureUpdateRequest true "需要更新的图片信息"
// @Success      200  {object}  common.Response{data=bool} "更新成功"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/update [POST]
func UpdatePicture(c *gin.Context) {
	updateReq := reqPicture.PictureUpdateRequest{}
	c.ShouldBind(&updateReq)
	if updateReq.ID <= 0 {
		common.BaseResponse(c, false, "参数错误", ecode.PARAMS_ERROR)
		return
	}
	//获取登录用户，使用中间件保证可以获取到用户
	loginUser, _ := sUser.GetLoginUser(c)
	//更新操作，参数校验等在service层完成
	if err := sPicture.UpdatePicture(&updateReq, loginUser); err != nil {
		common.BaseResponse(c, false, err.Msg, err.Code)
		return
	}
	common.Success(c, true)
}

// DeletePicture godoc
// @Summary      根据ID软删除图片「登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		id body common.DeleteRequest true "图片的ID"
// @Success      200  {object}  common.Response{data=bool} "删除成功"
// @Failure      400  {object}  common.Response "删除失败，详情见响应中的code"
// @Router       /v1/picture/delete [POST]
func DeletePicture(c *gin.Context) {
	deleReq := common.DeleteRequest{}
	c.ShouldBind(&deleReq)
	if deleReq.Id <= 0 {
		common.BaseResponse(c, false, "删除失败，参数错误", ecode.PARAMS_ERROR)
		return
	}
	user, _ := sUser.GetLoginUser(c)
	if err := sPicture.DeletePicture(user, &deleReq); err != nil {
		common.BaseResponse(c, false, err.Msg, err.Code)
		return
	}
	common.Success(c, true)
}

// GetPictureById godoc
// @Summary      根据ID获取图片「管理员」
// @Tags         picture
// @Accept		json
// @Produce      json
// @Param		id query string true "图片的ID"
// @Success      200  {object}  common.Response{data=entity.Picture} "获取成功"
// @Failure      400  {object}  common.Response "获取失败，详情见响应中的code"
// @Router       /v1/picture/get [GET]
func GetPictureById(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	if id <= 0 {
		common.BaseResponse(c, nil, "<UNK>", ecode.PARAMS_ERROR)
		return
	}
	pic, err := sPicture.GetPictureById(id)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, *pic)
}

// GetPictureVOById godoc
// @Summary      根据ID获取脱敏的图片
// @Tags         picture
// @Accept		json
// @Produce      json
// @Param		id query string true "图片的ID"
// @Success      200  {object}  common.Response{data=resPicture.PictureVO} "获取成功"
// @Failure      400  {object}  common.Response "获取失败，详情见响应中的code"
// @Router       /v1/picture/get/vo [GET]
func GetPictureVOById(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	if id <= 0 {
		common.BaseResponse(c, nil, "<UNK>", ecode.PARAMS_ERROR)
		return
	}
	pic, err := sPicture.GetPictureById(id)

	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}

	picVO := sPicture.GetPictureVO(pic)

	loginUser, _ := sUser.GetLoginUser(c)

	if pic.SpaceID != 0 && loginUser == nil {
		common.BaseResponse(c, nil, "无权限", ecode.NO_AUTH_ERROR)
	}
	var space *entity.Space
	if picVO.SpaceID != 0 {
		space, _ = sSpace.GetSpaceById(picVO.SpaceID)
	}
	picVO.PermissionList = service.GetPermissionList(space, loginUser)
	//检查是否拥有读权限
	if len(picVO.PermissionList) == 0 {
		common.BaseResponse(c, nil, "没有权限", ecode.NO_AUTH_ERROR)
		return
	}
	common.Success(c, *picVO)
}

// 管理员

// ListPictureByPage godoc
// @Summary      分页获取一系列图片信息「管理员」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureQueryRequest true "需要查询的页数、以及图片关键信息"
// @Success      200  {object}  common.Response{data=resPicture.ListPictureResponse} "查询成功"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/list/page [POST]
func ListPictureByPage(c *gin.Context) {
	queryReq := reqPicture.PictureQueryRequest{}
	c.ShouldBind(&queryReq)
	//获取分页查询对象
	pics, err := sPicture.ListPictureByPage(&queryReq)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, pics)
}

// 前端获取

// ListPictureVOByPage godoc
// @Summary      分页获取一系列图片信息
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureQueryRequest true "需要查询的页数、以及图片关键信息"
// @Success      200  {object}  common.Response{data=resPicture.ListPictureVOResponse} "查询成功"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/list/page/vo [POST]
func ListPictureVOByPage(c *gin.Context) {
	queryReq := reqPicture.PictureQueryRequest{}
	c.ShouldBind(&queryReq)
	//限制爬虫
	if queryReq.PageSize > 20 {
		common.BaseResponse(c, nil, "最多允许获取20张/页", ecode.PARAMS_ERROR)
		return
	}
	//空间权限校验,放在service更合理
	if queryReq.SpaceID != 0 {
		//私有空间
		loginUser, err := sUser.GetLoginUser(c)
		if err != nil {
			common.BaseResponse(c, nil, err.Msg, err.Code)
			return
		}
		space, err := sSpace.GetSpaceById(queryReq.SpaceID)
		if err != nil {
			common.BaseResponse(c, nil, err.Msg, err.Code)
			return
		}
		//区分私有空间和团队空间
		switch space.SpaceType {
		case consts.SPACE_PRIVATE:
			if space.UserID != loginUser.ID {
				common.BaseResponse(c, nil, "无权限", ecode.NO_AUTH_ERROR)
				return
			}
		case consts.SPACE_TEAM:
			//团队空间，校验是否有权限
			permissions := service.GetPermissionList(space, loginUser)
			if len(permissions) == 0 {
				common.BaseResponse(c, nil, "无权限", ecode.NO_AUTH_ERROR)
				return
			}
		}
	} else {
		//公开图库
		//普通用户默认只允许查询过审图片
		if queryReq.ReviewStatus == nil {
			queryReq.ReviewStatus = new(int) //创建指针
		}
		*queryReq.ReviewStatus = consts.PASS
		queryReq.IsNullSpaceID = true
	}
	//获取分页查询对象
	pics, err := sPicture.ListPictureVOByPage(&queryReq)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	//若是空间的图片，则需要获取权限
	if len(pics.Records) != 0 {
		var space *entity.Space
		picVO := pics.Records[0]
		if picVO.SpaceID != 0 {
			space, _ = sSpace.GetSpaceById(picVO.SpaceID)
		}
		loginUser, _ := sUser.GetLoginUser(c)
		PermissionList := service.GetPermissionList(space, loginUser)
		//为每一个pic填充
		for idx := range pics.Records {
			pics.Records[idx].PermissionList = PermissionList
		}
	}
	common.Success(c, *pics)

}

// 带缓存的获取一些列图片信息

// ListPictureVOByPageWithCache godoc
// @Summary      带有缓存的分页获取一系列图片信息
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureQueryRequest true "需要查询的页数、以及图片关键信息"
// @Success      200  {object}  common.Response{data=resPicture.ListPictureVOResponse} "查询成功"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/list/page/vo/cache [POST]
func ListPictureVOByPageWithCache(c *gin.Context) {
	queryReq := reqPicture.PictureQueryRequest{}
	c.ShouldBind(&queryReq)
	//限制爬虫
	if queryReq.PageSize > 20 {
		common.BaseResponse(c, nil, "最多只允许获取20张/页", ecode.PARAMS_ERROR)
		return
	}
	//空间权限校验
	if queryReq.SpaceID != 0 {
		//私有空间
		loginUser, err := sUser.GetLoginUser(c)
		if err != nil {
			common.BaseResponse(c, nil, err.Msg, err.Code)
			return
		}
		space, err := sSpace.GetSpaceById(queryReq.SpaceID)
		if err != nil {
			common.BaseResponse(c, nil, err.Msg, err.Code)
			return
		}
		//区分私有空间和团队空间
		switch space.SpaceType {
		case consts.SPACE_PRIVATE:
			if space.UserID != loginUser.ID {
				common.BaseResponse(c, nil, "无权限", ecode.NO_AUTH_ERROR)
				return
			}
		case consts.SPACE_TEAM:
			//团队空间，校验是否有权限
			permissions := service.GetPermissionList(space, loginUser)
			if len(permissions) == 0 {
				common.BaseResponse(c, nil, "无权限", ecode.NO_AUTH_ERROR)
				return
			}
		}
	} else {
		//公开图库
		//普通用户默认只允许查询过审图片
		if queryReq.ReviewStatus == nil {
			queryReq.ReviewStatus = new(int) //创建指针
		}
		*queryReq.ReviewStatus = consts.PASS
		queryReq.IsNullSpaceID = true
	}
	//获取分页查询对象
	pics, err := sPicture.ListPictureVOByPageWithCache(&queryReq)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	//若是空间的图片，则需要获取权限
	if len(pics.Records) != 0 {
		var space *entity.Space
		picVO := pics.Records[0]
		if picVO.SpaceID != 0 {
			space, _ = sSpace.GetSpaceById(picVO.SpaceID)
		}
		loginUser, _ := sUser.GetLoginUser(c)
		PermissionList := service.GetPermissionList(space, loginUser)
		//为每一个pic填充
		for idx := range pics.Records {
			pics.Records[idx].PermissionList = PermissionList
		}
	}
	common.Success(c, *pics)
}

// 图片编辑功能

// EditPicture godoc
// @Summary      编辑图片
// @Description  若图片不存在，则返回false
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureEditRequest true "需要更新的图片信息"
// @Success      200  {object}  common.Response{data=bool} "更新成功"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/edit [POST]
func EditPicture(c *gin.Context) {
	//update和edit复用了同一个请求
	updateReq := reqPicture.PictureUpdateRequest{}
	c.ShouldBind(&updateReq)
	if updateReq.ID <= 0 {
		common.BaseResponse(c, false, "参数错误", ecode.PARAMS_ERROR)
		return
	}
	//校验是否本人或管理员操作
	user, _ := sUser.GetLoginUser(c)
	if user == nil {
		common.BaseResponse(c, false, "未登录", ecode.NOT_LOGIN_ERROR)
		return
	}
	//更新操作，参数校验和权限等在service层完成
	if err := sPicture.UpdatePicture(&updateReq, user); err != nil {
		common.BaseResponse(c, false, err.Msg, err.Code)
		return
	}
	common.Success(c, true)
}

// 获取固定标签

// ListPictureTagCategory godoc
// @Summary      获取图片的标签和分类（固定）
// @Tags         picture
// @Accept       json
// @Produce      json
// @Success      200  {object}  common.Response{data=resPicture.PictureTagCategory} "获取成功"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/tag_category [GET]
func ListPictureTagCategory(c *gin.Context) {
	tagCate := resPicture.PictureTagCategory{
		TagList:      []string{"热门", "搞笑", "生活", "高清", "艺术", "校园", "背景", "简历", "创意"},
		CategoryList: []string{"模板", "电商", "表情包", "素材", "海报"},
	}
	common.Success(c, tagCate)
}

// 执行图片审核

// DoPictureReview godoc
// @Summary      执行图片审核「管理员」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureReviewRequest true "审核图片所需信息"
// @Success      200  {object}  common.Response{data=bool} "审核更新成功"
// @Failure      400  {object}  common.Response "更新失败，详情见响应中的code"
// @Router       /v1/picture/review [POST]
func DoPictureReview(c *gin.Context) {
	var req reqPicture.PictureReviewRequest
	c.ShouldBind(&req)
	//获取当前登录的用户
	user, _ := sUser.GetLoginUser(c)
	if err := sPicture.DoPictureReview(&req, user); err != nil {
		common.BaseResponse(c, false, err.Msg, err.Code)
		return
	}
	common.Success(c, true)
}

// 根据ID的图片去百度搜索图片

// SearchPictureByPicture godoc
// @Summary      根据图片ID搜索图片
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureSearchByPictureRequest true "图片的ID"
// @Success      200  {object}  common.Response{data=[]imgSearchModel.ImageSearchResult} "获取成功"
// @Failure      400  {object}  common.Response "获取失败，详情见响应中的code"
// @Router       /v1/picture/search/picture [POST]
func SearchPictureByPicture(c *gin.Context) {
	var req reqPicture.PictureSearchByPictureRequest
	if err := c.ShouldBind(&req); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}
	if req.PictureId <= 0 {
		common.BaseResponse(c, nil, "参数错误", ecode.PARAMS_ERROR)
		return
	}
	oldPic, err := sPicture.GetPictureById(req.PictureId)
	if err != nil || oldPic == nil {
		common.BaseResponse(c, nil, "不存在该图片，或图片获取失败", ecode.PARAMS_ERROR)
		return
	}
	resultList, err := imagesearch.SearchImage(oldPic.URL)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, resultList)
}

// 根据颜色搜索在指定id空间图片

// SearchPictureByColor godoc
// @Summary      根据图片的颜色搜索相似图片「登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureSearchByColorRequest true "图片的颜色和空间ID"
// @Success      200  {object}  common.Response{data=[]resPicture.PictureVO} "获取成功"
// @Failure      400  {object}  common.Response "获取失败，详情见响应中的code"
// @Router       /v1/picture/search/color [POST]
func SearchPictureByColor(c *gin.Context) {
	var req reqPicture.PictureSearchByColorRequest
	if err := c.ShouldBind(&req); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}
	if req.PicColor == "" || req.SpaceID <= 0 {
		common.BaseResponse(c, nil, "参数错误", ecode.PARAMS_ERROR)
		return
	}
	loginUser, _ := sUser.GetLoginUser(c)
	resultList, err := sPicture.SearchPictureByColor(loginUser, req.PicColor, req.SpaceID)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, resultList)
}

// 批量修改图片

// PictureEditByBatch godoc
// @Summary      批量更新图片请求「登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.PictureEditByBatchRequest true "批量的图片ID、空间ID和分类和标签"
// @Success      200  {object}  common.Response{data=bool} "更新成功"
// @Failure      400  {object}  common.Response "获取失败，详情见响应中的code"
// @Router       /v1/picture/edit/batch [POST]
func PictureEditByBatch(c *gin.Context) {
	var req reqPicture.PictureEditByBatchRequest
	//定义中间结构体，解析[]string数组
	type middleReq struct {
		PictureIdList []string `json:"pictureIdList" swaggertype:"array,string"` // 图片ID列表
		SpaceID       uint64   `json:"spaceId,string" swaggertype:"string"`      //空间ID
		Category      string   `json:"category"`                                 //分类
		Tags          []string `json:"tags"`                                     //标签
		NameRule      string   `json:"nameRule"`                                 //名称规则，暂时只支持“名称{序号}的形式，序号将会自动递增”
	}
	var midReq middleReq
	if err := c.ShouldBind(&midReq); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}
	//转化成请求结构体
	var picIdList []uint64
	for _, idStr := range midReq.PictureIdList {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			common.BaseResponse(c, nil, "参数错误", ecode.PARAMS_ERROR)
			return
		}
		picIdList = append(picIdList, id)
	}
	req.PictureIdList = picIdList
	req.Category = midReq.Category
	req.SpaceID = midReq.SpaceID
	req.Tags = midReq.Tags
	req.NameRule = midReq.NameRule
	//获取登录用户，调用service
	loginUser, _ := sUser.GetLoginUser(c)
	suc, err := sPicture.PictureEditByBatch(&req, loginUser)
	if err != nil {
		common.BaseResponse(c, false, err.Msg, err.Code)
		return
	}
	common.Success(c, suc)
}

//采用异步请求：长响应

// 创建ai扩图任务请求

// CreatePictureOutPaintingTask godoc
// @Summary      创建AI扩图任务请求「登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		request body reqPicture.CreateOutPaintingTaskRequest true "创建扩图任务所需信息"
// @Success      200  {object}  common.Response{data=resPicture.CreateOutPaintingTaskResponse} "创建成功，返回任务信息"
// @Failure      400  {object}  common.Response "获取失败，详情见响应中的code"
// @Router       /v1/picture/out_painting/create_task [POST]
func CreatePictureOutPaintingTask(c *gin.Context) {
	var req reqPicture.CreateOutPaintingTaskRequest
	if err := c.ShouldBind(&req); err != nil {
		common.BaseResponse(c, nil, "参数绑定失败", ecode.PARAMS_ERROR)
		return
	}
	loginUser, _ := sUser.GetLoginUser(c)
	res, err := sPicture.CreatePictureOutPaintingTask(&req, loginUser)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, *res)
}

// 获取ai扩图任务的信息

// GetOutPaintingTaskResponse godoc
// @Summary      获取AI扩图任务信息「登录校验」
// @Tags         picture
// @Accept       json
// @Produce      json
// @Param		taskId query string true "任务的ID"
// @Success      200  {object}  common.Response{data=resPicture.GetOutPaintingResponse} "获取成功，返回任务进展信息"
// @Failure      400  {object}  common.Response "获取失败，详情见响应中的code"
// @Router       /v1/picture/out_painting/create_task [GET]
func GetOutPaintingTaskResponse(c *gin.Context) {
	taskId := c.Query("taskId")
	res, err := sPicture.GetOutPaintingTaskResponse(taskId)
	if err != nil {
		common.BaseResponse(c, nil, err.Msg, err.Code)
		return
	}
	common.Success(c, *res)

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
