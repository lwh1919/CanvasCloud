package space

type SpaceEditRequest struct {
	ID        uint64 `json:"id,string" swaggertype:"string"` // Space ID
	SpaceName string `json:"spaceName"`                      // Space name
}
