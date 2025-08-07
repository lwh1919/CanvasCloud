package repository

import (
	"gorm.io/gorm"
	"backend/pkg/mysql"
)

type SpaceAnalyzeRepository struct {
	db *gorm.DB
}

// 开启事务
func (r *SpaceAnalyzeRepository) BeginTransaction() *gorm.DB {
	return r.db.Begin()
}

func NewSpaceAnalyzeRepository() *SpaceAnalyzeRepository {
	return &SpaceAnalyzeRepository{mysql.LoadDB()}
}
