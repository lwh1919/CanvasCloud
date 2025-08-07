package repository

import (
	"errors"
	"gorm.io/gorm"
	"backend/internal/model/entity"
	"backend/pkg/mysql"
)

type PictureRepository struct {
	db *gorm.DB
}

func NewPictureRepository() *PictureRepository {
	return &PictureRepository{mysql.LoadDB()}
}

// 开启事务
func (r *PictureRepository) BeginTransaction() *gorm.DB {
	return r.db.Begin()
}

// 根据ID查找图片
func (r *PictureRepository) FindById(tx *gorm.DB, id uint64) (*entity.Picture, error) {
	if tx == nil {
		tx = r.db
	}
	var picture entity.Picture
	if err := tx.Where("id = ?", id).First(&picture).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 无记录
		}
		return nil, err // 数据库查询异常
	}
	return &picture, nil
}
func (r *PictureRepository) UpdateById(tx *gorm.DB, id uint64, updateMap map[string]interface{}) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Model(&entity.Picture{ID: id}).Updates(updateMap).Error
}
func (r *PictureRepository) DeleteById(tx *gorm.DB, id uint64) error {
	if tx == nil {
		tx = r.db
	}
	err := tx.Where("id = ?", id).Delete(&entity.Picture{}).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 无记录
		}
		return err
	}
	return nil
}
func (r *PictureRepository) SavePicture(tx *gorm.DB, picture *entity.Picture) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Save(picture).Error
}

// UpdatePicturesByBatch 使用GORM风格批量更新图片的名称、标签和分类
//
// 参数:
//
//	tx *gorm.DB - 数据库事务（如果为nil则使用默认连接）
//	pics []entity.Picture - 要更新的图片列表
//	tags string - 序列化后的标签JSON字符串
//	category string - 新的分类ID
//
// 返回值:
//
//	error - 操作错误信息
func (r *PictureRepository) UpdatePicturesByBatch(tx *gorm.DB, pics []entity.Picture, tags string, category string) error {
	// 1. 初始化数据库连接
	if tx == nil {
		tx = r.db
	}

	// 2. 准备批量更新数据
	var updates []map[string]interface{}

	for _, pic := range pics {
		// 为每张图片构建更新字段映射
		updateFields := map[string]interface{}{
			"name":     pic.Name, // 更新后的名称
			"tags":     tags,     // 统一设置的标签
			"category": category, // 统一设置的分类
		}
		updates = append(updates, updateFields)
	}

	// 3. 执行批量更新操作
	// 注意：GORM的批量更新要求所有记录使用相同的更新字段
	// 这里我们使用事务确保原子性
	return tx.Transaction(func(tx *gorm.DB) error {
		// 获取图片ID列表
		var ids []uint64
		for _, pic := range pics {
			ids = append(ids, pic.ID)
		}

		// 使用WHERE IN条件限定更新范围
		// 执行批量更新
		return tx.Model(&entity.Picture{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"tags":     tags,
				"category": category,
			}).Error
	})
}
