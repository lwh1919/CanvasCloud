package repository

import (
	"gorm.io/gorm"
	"backend/pkg/mysql"
)

// 数据库操作层
type SpaceUserRepository struct {
	db *gorm.DB
}

// db.Begin() // 开始事务
// //db.Create(&order) // 操作1
// //db.Update(&user)  // 操作2
// //// 如果这里出错，需要手动db.Rollback()
// //db.Commit() // 提交
// 开启事务
func (r *SpaceUserRepository) BeginTransaction() *gorm.DB {
	return r.db.Begin()
}

func NewSpaceUserRepository() *SpaceUserRepository {
	return &SpaceUserRepository{mysql.LoadDB()}
}
