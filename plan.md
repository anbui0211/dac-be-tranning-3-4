# Kế Hoạch Chi Tiết: Pub-Sub System với Large Data Processing

## Tổng quan case study

Xây dựng hệ thống pub-sub xử lý dữ liệu lớn:
- **Pub-service**: Batch xử lý file CSV từ S3 → gửi vào SQS
- **Sub-service**: Consume từ SQS → dedupe → xử lý → mark done
- **Scale**: 100K users, tối ưu RAM và throughput

---

## Kiến trúc hệ thống

```
┌─────────────────────────────────────────────────────────────┐
│                    Docker Compose                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │ DynamoDB │  │  MinIO   │  │ElasticMQ │  │  Redis   │    │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘    │
└─────────────────────────────────────────────────────────────┘
                            ↓
        ┌───────────────────┴───────────────────┐
        │                                       │
┌───────▼────────┐                   ┌─────────▼────────┐
│  pub-service   │                   │  sub-service     │
│                │                   │                  │
│  /batch API    │                   │  Workers (N)     │
│       ↓        │                   │       ↓          │
│  CSV Stream    │                   │  Dedupe (Redis)  │
│       ↓        │       SQS         │       ↓          │
│  Worker Pool   ├──────────────────►│  Process (log)   │
│       ↓        │                   │       ↓          │
│  Batch Send    │                   │  Mark Done       │
└────────────────┘                   └──────────────────┘
```

---

## Cấu trúc Project

```
be-training-3-4/
├── docker-compose.yml
├── plan.md
├── README.md
├── pub-service/
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── Makefile
│   ├── pkg/
│   │   ├── db/
│   │   │   └── mysql.go
│   │   ├── handlers/
│   │   │   └── batch.go
│   │   └── model/
│   │       └── model.go
│   ├── providers/
│   │   ├── s3.go
│   │   └── sqs.go
│   └── database/
│       └── migrations/
│           ├── 000001_init.up.sql
│           └── 000001_init.down.sql
├── sub-service/
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   └── providers/
│       ├── sqs.go
│       └── redis.go
└── scripts/
    └── generate-csv.py
```

---

## Vấn đề và Giải pháp

### 1. Lưu file lớn vào S3 tối ưu hơn

**Giải pháp:**
- ✅ Nén file CSV thành `.csv.gz` (giảm ~80% size)
- ✅ Partition theo ngày nếu cần: `users_2024-04-02.csv.gz`
- ✅ Nếu file > 100MB: dùng multipart upload

**Lợi ích:**
- Giảm bandwidth cost
- Tăng tốc độ download
- Giảm storage cost

---

### 2. Load CSV có tốn RAM không?

**Vấn đề:** Nếu load toàn bộ CSV → OOM với file lớn.

**Giải pháp:** Row-by-row streaming

```go
reader := csv.NewReader(gzip.NewReader(file))
for {
    row, err := reader.Read()
    // Process row ngay lập tức - O(1) RAM
}
```

**Lợi ích:**
- RAM usage: O(1) - chỉ buffer 1 row
- Có thể xử lý file bất kỳ kích thước
- Streaming pipeline

---

### 3. Batch dữ liệu lớn từ CSV vào queue

**Vấn đề:** CSV có 100K rows → làm sao đưa vào queue nhanh và không tốn RAM?

**Giải pháp tốt nhất: Worker Pool + Batch API**

```
CSV Reader (streaming) → Channel (buffer 1000) → Workers (20) → SQS Batch Send
```

**Why?**
- Streaming CSV: O(1) RAM
- Channel: Decouple reader và workers
- Worker pool: Parallel send (10x-50x faster)
- Batch API: Giảm API calls (100K → 10K calls)

---

### 4. Lấy dữ liệu từ queue xử lý (large scale)

**Giải pháp tốt nhất: Worker Pool + Long Polling**

```
SQS (long poll, wait 20s, max 10 msg) → Channel (buffer 100) → Workers (50)
```

**Key optimizations:**
- Long polling: Giảm empty responses, cost ↓
- Worker pool: Parallel processing
- Batch receive: Lấy 10 messages/call
- Graceful shutdown: Xử lý hết messages đang trong workers

---

### 5. Handle job duplicate/đã xử lý

**Dedupe Logic trong Production:**

Combine cả 2 cách:

1. **Message-based dedupe (Technical):**
   - Purpose: Prevent duplicate từ hệ thống (retry, network failure, duplicate send)
   - Key: `message_id` (UUID v4)
   - TTL: 24h

2. **User-based dedupe (Business):**
   - Purpose: Prevent duplicate business logic (không spam user)
   - Key: `user_id` + `context` (VD: `daily_campaign_2024-04-02`)
   - TTL: 7 ngày

