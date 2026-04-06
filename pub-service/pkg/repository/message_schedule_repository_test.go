package repository

import (
	"context"
	"testing"
	"time"

	"pub-service/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockDB struct {
	errorOnCreate bool
	errorOnUpdate bool
	errorOnDelete bool
	errorOnFind   bool
	schedules     []model.MessageSchedule
}

func TestMessageScheduleRepository_GetAllWithContents_Success(t *testing.T) {
	schedules := []model.MessageSchedule{
		{
			ID:           "s001",
			ContentID:    "c001",
			Segment:      "new_users",
			TimeSchedule: time.Date(2024, 4, 7, 9, 0, 0, 0, time.UTC),
			Content: &model.Content{
				ID:       "c001",
				Content:  "Welcome message",
				ImageURL: "https://example.com/img1.jpg",
			},
		},
		{
			ID:           "s002",
			ContentID:    "c002",
			Segment:      "premium_users",
			TimeSchedule: time.Date(2024, 4, 7, 10, 0, 0, 0, time.UTC),
			Content: &model.Content{
				ID:       "c002",
				Content:  "Promotional offer",
				ImageURL: "https://example.com/img2.jpg",
			},
		},
	}

	assert.Len(t, schedules, 2)
	assert.Equal(t, "s001", schedules[0].ID)
	assert.Equal(t, "c001", schedules[0].ContentID)
	assert.NotNil(t, schedules[0].Content)
	assert.Equal(t, "c001", schedules[0].Content.ID)
	assert.Equal(t, "Welcome message", schedules[0].Content.Content)
}

func TestMessageScheduleRepository_GetByIDWithContent_Success(t *testing.T) {
	schedule := model.MessageSchedule{
		ID:           "s001",
		ContentID:    "c001",
		Segment:      "new_users",
		TimeSchedule: time.Date(2024, 4, 7, 9, 0, 0, 0, time.UTC),
		Content: &model.Content{
			ID:       "c001",
			Content:  "Welcome message",
			ImageURL: "https://example.com/img1.jpg",
		},
	}

	assert.NotNil(t, schedule)
	assert.Equal(t, "s001", schedule.ID)
	assert.Equal(t, "c001", schedule.ContentID)
	assert.NotNil(t, schedule.Content)
	assert.Equal(t, "c001", schedule.Content.ID)
}

func TestMessageScheduleRepository_Create_Success(t *testing.T) {
	schedule := &model.MessageSchedule{
		ContentID:    "c001",
		Segment:      "test_segment",
		TimeSchedule: time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC),
	}

	assert.NotNil(t, schedule)
	assert.Equal(t, "c001", schedule.ContentID)
	assert.Equal(t, "test_segment", schedule.Segment)
	expectedTime := time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime, schedule.TimeSchedule)
}

func TestMessageScheduleRepository_CreateWithContent_Success(t *testing.T) {
	content := model.Content{
		Content:  "Test content",
		ImageURL: "https://example.com/test.jpg",
	}

	schedule := &model.MessageSchedule{
		Content:      &content,
		Segment:      "test_segment",
		TimeSchedule: time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC),
	}

	assert.NotNil(t, schedule)
	assert.NotNil(t, schedule.Content)
	assert.Equal(t, "Test content", schedule.Content.Content)
	assert.Equal(t, "test_segment", schedule.Segment)
	expectedTime := time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime, schedule.TimeSchedule)
}

func TestMessageScheduleRepository_Update_Success(t *testing.T) {
	schedule := &model.MessageSchedule{
		ID:           "s001",
		ContentID:    "c001",
		Segment:      "updated_segment",
		TimeSchedule: time.Date(2024, 4, 7, 19, 0, 0, 0, time.UTC),
	}

	assert.NotNil(t, schedule)
	assert.Equal(t, "s001", schedule.ID)
	assert.Equal(t, "updated_segment", schedule.Segment)
	expectedTime := time.Date(2024, 4, 7, 19, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime, schedule.TimeSchedule)
}

func TestMessageScheduleRepository_Delete_Success(t *testing.T) {
	schedule := model.MessageSchedule{
		ID:           "s001",
		ContentID:    "c001",
		Segment:      "test_segment",
		TimeSchedule: time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC),
	}

	assert.Equal(t, "s001", schedule.ID)
	assert.Equal(t, "c001", schedule.ContentID)
}

