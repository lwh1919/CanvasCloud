package spaceuser

// 空间管理员使用，编辑空间角色
// 最小必要字段原则和操作场景分离原则
type SpaceUserEditRequest struct {
	ID        uint64 `json:"Id,string" swaggertype:"string"` //表的元组ID
	SpaceRole string `json:"spaceRole"`                      //空间角色：viewer-查看者 editor-编辑者 admin-管理员
}
