package space

import (
	"backend/internal/common"
	"backend/internal/model/entity"
)

type ListSpaceResponse struct {
	common.PageResponse
	Records []entity.Space `json:"records"`
}