**Pattern:**
```go
// Step 1: Technical dedupe (fast fail)
if exists in redis:processed:{message_id} {
    return nil // Already processed
}

// Step 2: Business dedupe (domain logic)
if exists in redis:user:{user_id}:campaign:{date} {
    log.Printf("User %s already processed today", user_id)
    redis.SET(processed:{message_id}, 1, 24h) // Still mark
    return
}

// Process...
redis.SET(processed:{message_id}, 1, 24h)
redis.SET(user:{user_id}:campaign:{date}, 1, 7d)
```

---

## Chi tiết từng thành phần

### Docker Compose

**Services:**
- `dynamodb`: amazon/dynamodb-local:latest
- `minio`: minio/minio:latest (S3)
- `minio-console`: minio/console:latest (UI)
- `elasticmq`: softwaremill/elasticmq:latest (SQS)
- `redis`: redis:7-alpine
- `pub-service`: Tự build
- `sub-service`: Tự build

**Key configurations:**
- MinIO: `MINIO_ROOT_USER=admin`, `MINIO_ROOT_PASSWORD=password`
- ElasticMQ: Create queue `user-jobs-queue` on startup
- All services in same network
- Port mapping cho local access

---

### Script Tạo Test Data (`scripts/generate-csv.py`)

**Features:**
- Tạo 100K rows (configurable)
- 5 columns: `user_id, email, name, phone, message`
- Gzip compress output → `users_2024-04-02.csv.gz`
- Upload lên MinIO (optional)

**Format:**
```csv
user_id,email,name,phone,message
1001,user1001@example.com,Nguyen Van A,0901234567,Chào mừng tham gia chương trình
1002,user1002@example.com,Tran Thi B,0912345678,Đặc quyền mới cho bạn
...
```

---

### Pub-Service

**Architecture:**
```
main.go
  ↓
Cron scheduler (ticker)
  ↓
/batch handler
  ↓
S3 Client → Download CSV.gz (streaming)
  ↓
CSV Streaming Reader (gzip + csv)
  ↓
Channel (buffer 1000)
  ↓
Worker Pool (20 workers)
  ↓
SQS Batch Send (10 msg/batch)
```

**Components:**

**main.go:**
- Gin server với `/batch` endpoint
- Cron job: Gọi `/batch` mỗi X phút (configurable)
- Graceful shutdown

**handlers/batch.go:**
- Handler xử lý batch logic
- Orchestrator cho S3 → CSV → Workers → SQS

**providers/s3.go:**
- MinIO client với custom endpoint
- `DownloadFile(bucket, key)` → `io.ReadCloser`
- Handle streaming download

**providers/sqs.go:**
- ElasticMQ client
- `BatchSend(messages)` → SQS SendMessageBatch API
- Max 10 messages/batch

**Message Format:**
```go
type Message struct {
    MessageID string    `json:"message_id"`
    UserID    string    `json:"user_id"`
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
    Email     string    `json:"email"`
    Name      string    `json:"name"`
    Phone     string    `json:"phone"`
}
```

**Worker Pool Logic:**
```go
// Worker pool với 20 goroutines
for i := 0; i < 20; i++ {
    go worker(rowChannel)
}

// Channel buffer 1000
rowChannel := make(chan []string, 1000)

// CSV reader
reader := csv.NewReader(gzipReader)
for {
    row, err := reader.Read()
    rowChannel <- row
}
```

---

### Sub-Service

**Architecture:**
```
main.go
  ↓
Worker Pool (50 workers)
  ↓
SQS Long Polling (wait 20s, max 10 msg)
  ↓
Channel (buffer 100)
  ↓
Process Worker
  ↓
Redis Dedupe (message_id + user_id)
  ↓
Process (log output)
  ↓
Mark Done (SET in Redis + Delete from SQS)
```

**Components:**

**main.go:**
- Gin server (health check)
- Graceful shutdown
- Metrics: processed count, duplicate count, error count

**providers/sqs.go:**
- ElasticMQ client
- `ReceiveMessage()` với Long Polling
- `DeleteMessage()` sau khi process thành công

**providers/redis.go:**
- Redis client (go-redis v9)
- `CheckDuplicate(messageID, userID)` → bool
- `MarkProcessed(messageID, userID)` → error
- Helper functions cho dedupe keys

**Dedupe Logic:**
```go
// Key patterns
processedKey := fmt.Sprintf("processed:%s", message.MessageID)
userCampaignKey := fmt.Sprintf("user:%s:campaign:%s", message.UserID, today)

// Check technical dedupe
exists, _ := redis.Exists(ctx, processedKey)
if exists {
    return // Already processed
}

// Check business dedupe
exists, _ = redis.Exists(ctx, userCampaignKey)
if exists {
    log.Printf("User %s already processed today", message.UserID)
    redis.SET(processedKey, 1, 24h) // Still mark to prevent retry
    return
}

// Process...
log.Printf("Processing user %s: %s", message.UserID, message.Message)

// Mark done
redis.SET(processedKey, 1, 24h)
redis.SET(userCampaignKey, 1, 7d)
```

