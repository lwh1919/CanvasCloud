package controller

import (
	"backend/internal/service"
)

func Init() {
	sPicture = service.NewPictureService()
	sSpaceAnalyze = service.NewSpaceAnalyzeService()
	sSpace = service.NewSpaceService()
	sSpaceUser = service.NewSpaceUserService()
	sUser = service.NewUserService()
	sITask = service.NewITaskService()
}
