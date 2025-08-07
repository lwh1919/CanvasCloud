package user

import "backend/internal/common"

type ListUserVOResponse struct {
	common.PageResponse
	Records []UserVO `json:"records" `
}
