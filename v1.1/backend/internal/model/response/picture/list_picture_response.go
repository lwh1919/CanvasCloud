package picture

import (
	"backend/internal/common"
	"backend/internal/model/entity"
)

type ListPictureResponse struct {
	common.PageResponse
	Records []entity.Picture `json:"records" `
}
