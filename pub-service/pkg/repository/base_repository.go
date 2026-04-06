package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type BaseRepository interface {
	Create(ctx context.Context, entity interface{}) error
	Update(ctx context.Context, entity interface{}) error
	Delete(ctx context.Context, id string, model interface{}) error
	GetByID(ctx context.Context, id string, model interface{}) error
}

type BaseRepositoryImpl struct {
	db *gorm.DB
}

func NewBaseRepository(db *gorm.DB) BaseRepository {
	return &BaseRepositoryImpl{
		db: db,
	}
}

func (r *BaseRepositoryImpl) Create(ctx context.Context, entity interface{}) error {
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}
	return nil
}

func (r *BaseRepositoryImpl) Update(ctx context.Context, entity interface{}) error {
	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}
	return nil
}

func (r *BaseRepositoryImpl) Delete(ctx context.Context, id string, model interface{}) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(model).Error; err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}
	return nil
}

func (r *BaseRepositoryImpl) GetByID(ctx context.Context, id string, model interface{}) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("entity not found")
		}
		return fmt.Errorf("failed to get entity by id: %w", err)
	}
	return nil
}
