package image_expand_task

import "time"

type ITaskVO struct {
	ID             uint64    `json:"id,string" swaggertype:"string"`                  // ID保持原样
	Name           string    `json:"name"`                                            // 任务名称
	Prompt         string    `json:"prompt,omitempty"`                                // 【脱敏】用户提示词（敏感时可omitempty）
	OriginalPicUrl string    `json:"originalPicUrl"`                                  // 原图URL（可考虑返回脱敏后的CDN地址）
	ExpandedPicUrl string    `json:"expandedPicUrl,omitempty"`                        // 结果图URL（未生成时隐藏）
	PictureId      uint64    `json:"pictureId,string,omitempty" swaggertype:"string"` // 关联图片ID（非必返）
	AIRecap        string    `json:"aiRecap,omitempty"`                               // AI说明（可能含敏感信息）
	ExecMessage    string    `json:"execMessage,omitempty"`                           // 执行错误时才返回
	Status         string    `json:"status"`                                          // 任务状态（必须返回）
	ExpandParams   string    `json:"expandParams,omitempty"`                          // 参数配置（内部用可脱敏）
	CreateTime     time.Time `json:"createTime"`                                      // 【重要】VO建议增加时间字段
}
