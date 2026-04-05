package model

import "time"

type Content struct {
	ID        string    `gorm:"column:id;primaryKey;type:varchar(36)"`
	Content   string    `gorm:"column:content;type:text;not null"`
	ImageURL  string    `gorm:"column:image_url;type:varchar(255)"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

type MessageSchedule struct {
	ID           string    `gorm:"column:id;primaryKey;type:varchar(36)"`
	ContentID    string    `gorm:"column:content_id;type:varchar(36)"`
	Segment      string    `gorm:"column:segment;type:varchar(100)"`
	TimeSchedule string    `gorm:"column:time_schedule;type:varchar(50)"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Content) TableName() string {
	return "contents"
}

func (MessageSchedule) TableName() string {
	return "message_schedules"
}
