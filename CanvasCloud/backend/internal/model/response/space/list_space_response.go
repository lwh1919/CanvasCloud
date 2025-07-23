package space

import (
	"web_app2/internal/common"
	"web_app2/internal/model/entity"
)

type ListSpaceResponse struct {
	common.PageResponse
	Records []entity.Space `json:"records"`
}
