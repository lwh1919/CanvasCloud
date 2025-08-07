package iTask

type TaskRequest struct {
	ImageURL    string  `json:"imageUrl" binding:"required,url"` // 图片URL（必需，需验证URL格式）
	Prompt      string  `json:"prompt" binding:"required"`       // 用户提示词（必需）
	ExpandRatio float64 `json:"expandRatio,omitempty"`           // 扩展比例（可选）
	Direction   string  `json:"direction,omitempty"`             // 扩展方向（可选，如：left/right/top/bottom）
}
