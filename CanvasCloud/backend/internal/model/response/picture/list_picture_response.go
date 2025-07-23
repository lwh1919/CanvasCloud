package picture

import (
	"web_app2/internal/common"
	"web_app2/internal/model/entity"
)

type ListPictureResponse struct {
	common.PageResponse
	Records []entity.Picture `json:"records" `
}
