package repository

import (
	"backend/internal/model/entity"
	"backend/pkg/mysql"
	"errors"
	"fmt"
	"gorm.io/gorm"
)

type ITaskRepository struct {
	db *gorm.DB
}

func NewITaskRepository() *ITaskRepository {
	return &ITaskRepository{db: mysql.LoadDB()}
}

// 开启事务
func (r *ITaskRepository) BeginTransaction() *gorm.DB {
	return r.db.Begin()
}

// FindById 通过ID查询任务
func (r *ITaskRepository) FindById(tx *gorm.DB, id uint64) (*entity.ITask, error) {
	if tx == nil {
		tx = r.db
	}

	var task entity.ITask
	result := tx.Where("id = ?", id).First(&task)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &task, nil
}

func (r *ITaskRepository) FindByUserId(tx *gorm.DB, userId uint64) ([]entity.ITask, error) {
	if tx == nil {
		tx = r.db
	}

	var task []entity.ITask
	result := tx.Where("user_id = ?", userId).Find(&task)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return task, nil
}

// UpdateByMap 使用map更新任务字段
func (r *ITaskRepository) UpdateByMap(tx *gorm.DB, id uint64, updateMap map[string]interface{}) error {
	if tx == nil {
		tx = r.db
	}

	result := tx.Model(&entity.ITask{}).Where("id = ?", id).Updates(updateMap)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("no records updated")
	}
	return nil
}

// FindByStatus 根据状态查询任务列表
func (r *ITaskRepository) FindByStatus(tx *gorm.DB, status string) ([]entity.ITask, error) {
	if tx == nil {
		tx = r.db
	}

	var tasks []entity.ITask
	result := tx.Where("status = ?", status).Find(&tasks)
	if result.Error != nil {
		return nil, result.Error
	}
	return tasks, nil
}

// Create 创建新任务
func (r *ITaskRepository) Create(tx *gorm.DB, task *entity.ITask) error {
	if tx == nil {
		tx = r.db
	}
	fmt.Println(task)
	result := tx.Create(task)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// Delete 删除任务
func (r *ITaskRepository) Delete(tx *gorm.DB, id uint64) error {
	if tx == nil {
		tx = r.db
	}

	result := tx.Delete(&entity.ITask{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("no records deleted")
	}
	return nil
}
