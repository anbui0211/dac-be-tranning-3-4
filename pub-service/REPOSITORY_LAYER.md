# Repository Layer Implementation

## Overview
Created repository layer with interface for message_schedules management.

## Files Created/Modified

### 1. `pkg/model/model.go` (Modified)
Added `Content` relationship to `MessageSchedule` model:
```go
type MessageSchedule struct {
    ID           string    `gorm:"column:id;primaryKey;type:varchar(36)"`
    ContentID    string    `gorm:"column:content_id;type:varchar(36)"`
    Content      *Content  `gorm:"foreignKey:ContentID"`
    Segment      string    `gorm:"column:segment;type:varchar(100)"`
    TimeSchedule string    `gorm:"column:time_schedule;type:varchar(50)"`
    CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
    UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
```

### 2. `pkg/repository/message_schedule_repository.go` (New)
Repository interface and implementation:

**Interface:**
```go
type MessageScheduleRepository interface {
    GetAllWithContents(ctx context.Context) ([]model.MessageSchedule, error)
    GetByIDWithContent(ctx context.Context, id string) (*model.MessageSchedule, error)
}
```

**Implementation:**
- `GetAllWithContents()` - Fetches all message schedules with preloaded content data
- `GetByIDWithContent()` - Fetches a single message schedule by ID with content data

## Usage Example

### Initialize Repository in main.go:
```go
import (
    "pub-service/pkg/db"
    "pub-service/pkg/repository"
)

// In main()
mysqlDB, err := db.NewMySQLDB(ctx)
if err != nil {
    log.Fatalf("Failed to initialize database: %v", err)
}

messageScheduleRepo := repository.NewMessageScheduleRepository(mysqlDB)
```

### Use Repository:
```go
// Get all message schedules with contents
schedules, err := messageScheduleRepo.GetAllWithContents(ctx)
if err != nil {
    log.Printf("Error: %v", err)
}

// Get specific schedule by ID
schedule, err := messageScheduleRepo.GetByIDWithContent(ctx, "s001")
if err != nil {
    log.Printf("Error: %v", err)
}
```

## Features
- Interface-based design for easy testing and mockability
- GORM Preload for efficient JOIN operations
- Proper error handling
- Context support for request cancellation and timeouts
- Returns structured data with content information included

## Database Schema Reference
- `contents` table: id, content, image_url, created_at, updated_at
- `message_schedules` table: id, content_id (FK), segment, time_schedule, created_at, updated_at
