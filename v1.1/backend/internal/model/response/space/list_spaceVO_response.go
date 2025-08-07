package space

import "backend/internal/common"

type ListSpaceVOResponse struct {
	common.PageResponse
	Records []SpaceVO `json:"records"`
}