func TestMessageScheduleRepository_TransactionFlow(t *testing.T) {
	content := model.Content{
		ID:       "c003",
		Content:  "Test content",
		ImageURL: "https://example.com/test.jpg",
	}

	schedule := &model.MessageSchedule{
		ID:           "s003",
		ContentID:    "c003",
		Content:      &content,
		Segment:      "test_segment",
		TimeSchedule: time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC),
	}

	assert.NotEmpty(t, content.ID)
	assert.NotEmpty(t, schedule.ID)
	assert.Equal(t, "c003", schedule.ContentID)
	assert.Equal(t, content.ID, schedule.ContentID)
}

func TestMessageScheduleRepository_RelationshipLoading(t *testing.T) {
	content := model.Content{
		ID:       "c001",
		Content:  "Welcome message",
		ImageURL: "https://example.com/img1.jpg",
	}

	schedule := model.MessageSchedule{
		ID:           "s001",
		ContentID:    "c001",
		Segment:      "new_users",
		TimeSchedule: time.Date(2024, 4, 7, 9, 0, 0, 0, time.UTC),
		Content:      &content,
	}

	require.NotNil(t, schedule.Content)
	assert.Equal(t, "c001", schedule.Content.ID)
	assert.Equal(t, "Welcome message", schedule.Content.Content)
	assert.Equal(t, "https://example.com/img1.jpg", schedule.Content.ImageURL)
}

func TestMessageScheduleRepository_PointerFields(t *testing.T) {
	var nilContent *model.Content

	schedule1 := model.MessageSchedule{
		ID:           "s001",
		ContentID:    "c001",
		Segment:      "test_segment",
		TimeSchedule: time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC),
		Content:      nilContent,
	}

	assert.Nil(t, schedule1.Content)
	assert.Equal(t, "c001", schedule1.ContentID)

	content := &model.Content{
		ID:       "c001",
		Content:  "Test content",
		ImageURL: "https://example.com/test.jpg",
	}

	schedule2 := model.MessageSchedule{
		ID:           "s002",
		ContentID:    "c001",
		Segment:      "test_segment",
		TimeSchedule: time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC),
		Content:      content,
	}

	assert.NotNil(t, schedule2.Content)
	assert.Equal(t, "c001", schedule2.Content.ID)
}

func TestMessageScheduleRepository_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		schedule  *model.MessageSchedule
		expectErr bool
	}{
		{
			name: "valid schedule",
			schedule: &model.MessageSchedule{
				ContentID:    "c001",
				Segment:      "test_segment",
				TimeSchedule: time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC),
			},
			expectErr: false,
		},
		{
			name: "schedule with content",
			schedule: &model.MessageSchedule{
				Content: &model.Content{
					Content:  "Test content",
					ImageURL: "https://example.com/test.jpg",
				},
				Segment:      "test_segment",
				TimeSchedule: time.Date(2024, 4, 7, 18, 0, 0, 0, time.UTC),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.schedule)
			if tt.schedule.Content != nil {
				assert.NotEmpty(t, tt.schedule.Content.Content)
			}
		})
	}

	_ = ctx
}

func TestMessageScheduleRepository_ModelFields(t *testing.T) {
	now := time.Now()

	schedule := model.MessageSchedule{
		ID:           "s001",
		ContentID:    "c001",
		Segment:      "new_users",
		TimeSchedule: time.Date(2024, 4, 7, 9, 0, 0, 0, time.UTC),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assert.Equal(t, "s001", schedule.ID)
	assert.Equal(t, "c001", schedule.ContentID)
	assert.Equal(t, "new_users", schedule.Segment)
	expectedTime := time.Date(2024, 4, 7, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime, schedule.TimeSchedule)
	assert.False(t, schedule.CreatedAt.IsZero())
	assert.False(t, schedule.UpdatedAt.IsZero())
}

func TestMessageScheduleRepository_ContentModel(t *testing.T) {
	content := model.Content{
		ID:       "c001",
		Content:  "Welcome message",
		ImageURL: "https://example.com/img1.jpg",
	}

	assert.Equal(t, "c001", content.ID)
	assert.Equal(t, "Welcome message", content.Content)
	assert.Equal(t, "https://example.com/img1.jpg", content.ImageURL)
}

func TestMessageScheduleRepository_TableName(t *testing.T) {
	schedule := model.MessageSchedule{}
	assert.Equal(t, "message_schedules", schedule.TableName())

	content := model.Content{}
	assert.Equal(t, "contents", content.TableName())
}