**Worker Pool Logic:**
```go
// 50 workers parallel
for i := 0; i < 50; i++ {
    go worker(msgChannel)
}

// SQS receiver
for {
    messages := sqs.ReceiveMessage(10, 20s)
    for _, msg := range messages {
        msgChannel <- msg
    }
}
```

---

## Environment Variables

**Common:**
```env
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=dummy
AWS_SECRET_ACCESS_KEY=dummy
```

**Pub-service:**
```env
PORT=8081
S3_ENDPOINT=http://minio:9000
S3_BUCKET=user-data
S3_ACCESS_KEY=admin
S3_SECRET_KEY=password
SQS_ENDPOINT=http://elasticmq:9324
SQS_QUEUE_URL=http://elasticmq:9324/queue/user-jobs-queue
SQS_WORKER_COUNT=20
BATCH_SIZE=10000
BATCH_INTERVAL=5m
```

**Sub-service:**
```env
PORT=8082
SQS_ENDPOINT=http://elasticmq:9324
SQS_QUEUE_URL=http://elasticmq:9324/queue/user-jobs-queue
REDIS_ADDR=redis:6379
SQS_WORKER_COUNT=50
```

---

## Flow Hoàn chỉnh

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Setup                                                     │
│    - Start docker-compose                                   │
│    - Generate CSV (100K rows)                               │
│    - Upload to MinIO                                        │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Pub-Service (Cron triggers every 5m)                     │
│    - /batch endpoint called                                 │
│    - Download CSV.gz from S3 (streaming)                    │
│    - Parse CSV row-by-row (O(1) RAM)                        │
│    - 20 workers parallel send to SQS                         │
│    - Batch 10 messages per API call                         │
│    - ~10K SQS API calls for 100K messages                   │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Sub-Service (Always running)                             │
│    - 50 workers listen to SQS                               │
│    - Long polling (wait 20s)                                │
│    - Dedupe check in Redis                                  │
│        a. message_id (technical)                            │
│        b. user_id + date (business)                        │
│    - Process: log to stdout                                 │
│    - Mark done in Redis (TTL: 24h message, 7d user)         │
│    - Delete from SQS                                        │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. Monitoring                                                │
│    - Redis: Monitor processed/duplicate counts             │
│    - SQS: Monitor queue depth                               │
│    - Logs: Track processing flow                            │
└─────────────────────────────────────────────────────────────┘
```

---

## Optimization Techniques

### Pub-Service Optimization:
- ✅ Streaming CSV read (O(1) RAM)
- ✅ Gzip decompress streaming
- ✅ Worker pool (20 workers)
- ✅ Batch SQS API (10 msg/call)
- ✅ Channel buffer (1000) → smooth producer-consumer

### Sub-Service Optimization:
- ✅ Long polling SQS (reduce cost, wait 20s)
- ✅ Worker pool (50 workers)
- ✅ Batch receive (10 msg/call)
- ✅ Redis pipelining (optional for high throughput)
- ✅ TTL for auto cleanup

### Redis Optimization:
- ✅ SET with EX (TTL) → auto cleanup
- ✅ Key pattern: `namespace:entity:attribute`
- ✅ Consider Redis Cluster nếu scale lớn hơn

---

## Test Scenarios

### Scenario 1: Normal flow
- 100K messages → SQS → Sub-service → Processed
- Verify: 100K unique in Redis, 0 duplicates

### Scenario 2: Duplicate messages
- Re-run pub-service → Same messages again
- Verify: All skipped (Redis dedupe)

### Scenario 3: Redis TTL
- Wait 24h → Re-run
- Verify: Technical dedupe expired, but business dedupe still works

### Scenario 4: Failure handling
- Kill sub-service mid-processing
- Verify: SQS retains unprocessed messages
- Restart → Process resumes

---

## Key Libraries

**Go:**
- `github.com/gin-gonic/gin` - Web framework
- `github.com/aws/aws-sdk-go-v2/*` - AWS SDK v2
- `github.com/redis/go-redis/v9` - Redis client
- `compress/gzip`, `encoding/csv` - CSV streaming
- `github.com/google/uuid` - UUID generation
- `github.com/robfig/cron` - Cron scheduler

**Python (script):**
- Standard lib only (csv, gzip)

---

## Performance Expectations

**Pub-Service:**
- 100K messages → ~10K SQS API calls (batch of 10)
- 20 workers → ~5-10K msg/sec
- RAM: ~100MB (channel buffers)

**Sub-Service:**
- 50 workers → ~10-20K msg/sec
- Long polling → giảm cost
- Redis dedupe: <1ms per check

**Total:**
- Process 100K messages trong ~10-20 seconds
- Scale bằng cách tăng workers

---

## Next Steps

1. ✅ Tạo cấu trúc project
2. ✅ Implement docker-compose
3. ✅ Implement script tạo test data
4. ✅ Implement pub-service
5. ✅ Implement sub-service
6. ✅ Test và verify
7. ✅ Tài liệu README

