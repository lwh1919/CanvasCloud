package space

import "web_app2/internal/common"

type ListSpaceVOResponse struct {
	common.PageResponse
	Records []SpaceVO `json:"records"`
}
