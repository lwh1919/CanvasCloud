package v1

import (
	"backend/internal/consts"
	"backend/internal/controller"
	"backend/internal/midwares"
	"github.com/gin-gonic/gin"
)

// RegisterV1Routes 注册v1版本所有路由
func RegisterV1Routes(apiV1 *gin.RouterGroup) {
	registerUserRoutes(apiV1)
	registerSpaceRoutes(apiV1)
	registerSpaceUserRoutes(apiV1)
	registerSpaceAnalyzeRoutes(apiV1)
	registerFileRoutes(apiV1)
	registerPictureRoutes(apiV1)
}

func registerUserRoutes(apiV1 *gin.RouterGroup) {
	// @Tags User
	userAPI := apiV1.Group("/user")
	{
		userAPI.POST("/register", controller.UserRegister)
		userAPI.POST("/login", controller.UserLogin)
		userAPI.GET("/get/login", midwares.JWTAuthMiddleware(), controller.GetLoginUser)
		userAPI.POST("/logout", controller.UserLogout)
		userAPI.GET("/get/vo", controller.GetUserVOById)
		//以下需要权限
		userAPI.POST("/list/page/vo", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.ListUserVOByPage)
		userAPI.POST("/update", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.UpdateUser)
		userAPI.POST("/delete", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.DeleteUser)
		userAPI.POST("/add", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.AddUser)
		userAPI.GET("/get", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.GetUserById)
		userAPI.POST("/avatar", midwares.JWTAuthMiddleware(), controller.UploadAvatar)
		userAPI.POST("/edit", midwares.JWTAuthMiddleware(), controller.EditUser)
	}
}

func registerSpaceRoutes(apiV1 *gin.RouterGroup) {
	// @Tags Space
	spaceAPI := apiV1.Group("/space")
	{
		spaceAPI.POST("/update", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.UpdateSpace)
		spaceAPI.POST("/edit", midwares.JWTAuthMiddleware(), controller.EditSpace)
		spaceAPI.POST("/list/page", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.ListSpaceByPage)
		spaceAPI.POST("/list/page/vo", midwares.JWTAuthMiddleware(), controller.ListSpaceVOByPage)
		spaceAPI.POST("/add", midwares.JWTAuthMiddleware(), midwares.JWTAuthMiddleware(), controller.AddSpace)
		spaceAPI.GET("/list/level", controller.ListSpaceLevel)
		spaceAPI.GET("/get/vo", midwares.JWTAuthMiddleware(), midwares.JWTAuthMiddleware(), controller.GetSpaceVOById)
	}
}

func registerSpaceUserRoutes(apiV1 *gin.RouterGroup) {
	// @Tags SpaceUser
	spaceUserAPI := apiV1.Group("/spaceUser", midwares.JWTAuthMiddleware())
	{
		spaceUserAPI.POST("/add", midwares.CasbinAuthCheck(consts.DOM_SPACE, consts.OBJ_SPACEUSER, consts.ACT_SPACEUSER_MANAGE), controller.AddSpaceUser)
		spaceUserAPI.POST("/delete", midwares.CasbinAuthCheck(consts.DOM_SPACE, consts.OBJ_SPACEUSER, consts.ACT_SPACEUSER_MANAGE), controller.DeleteSpaceUser)
		spaceUserAPI.POST("/get", controller.GetSpaceUser)
		spaceUserAPI.POST("/list", controller.ListSpaceUser)
		spaceUserAPI.POST("/edit", midwares.CasbinAuthCheck(consts.DOM_SPACE, consts.OBJ_SPACEUSER, consts.ACT_SPACEUSER_MANAGE), controller.EditSpaceUser)
		spaceUserAPI.POST("/list/my", controller.ListMyTeamSpace)
	}
}

