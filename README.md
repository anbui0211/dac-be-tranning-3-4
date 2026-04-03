# Pub-Sub Large Data Processing System

Hệ thống xử lý dữ liệu lớn với pattern pub-sub, sử dụng AWS SDK v2, Redis cho dedupe, và tối ưu hóa throughput.

## Kiến trúc

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
│  /batch API    │                   │  Workers (50)    │
│       ↓        │                   │       ↓          │
│  CSV Stream    │                   │  Dedupe (Redis)  │
│       ↓        │       SQS         │       ↓          │
│  Worker Pool   ├──────────────────►│  Process (log)   │
│  (20 workers)  │                   │       ↓          │
│       ↓        │                   │  Mark Done       │
│  Batch Send    │                   └──────────────────┘
└────────────────┘
```

## Các thành phần

### Docker Services
- **DynamoDB**: amazon/dynamodb-local:latest (Port: 8000)
- **MinIO**: S3-compatible storage (Port: 9000, Console: 9001)
- **ElasticMQ**: SQS-compatible queue (Port: 9324)
- **Redis**: Dedupe cache (Port: 6379)
- **Pub-Service**: Batch processor (Port: 8081)
- **Sub-Service**: Consumer (Port: 8082)

### Pub-Service
- Endpoint: `/batch`, `/files`, `/health`
- Features:
  - Download CSV từ S3 (streaming)
  - Parse CSV row-by-row (O(1) RAM)
  - 20 workers parallel send to SQS
  - Batch 10 messages per API call
  - Cron job tự động chạy mỗi 5 phút

### Sub-Service
- Endpoint: `/health`
- Features:
  - 50 workers consume từ SQS
  - Long polling (wait 20s)
  - Redis dedupe:
    - Technical: `processed:{message_id}` (24h)
    - Business: `user:{user_id}:campaign:{date}` (7d)
  - Graceful shutdown

## Thiết kế và tối ưu hóa

### 1. Lưu file CSV vào S3
- **Giải pháp**: Nén thành `.csv.gz`
- **Lợi ích**: Giảm ~80% size, tăng tốc độ download

### 2. Load CSV (RAM optimization)
- **Giải pháp**: Row-by-row streaming
- **Code**:
  ```go
  reader := csv.NewReader(gzip.NewReader(file))
  for {
      row, err := reader.Read()
      // Process row ngay - O(1) RAM
  }
  ```

### 3. Batch dữ liệu vào SQS
- **Giải pháp**: Worker Pool + Batch API
- **Architecture**:
  ```
  CSV Reader → Channel (1000) → Workers (20) → SQS Batch (10 msg)
  ```
- **Lợi ích**:
  - Parallel send: 10x-50x faster
  - Giảm API calls: 100K → 10K calls
  - O(1) RAM usage

### 4. Consume từ SQS
- **Giải pháp**: Worker Pool + Long Polling
- **Architecture**:
  ```
  SQS (long poll, 20s) → Channel (100) → Workers (50)
  ```
- **Lợi ích**:
  - Parallel processing
  - Giảm cost với long polling
  - Graceful shutdown

### 5. Dedupe Logic

**Technical dedupe** (phòng lỗi hệ thống):
- Key: `processed:{message_id}`
- TTL: 24h
- Purpose: Prevent duplicate từ retry, network failure

**Business dedupe** (phòng spam user):
- Key: `user:{user_id}:campaign:{date}`
- TTL: 7 ngày
- Purpose: Không spam user cùng ngày

**Flow**:
```go
1. Check processed:{message_id} → nếu tồn tại, skip
2. Check user:{user_id}:campaign:{date} → nếu tồn tại, log duplicate
3. Process message
4. SET processed:{message_id} EX 86400
5. SET user:{user_id}:campaign:{date} EX 604800
```

## Cài đặt

### Prerequisites
- Docker & Docker Compose
- Go 1.22+ (cho development local)

### Quick Start

1. **Clone repository**
   ```bash
   cd be-training-3-4
   ```

2. **Generate test data**
   ```bash
   python3 scripts/generate-csv.py
   ```
   - Output: `users_2024-04-02.csv.gz` (100K rows)

3. **Upload CSV lên MinIO**
   ```bash
   docker-compose up -d minio redis elasticmq
   # Upload file manually via MinIO Console: http://localhost:9001
   # Username: admin, Password: password
   # Create bucket: user-data
   # Upload users_2024-04-02.csv.gz
   ```

4. **Start all services**
   ```bash
   docker-compose up -d
   ```

5. **Monitor logs**
   ```bash
   # Pub-service logs
   docker-compose logs -f pub-service

   # Sub-service logs
   docker-compose logs -f sub-service

   # All logs
   docker-compose logs -f
   ```

6. **Health check**
   ```bash
   # Pub-service
   curl http://localhost:8081/health

   # Sub-service (với metrics)
   curl http://localhost:8082/health
   ```

## API Endpoints

### Pub-Service (Port: 8081)

**POST /batch**
- Trigger batch processing
- Body:
  ```json
  {
    "csv_file": "users_2024-04-02.csv.gz"
  }
  ```
- Response:
  ```json
  {
    "status": "success",
    "message": "Batch processing started"
  }
  ```

**GET /files**
- List files in S3 bucket
- Response:
  ```json
  {
    "files": ["users_2024-04-02.csv.gz"]
  }
  ```

**GET /health**
- Health check
- Response:
  ```json
  {
    "status": "ok",
    "service": "pub-service"
  }
  ```

### Sub-Service (Port: 8082)

**GET /health**
- Health check với metrics
- Response:
  ```json
  {
    "status": "ok",
    "service": "sub-service",
    "processed": 50000,
    "duplicates": 0,
    "errors": 0,
    "goroutines": 55
  }
  ```

## Environment Variables

### Pub-Service
```env
PORT=8081
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=dummy
AWS_SECRET_ACCESS_KEY=dummy
S3_ENDPOINT=http://minio:9000
S3_BUCKET=user-data
S3_ACCESS_KEY=admin
S3_SECRET_KEY=password
SQS_ENDPOINT=http://elasticmq:9324
SQS_QUEUE_URL=http://elasticmq:9324/queue/user-jobs-queue
SQS_WORKER_COUNT=20
BATCH_INTERVAL=5m
```

### Sub-Service
```env
PORT=8082
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=dummy
AWS_SECRET_ACCESS_KEY=dummy
SQS_ENDPOINT=http://elasticmq:9324
SQS_QUEUE_URL=http://elasticmq:9324/queue/user-jobs-queue
SQS_WORKER_COUNT=50
REDIS_ADDR=redis:6379
```

## Test Scenarios

### Scenario 1: Normal flow
```bash
# Upload CSV lên MinIO
# Trigger batch processing
curl -X POST http://localhost:8081/batch \
  -H "Content-Type: application/json" \
  -d '{"csv_file": "users_2024-04-02.csv.gz"}'

