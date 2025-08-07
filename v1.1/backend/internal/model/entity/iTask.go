package entity

import (
	"backend/pkg/snowflake"
	"gorm.io/gorm"
	"time"
)

type ITask struct {
	ID             uint64         `gorm:"primaryKey;comment:id" json:"id,string" swaggertype:"string"`
	Name           string         `gorm:"type:varchar(128);comment:任务名称" json:"name"`
	Prompt         string         `gorm:"type:text;comment:用户扩图提示词" json:"prompt"`
	OriginalPicUrl string         `gorm:"type:varchar(512);comment:原图URL" json:"originalPicUrl"`             // 修正：改为varchar类型并移除,string
	ExpandedPicUrl string         `gorm:"type:varchar(512);comment:扩展图URL;default:''" json:"expandedPicUrl"` // 修正：改为varchar类型并移除,string
	PictureId      uint64         `gorm:"comment:拓展图ID" json:"pictureId,string" swaggertype:"string"`
	AIRecap        string         `gorm:"type:text;comment:AI返回的扩图说明" json:"aiRecap"`
	ExecMessage    string         `gorm:"type:text;comment:执行消息" json:"execMessage"`
	Status         string         `gorm:"type:varchar(32);default:'wait';comment:任务状态: wait/running/succeed/failed" json:"status"`
	UserID         uint64         `gorm:"comment:用户ID" json:"userId,string" swaggertype:"string"`
	ExpandParams   string         `gorm:"type:json;comment:扩图参数配置" json:"expandParams"`
	CreateTime     time.Time      `gorm:"autoCreateTime;comment:创建时间" json:"createTime"`
	UpdateTime     time.Time      `gorm:"autoUpdateTime;comment:更新时间" json:"updateTime"`
	IsDelete       gorm.DeletedAt `gorm:"comment:是否删除" swaggerignore:"true" json:"isDelete" swaggerignore:"true"`
}

// AutoMigratePicture 执行数据库迁移
func AutoMigrateITask(db *gorm.DB) {
	err := db.AutoMigrate(&ITask{})
	if err != nil {
		panic("⚠️ 用户表迁移失败: " + err.Error())
	}
}

// 钩子，使用sonyflake生成ID
func (p *ITask) BeforeTask(tx *gorm.DB) error {
	if p.ID == 0 {
		id, _ := snowflake.GenID()
		p.ID = id
	}
	return nil
}
