package websocket

import "web_app2/internal/service"

func Init() {
	userService = service.NewUserService()
	pictureService = service.NewPictureService()
	spaceService = service.NewSpaceService()
}