func registerSpaceAnalyzeRoutes(apiV1 *gin.RouterGroup) {
	// @Tags SpaceAnalyze
	spaceAnalyzeAPI := apiV1.Group("/space/analyze", midwares.JWTAuthMiddleware())
	{
		spaceAnalyzeAPI.POST("/usage", controller.GetSpaceUsageAnalyze)
		spaceAnalyzeAPI.POST("/category", midwares.JWTAuthMiddleware(), controller.GetSpaceCategoryAnalyze)
		spaceAnalyzeAPI.POST("/tag", controller.GetSpaceTagAnalyze)
		spaceAnalyzeAPI.POST("/size", controller.GetSpaceSizeAnalyze)
		spaceAnalyzeAPI.POST("/user", controller.GetSpaceUserAnalyze)
		spaceAnalyzeAPI.POST("/rank", midwares.AuthCheck(consts.ADMIN_ROLE), controller.GetSpaceRankAnalyze)
	}
}

func registerFileRoutes(apiV1 *gin.RouterGroup) {
	// @Tags File
	fileAPI := apiV1.Group("/file", midwares.JWTAuthMiddleware())
	{
		fileAPI.POST("/test/upload", midwares.AuthCheck(consts.ADMIN_ROLE), controller.TestUploadFile)
		fileAPI.GET("/test/download", midwares.AuthCheck(consts.ADMIN_ROLE), controller.TestDownloadFile)
	}
}

func registerPictureRoutes(apiV1 *gin.RouterGroup) {
	// @Tags Picture
	pictureAPI := apiV1.Group("/picture")
	{
		pictureAPI.POST("/upload", midwares.JWTAuthMiddleware(), controller.UploadPicture)
		pictureAPI.POST("/upload/url", midwares.JWTAuthMiddleware(), controller.UploadPictureByUrl)
		pictureAPI.POST("/upload/batch", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.UploadPictureByBatch)
		pictureAPI.POST("/delete", midwares.JWTAuthMiddleware(), controller.DeletePicture)
		pictureAPI.POST("/update", midwares.JWTAuthMiddleware(), controller.UpdatePicture)
		pictureAPI.POST("/edit", midwares.JWTAuthMiddleware(), controller.EditPicture)
		pictureAPI.GET("/get", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.GetPictureById)
		pictureAPI.GET("/get/vo", midwares.JWTAuthMiddleware(), controller.GetPictureVOById)
		pictureAPI.POST("/list/page", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.ListPictureByPage)
		pictureAPI.POST("/list/page/vo", controller.ListPictureVOByPage)
		pictureAPI.POST("/list/page/vo/cache", controller.ListPictureVOByPageWithCache)
		pictureAPI.POST("/list/page/vo/procache", controller.ProListPictureVOByPageWithCache)
		pictureAPI.GET("/tag_category", controller.ListPictureTagCategory)
		pictureAPI.POST("/review", midwares.JWTAuthMiddleware(), midwares.AuthCheck(consts.ADMIN_ROLE), controller.DoPictureReview)
		pictureAPI.POST("/search/picture", midwares.JWTAuthMiddleware(), controller.SearchPictureByPicture)
		// 修复：移除颜色搜索的重复JWTAuthMiddleware
		pictureAPI.POST("/search/color", midwares.JWTAuthMiddleware(), controller.SearchPictureByColor)
		pictureAPI.POST("/edit/batch", midwares.JWTAuthMiddleware(), controller.PictureEditByBatch)
		pictureAPI.POST("/out_painting/create_task", midwares.JWTAuthMiddleware(), controller.CreatePictureOutPaintingTask)
		pictureAPI.GET("/out_painting/create_task", midwares.JWTAuthMiddleware(), controller.GetOutPaintingTaskResponse)
		pictureAPI.POST("/out_painting/procreate_task", midwares.JWTAuthMiddleware(), controller.ProCreatePictureOutPaintingTask)
		pictureAPI.GET("/out_painting/list", midwares.JWTAuthMiddleware(), controller.GetOutPaintingTaskListResponse)
		pictureAPI.POST("/out_painting/delete_task/:id", midwares.JWTAuthMiddleware(), controller.DeleteImageExpandTask)

	}
}
