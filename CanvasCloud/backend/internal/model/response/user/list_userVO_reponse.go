package user

import "web_app2/internal/common"

type ListUserVOResponse struct {
	common.PageResponse
	Records []UserVO `json:"records" `
}
