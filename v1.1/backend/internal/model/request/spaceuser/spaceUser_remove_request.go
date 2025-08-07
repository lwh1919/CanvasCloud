package spaceuser

type SpaceUserRemoveRequest struct {
	ID uint64 `json:"Id,string" binding:"required" swaggertype:"string"` //表的元组ID
}
