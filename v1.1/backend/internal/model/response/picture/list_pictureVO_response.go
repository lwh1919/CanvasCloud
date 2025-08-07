package picture

import "backend/internal/common"

type ListPictureVOResponse struct {
	common.PageResponse
	Records []PictureVO `json:"records" `
}