# Monitor sub-service metrics
watch -n 5 curl http://localhost:8082/health
```

**Expected result**:
- 100K messages processed
- 0 duplicates
- ~10-20 seconds processing time

### Scenario 2: Duplicate messages
```bash
# Re-trigger batch processing
curl -X POST http://localhost:8081/batch \
  -H "Content-Type: application/json" \
  -d '{"csv_file": "users_2024-04-02.csv.gz"}'

# All messages should be skipped
```

**Expected result**:
- 0 new messages processed
- 100K duplicates detected

### Scenario 3: Redis TTL
```bash
# Wait 24h
# Technical dedupe expired
# Re-trigger batch

# Wait 7 days
# Business dedupe expired
# Re-trigger batch
```

### Scenario 4: Failure handling
```bash
# Kill sub-service mid-processing
docker-compose stop sub-service

# Messages should remain in SQS
# Restart sub-service
docker-compose start sub-service

# Processing resumes
```

## Performance

### Expected Throughput
- **Pub-Service**: ~5-10K messages/sec
- **Sub-Service**: ~10-20K messages/sec
- **Total**: Process 100K messages trong ~10-20 seconds

### RAM Usage
- **Pub-Service**: ~100MB (channel buffers)
- **Sub-Service**: ~50MB
- **Total**: O(1) RAM regardless of CSV size

## Troubleshooting

### Issues

**1. CSV không được tìm thấy**
- Kiểm tra file đã upload lên MinIO bucket `user-data`
- Verify file path trong `/batch` request

**2. Duplicate messages không được skip**
- Kiểm tra Redis connection: `docker exec -it redis redis-cli ping`
- Monitor Redis keys: `docker exec -it redis redis-cli keys "*"`

**3. SQS queue không nhận message**
- Kiểm tra ElasticMQ: http://localhost:9325
- Verify SQS_ENDPOINT và SQS_QUEUE_URL

**4. OOM error**
- Giảm `SQS_WORKER_COUNT` trong docker-compose.yml
- Giảm channel buffer sizes

## Architecture Decisions

### Why Worker Pool?
- **Sequential**: Chậm, 1 message/sec
- **Worker Pool**: Nhanh, 10-50x faster
- **Trade-off**: Tăng RAM cho channel buffers

### Why Batch API?
- **Single API**: 100K calls cho 100K messages
- **Batch API**: 10K calls (batch 10)
- **Trade-off**: Cần buffer messages

### Why Long Polling?
- **Short polling**: Nhiều empty responses, cost cao
- **Long polling**: Giảm empty responses, wait 20s
- **Trade-off**: Higher latency cho đầu tiên

### Why Redis dedupe?
- **DynamoDB**: Durable nhưng chậm, cost cao
- **Redis**: Nhanh, đơn giản, TTL auto cleanup
- **Trade-off**: Not durable (nhưng đủ cho demo)

## Future Improvements

1. **Horizontal scaling**:
   - Multiple sub-service instances
   - SQS load balancing

2. **Monitoring**:
   - Prometheus metrics
   - Grafana dashboards
   - Alerting

3. **Error handling**:
   - Dead-letter queue (DLQ)
   - Retry logic with exponential backoff
   - Circuit breaker

4. **Performance**:
   - Redis pipelining
   - Connection pooling
   - Batch Redis operations

5. **Features**:
   - Dynamic worker count
   - Rate limiting
   - Message prioritization

## License

MIT

## Author

Training Project - Backend Engineering
