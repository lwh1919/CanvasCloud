package entity

import (
	"backend/pkg/snowflake"
	"time"

	"gorm.io/gorm"
)

type ITask struct {
	ID             uint64         `gorm:"primaryKey;comment:id" json:"id,string" swaggertype:"string"`
	Name           string         `gorm:"type:varchar(128);comment:任务名称;index:idx_name_status,priority:1" json:"name"`
	Prompt         string         `gorm:"type:text;comment:用户扩图提示词" json:"prompt"`
	OriginalPicUrl string         `gorm:"type:varchar(512);comment:原图URL" json:"originalPicUrl"`
	ExpandedPicUrl string         `gorm:"type:varchar(512);comment:扩展图URL;default:''" json:"expandedPicUrl"`
	PictureId      uint64         `gorm:"comment:拓展图ID;index:idx_picture_user,priority:2" json:"pictureId,string" swaggertype:"string"`
	AIRecap        string         `gorm:"type:text;comment:AI返回的扩图说明" json:"aiRecap"`
	ExecMessage    string         `gorm:"type:text;comment:执行消息" json:"execMessage"`
	Status         string         `gorm:"type:varchar(32);default:'wait';comment:任务状态: wait/running/succeed/failed;index:idx_user_status,priority:2;index:idx_status_time,priority:1;index:idx_name_status,priority:2" json:"status"`
	UserID         uint64         `gorm:"comment:用户ID;index:idx_user_status,priority:1;index:idx_user_time,priority:1;index:idx_picture_user,priority:1" json:"userId,string" swaggertype:"string"`
	ExpandParams   string         `gorm:"type:json;comment:扩图参数配置" json:"expandParams"`
	CreateTime     time.Time      `gorm:"autoCreateTime;comment:创建时间;index:idx_user_time,priority:2;index:idx_status_time,priority:2" json:"createTime"`
	UpdateTime     time.Time      `gorm:"autoUpdateTime;comment:更新时间;index:idx_user_time,priority:3" json:"updateTime"`
	IsDelete       gorm.DeletedAt `gorm:"comment:是否删除;index" swaggerignore:"true" json:"isDelete" swaggerignore:"true"`
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
