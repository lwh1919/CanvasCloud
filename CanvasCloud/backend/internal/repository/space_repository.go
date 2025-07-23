package repository

import (
	"gorm.io/gorm"
	"web_app2/internal/model/entity"
	"web_app2/pkg/mysql"
)

type SpaceRepository struct {
	db *gorm.DB
}

func NewSpaceRepository() *SpaceRepository {
	return &SpaceRepository{mysql.LoadDB()}
}
func (r *SpaceRepository) BeginTransaction() *gorm.DB {
	return r.db.Begin()
}
func (r *SpaceRepository) GetSpaceById(tx *gorm.DB, id uint64) (*entity.Space, error) {
	if tx == nil {
		tx = r.db.Begin()
	}
	space := &entity.Space{}
	err := tx.Where("id = ?", id).First(space).Error
	//区分记录不存在还是查询异常
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return space, nil
}

func (r *SpaceRepository) UpdateSpaceById(tx *gorm.DB, id uint64, updateMap map[string]interface{}) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Model(&entity.Space{}).Where("id = ?", id).Updates(updateMap).Error
}

// 根据空间id判断空间是否存在
func (r *SpaceRepository) IsExistById(tx *gorm.DB, id uint64) (bool, error) {
	if tx == nil {
		tx = r.db
	}
	var count int64
	err := tx.Model(&entity.Space{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// 根据用户ID判断空间是否存在，需要判断是私有空间还是团队空间
// 推荐使用Count计数法的完整实现
func (r *SpaceRepository) IsExistByUserId(tx *gorm.DB, userId uint64, spaceType int) bool {

	if tx == nil {
		tx = r.db
	}

	var count int64
	tx.Model(&entity.Space{}).Where("user_id = ? AND space_type = ?", userId, spaceType).Count(&count)

	return count > 0
}
func (r *SpaceRepository) SaveSpace(tx *gorm.DB, space *entity.Space) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Save(space).Error
}

//nihao
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
//1
//nihao1
