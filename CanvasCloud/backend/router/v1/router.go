package v1

import (
	"github.com/gin-gonic/gin"
	"web_app2/internal/consts"
	"web_app2/internal/controller"
	"web_app2/internal/midwares"
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
		userAPI.GET("/get/login", controller.GetLoginUser)
		userAPI.POST("/logout", controller.UserLogout)
		userAPI.GET("/get/vo", controller.GetUserVOById)
		//以下需要权限
		userAPI.POST("/list/page/vo", midwares.AuthCheck(consts.ADMIN_ROLE), controller.ListUserVOByPage)
		userAPI.POST("/update", midwares.AuthCheck(consts.ADMIN_ROLE), controller.UpdateUser)
		userAPI.POST("/delete", midwares.AuthCheck(consts.ADMIN_ROLE), controller.DeleteUser)
		userAPI.POST("/add", midwares.AuthCheck(consts.ADMIN_ROLE), controller.AddUser)
		userAPI.GET("/get", midwares.AuthCheck(consts.ADMIN_ROLE), controller.GetUserById)
		userAPI.POST("/avatar", midwares.LoginCheck(), controller.UploadAvatar)
		userAPI.POST("/edit", midwares.LoginCheck(), controller.EditUser)
	}
}

func registerSpaceRoutes(apiV1 *gin.RouterGroup) {
	// @Tags Space
	spaceAPI := apiV1.Group("/space")
	{
		spaceAPI.POST("/update", midwares.AuthCheck(consts.ADMIN_ROLE), controller.UpdateSpace)
		spaceAPI.POST("/edit", controller.EditSpace)
		spaceAPI.POST("/list/page", midwares.AuthCheck(consts.ADMIN_ROLE), controller.ListSpaceByPage)
		spaceAPI.POST("/list/page/vo", controller.ListSpaceVOByPage)
		spaceAPI.POST("/add", midwares.LoginCheck(), controller.AddSpace)
		spaceAPI.GET("/list/level", controller.ListSpaceLevel)
		spaceAPI.GET("/get/vo", midwares.LoginCheck(), controller.GetSpaceVOById)
	}
}

func registerSpaceUserRoutes(apiV1 *gin.RouterGroup) {
	// @Tags SpaceUser
	spaceUserAPI := apiV1.Group("/spaceUser")
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
	spaceAnalyzeAPI := apiV1.Group("/space/analyze")
	{
		spaceAnalyzeAPI.POST("/usage", midwares.LoginCheck(), controller.GetSpaceUsageAnalyze)
		spaceAnalyzeAPI.POST("/category", midwares.LoginCheck(), controller.GetSpaceCategoryAnalyze)
		spaceAnalyzeAPI.POST("/tag", midwares.LoginCheck(), controller.GetSpaceTagAnalyze)
		spaceAnalyzeAPI.POST("/size", midwares.LoginCheck(), controller.GetSpaceSizeAnalyze)
		spaceAnalyzeAPI.POST("/user", midwares.LoginCheck(), controller.GetSpaceUserAnalyze)
		spaceAnalyzeAPI.POST("/rank", midwares.AuthCheck(consts.ADMIN_ROLE), controller.GetSpaceRankAnalyze)
	}
}

func registerFileRoutes(apiV1 *gin.RouterGroup) {
	// @Tags File
	fileAPI := apiV1.Group("/file")
	{
		fileAPI.POST("/test/upload", midwares.AuthCheck(consts.ADMIN_ROLE), controller.TestUploadFile)
		fileAPI.GET("/test/download", midwares.AuthCheck(consts.ADMIN_ROLE), controller.TestDownloadFile)
	}
}

func registerPictureRoutes(apiV1 *gin.RouterGroup) {
	// @Tags Picture
	pictureAPI := apiV1.Group("/picture")
	{
		pictureAPI.POST("/upload", midwares.LoginCheck(), controller.UploadPicture)
		pictureAPI.POST("/upload/url", midwares.LoginCheck(), controller.UploadPictureByUrl)
		pictureAPI.POST("/upload/batch", midwares.AuthCheck(consts.ADMIN_ROLE), controller.UploadPictureByBatch)
		pictureAPI.POST("/delete", midwares.LoginCheck(), controller.DeletePicture)
		pictureAPI.POST("/update", midwares.LoginCheck(), controller.UpdatePicture)
		pictureAPI.POST("/edit", midwares.LoginCheck(), controller.EditPicture)
		pictureAPI.GET("/get", midwares.AuthCheck(consts.ADMIN_ROLE), controller.GetPictureById)
		pictureAPI.GET("/get/vo", controller.GetPictureVOById)
		pictureAPI.POST("/list/page", midwares.AuthCheck(consts.ADMIN_ROLE), controller.ListPictureByPage)
		pictureAPI.POST("/list/page/vo", controller.ListPictureVOByPage)
		pictureAPI.POST("/list/page/vo/cache", controller.ListPictureVOByPageWithCache)
		pictureAPI.GET("/tag_category", controller.ListPictureTagCategory)
		pictureAPI.POST("/review", midwares.AuthCheck(consts.ADMIN_ROLE), controller.DoPictureReview)
		pictureAPI.POST("/search/picture", controller.SearchPictureByPicture)
		pictureAPI.POST("/search/color", midwares.LoginCheck(), controller.SearchPictureByColor)
		pictureAPI.POST("/edit/batch", midwares.LoginCheck(), controller.PictureEditByBatch)
		pictureAPI.POST("/out_painting/create_task", midwares.LoginCheck(), controller.CreatePictureOutPaintingTask)
		pictureAPI.GET("/out_painting/create_task", midwares.LoginCheck(), controller.GetOutPaintingTaskResponse)
	}
}
