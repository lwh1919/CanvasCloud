package controller

import (
	"web_app2/internal/service"
)

func Init() {
	sPicture = service.NewPictureService()
	sSpaceAnalyze = service.NewSpaceAnalyzeService()
	sSpace = service.NewSpaceService()
	sSpaceUser = service.NewSpaceUserService()
	sUser = service.NewUserService()
}
