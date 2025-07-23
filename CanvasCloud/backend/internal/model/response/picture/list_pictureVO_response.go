package picture

import "web_app2/internal/common"

type ListPictureVOResponse struct {
	common.PageResponse
	Records []PictureVO `json:"records" `
}
