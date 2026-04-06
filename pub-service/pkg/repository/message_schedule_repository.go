package repository

import (
	"context"
	"fmt"

	"pub-service/pkg/db"
	"pub-service/pkg/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MessageScheduleRepository interface {
	GetAllWithContents(ctx context.Context) ([]model.MessageSchedule, error)
	GetByIDWithContent(ctx context.Context, id string) (*model.MessageSchedule, error)
	Create(ctx context.Context, schedule *model.MessageSchedule) error
	CreateWithContent(ctx context.Context, schedule *model.MessageSchedule) error
	Update(ctx context.Context, schedule *model.MessageSchedule) error
	Delete(ctx context.Context, id string) error
}

type MessageScheduleRepositoryImpl struct {
	db *gorm.DB
}

func NewMessageScheduleRepository(mysqlDB *db.MySQLDB) MessageScheduleRepository {
	return &MessageScheduleRepositoryImpl{
		db: mysqlDB.GetDB(),
	}
}

func (r *MessageScheduleRepositoryImpl) GetAllWithContents(ctx context.Context) ([]model.MessageSchedule, error) {
	var schedules []model.MessageSchedule

	err := r.db.WithContext(ctx).Preload("Content").Find(&schedules).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get message schedules with contents: %w", err)
	}

	return schedules, nil
}

func (r *MessageScheduleRepositoryImpl) GetByIDWithContent(ctx context.Context, id string) (*model.MessageSchedule, error) {
	var schedule model.MessageSchedule

	err := r.db.WithContext(ctx).Preload("Content").Where("id = ?", id).First(&schedule).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("message schedule not found")
		}
		return nil, fmt.Errorf("failed to get message schedule by id: %w", err)
	}

	return &schedule, nil
}

func (r *MessageScheduleRepositoryImpl) Create(ctx context.Context, schedule *model.MessageSchedule) error {
	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	if err := r.db.WithContext(ctx).Create(schedule).Error; err != nil {
		return fmt.Errorf("failed to create message schedule: %w", err)
	}

	return nil
}

func (r *MessageScheduleRepositoryImpl) CreateWithContent(ctx context.Context, schedule *model.MessageSchedule) error {
	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	if schedule.Content != nil && schedule.Content.ID == "" {
		schedule.Content.ID = uuid.New().String()
	}

	if schedule.Content != nil {
		if err := tx.Create(schedule.Content).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create content: %w", err)
		}
		schedule.ContentID = schedule.Content.ID
	}

	if err := tx.Create(schedule).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create message schedule: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *MessageScheduleRepositoryImpl) Update(ctx context.Context, schedule *model.MessageSchedule) error {
	if err := r.db.WithContext(ctx).Save(schedule).Error; err != nil {
		return fmt.Errorf("failed to update message schedule: %w", err)
	}

	return nil
}

func (r *MessageScheduleRepositoryImpl) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MessageSchedule{}).Error; err != nil {
		return fmt.Errorf("failed to delete message schedule: %w", err)
	}

	return nil
}
