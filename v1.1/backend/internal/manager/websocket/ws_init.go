package websocket

import "backend/internal/service"

func Init() {
	userService = service.NewUserService()
	pictureService = service.NewPictureService()
	spaceService = service.NewSpaceService()
}
