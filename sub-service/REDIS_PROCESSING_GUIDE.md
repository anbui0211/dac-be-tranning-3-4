# Redis Processing Guide - Sub-Service Distributed Lock

## 📋 Overview

Đây là guide giải thích chi tiết về cách hệ thống sử dụng Redis để **distributed lock** và **prevent duplicate processing** trong multi-worker environment.

---

## 🔑 Redis Keys Structure

### **1. Processing Lock Key**
```
Key:    processing:{messageID}
Value:   "{workerID}:{timestamp}"
TTL:     5 phút (PROCESSING_LOCK_TTL env var)
Example:  processing:msg-001 → "42:1715100000"
```

**Purpose:**
- ✅ Đánh dấu message đang được xử lý bởi worker nào
- ✅ Prevent multiple workers processing cùng message
- ✅ TTL auto-expire để prevent deadlock

**TTL logic:**
```
Worker acquires lock → Key set với TTL 5 phút
Worker crashes/timeout  → Key tự động expire sau 5 phút
Lock expires          → Worker khác có thể acquire lock
```

---

### **2. Processed Tracking Key**
```
Key:    processed:{messageID}
Value:   "1"
TTL:     24 hours (hardcoded)
Example:  processed:msg-001 → "1"
```

**Purpose:**
- ✅ Track message đã processed thành công
- ✅ Prevent reprocessing khi worker restart
- ✅ Auto-cleanup sau 24 giờ

**Usage:**
```go
// Check nếu đã processed
if redis.Exists("processed:msg-001") {
    // Skip - đã process rồi
}

// Mark đã processed
redis.Set("processed:msg-001", "1", 24*time.Hour)
```

---

### **3. User Campaign Deduplication Key**
```
Key:    user:{userID}:campaign:{date}
Value:   "1"
TTL:     7 days (hardcoded)
Example:  user:john:campaign:2025-04-07 → "1"
```

**Purpose:**
- ✅ Deduplicate messages per user per day
- ✅ Prevent spam to LINE cho cùng user cùng ngày
- ✅ Auto-cleanup sau 7 ngày

**Usage:**
```go
// Check nếu user đã nhận campaign hôm nay
if redis.Exists("user:john:campaign:2025-04-07") {
    // Skip - user đã nhận campaign hôm nay
}
```

---

## 🔄 Redis Operations Flow

### **Operation 1: Acquire Processing Lock (SETNX)**

**Redis Command:**
```redis
SETNX processing:{messageID} "{workerID}:{timestamp}" EX 300
```

**Code Implementation:**
```go
func (p *RedisProvider) AcquireProcessingLock(
    ctx context.Context,
    messageID string,
    workerID int,
    ttl time.Duration,
) (bool, error) {
    lockKey := fmt.Sprintf("processing:%s", messageID)
    lockValue := fmt.Sprintf("%d:%d", workerID, time.Now().Unix())

    // SETNX = "SET if Not Exists" - Atomic operation
    acquired, err := p.client.SetNX(ctx, lockKey, lockValue, ttl).Result()
    if err != nil {
        return false, fmt.Errorf("failed to acquire lock: %w", err)
    }

    return acquired, nil
}
```

**SETNX Behavior:**
```
Scenario A: Key không tồn tại
    Worker 1: SETNX processing:msg-001 "1:1715100000" EX 300
    → SUCCESS (return TRUE)
    → Lock acquired

Scenario B: Key đã tồn tại (Worker 1 đã acquire)
    Worker 2: SETNX processing:msg-001 "2:1715100005" EX 300
    → FAIL (return FALSE)
    → Lock not acquired (Worker 1 đang xử lý)
```

**Tại sao SETNX là atomic?**
- ✅ Redis single-threaded: Mọi operation chạy sequentially
- ✅ Không có race condition: Không thể có 2 workers acquire cùng lock
- ✅ Thread-safe: Guaranteed consistency

---

### **Operation 2: Check Processed Status (EXISTS)**

**Redis Command:**
```redis
EXISTS processed:{messageID}
```

**Code Implementation:**
```go
func (p *RedisProvider) CheckProcessed(
    ctx context.Context,
    messageID string,
) (bool, error) {
    processedKey := fmt.Sprintf("processed:%s", messageID)

    exists, err := p.client.Exists(ctx, processedKey).Result()
    if err != nil {
        return false, fmt.Errorf("failed to check processed status: %w", err)
    }

    return exists > 0, nil
}
```

**EXISTS Behavior:**
```
Redis: EXISTS processed:msg-001
→ Return: 1 (true) nếu key tồn tại
→ Return: 0 (false) nếu key không tồn tại
```

**Usage in worker:**
```go
isProcessed, _ := redis.CheckProcessed(ctx, messageID)
if isProcessed {
    // Message đã process trước đó
    sqs.DeleteMessage(ctx, msg.ReceiptHandle)
    duplicateSkipped++
    continue
}
// Nếu không processed → tiếp tục process
```

---

### **Operation 3: Mark Processed (SET)**

**Redis Command:**
```redis
SET processed:{messageID} "1" EX 86400
```

**Code Implementation:**
```go
func (p *RedisProvider) MarkProcessed(
    ctx context.Context,
    messageID string,
    userID string,
) error {
    today := time.Now().Format("2006-01-02")
    processedKey := fmt.Sprintf("processed:%s", messageID)
    userCampaignKey := fmt.Sprintf("user:%s:campaign:%s", userID, today)

    // Set cả 2 keys cùng lúc (pipeline)
    pipe := p.client.Pipeline()
    pipe.Set(ctx, processedKey, "1", 24*time.Hour)
    pipe.Set(ctx, userCampaignKey, "1", 7*24*time.Hour)

    _, err := pipe.Exec(ctx)
    if err != nil {
        return fmt.Errorf("failed to mark processed: %w", err)
    }

    return nil
}
```

**Pipeline Benefit:**
- ✅ **Atomic**: Cả 2 keys set cùng lúc
- ✅ **Performance**: 1 round-trip thay vì 2
- ✅ **Consistency**: Không có partial state

---

### **Operation 4: Release Processing Lock (DEL)**

**Redis Command:**
```redis
DEL processing:{messageID}
```

**Code Implementation:**
```go
func (p *RedisProvider) ReleaseProcessingLock(
    ctx context.Context,
    messageID string,
) error {
    lockKey := fmt.Sprintf("processing:%s", messageID)

    if err := p.client.Del(ctx, lockKey).Err(); err != nil {
        return fmt.Errorf("failed to release lock: %w", err)
    }

    return nil
}
```

**Khi nào release lock?**
```go
// SUCCESS PATH (LINE push thành công)
redis.MarkProcessed(messageID, userID)
redis.ReleaseProcessingLock(messageID)
sqs.DeleteMessage(messageID)
processedCount++

// ERROR PATH (LINE push fail)
redis.ReleaseProcessingLock(messageID)
// KHÔNG delete SQS → message sẽ retry
```

---

## 🎯 Complete Processing Workflow

### **Step-by-Step Flow:**

```
┌─────────────────────────────────────────────────────────────┐
│ STEP 1: Worker nhận message từ SQS                       │
├─────────────────────────────────────────────────────────────┤
│ Message A nhận được:                                    │
│   - messageID: "msg-001"                               │
│   - userID: "john"                                     │
│   - message: "Hello from LINE integration"                │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ STEP 2: Check nếu đã processed (EXISTS)                  │
├─────────────────────────────────────────────────────────────┤
│ Redis: EXISTS processed:msg-001                        │
│                                                        │
│ Scenario A: Key không tồn tại                           │
│   → Return: 0 (false)                                  │
│   → Action: Tiếp tục                                    │
│                                                        │
│ Scenario B: Key đã tồn tại                              │
│   → Return: 1 (true)                                   │
│   → Action: Skip, Delete SQS, increment duplicateSkipped  │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ STEP 3: Acquire processing lock (SETNX)                  │
├─────────────────────────────────────────────────────────────┤
│ Redis: SETNX processing:msg-001 "1:1715100000" EX 300 │
│                                                        │
│ Scenario A: Key không tồn tại (SUCCESS)                  │
│   → Return: TRUE                                        │
│   → Key created: processing:msg-001 = "1:1715100000"    │
│   → TTL set: 300 seconds (5 phút)                       │
│   → Worker 1 acquires lock                               │
│                                                        │
│ Scenario B: Key đã tồn tại (Worker 2 khác)               │
│   → Return: FALSE                                       │
│   → Key giữ nguyên                                       │
│   → Worker 2 skip (duplicateSkipped++)                    │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ STEP 4: Process business logic (LINE push)               │
├─────────────────────────────────────────────────────────────┤
│ LINE API: POST /v2/bot/message/push                    │
│   Body: {"to": "john", "messages": [...]}               │
│                                                        │
│ Scenario A: Success (200 OK)                              │
│   → Action: Tiếp tục đến Step 5                          │
│                                                        │
│ Scenario B: Error (500/timeout)                           │
│   → Action: Release lock, Keep in SQS (retry)              │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ STEP 5: Mark processed (SET)                              │
├─────────────────────────────────────────────────────────────┤
│ Redis Pipeline:                                          │
│   SET processed:msg-001 "1" EX 86400                    │
│   SET user:john:campaign:2025-04-07 "1" EX 604800       │
│                                                        │
│ Scenario A: Success                                      │
│   → Action: Tiếp tục đến Step 6                          │
│                                                        │
│ Scenario B: Error                                        │
│   → Action: Release lock, Keep in SQS                      │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ STEP 6: Release processing lock (DEL)                      │
├─────────────────────────────────────────────────────────────┤
│ Redis: DEL processing:msg-001                             │
│                                                        │
│ Action: Delete lock key                                  │
│ → Key removed: processing:msg-001                        │
│ → Worker 1 releases lock                                  │
│ → Worker khác có thể acquire lock này                      │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ STEP 7: Delete message từ SQS                              │
├─────────────────────────────────────────────────────────────┤
│ SQS API: DeleteMessage(receiptHandle)                    │
│                                                        │
│ Action: Xóa message khỏi queue                           │
│ → Message A được xóa khỏi SQS                             │
│ → processedCount++                                        │
└─────────────────────────────────────────────────────────────┘
```

---

## 🔒 Distributed Lock Mechanism

### **Why Need Distributed Lock?**

**Problem:**
```
SQS Queue
    ↓ (message becomes visible)
    ↓
Worker 1 nhận message A
Worker 2 nhận message A (DUPLICATE PROCESSING!)
Worker 3 nhận message A (DUPLICATE PROCESSING!)
```

**Solution with Redis SETNX:**
```
SQS Queue
    ↓ (message becomes visible)
    ↓
Worker 1: SETNX processing:msg-A → SUCCESS (acquired)
Worker 2: SETNX processing:msg-A → FAIL (skip)
Worker 3: SETNX processing:msg-A → FAIL (skip)
```

**Guarantee:**
- ✅ Only 1 worker processes mỗi message
- ✅ Atomic operation (no race condition)
- ✅ Thread-safe (Redis single-threaded)

---

## ⏰ TTL (Time To Live) - Auto Cleanup

### **Processing Lock TTL: 5 Minutes**

**Purpose:**
```
Nếu worker crash/tôi:
  → Lock key expire sau 5 phút
  → Message visible lại trong SQS
  → Worker khác có thể pick up
```

**Example Timeline:**
```
T+0s:    Worker 1 acquires lock (TTL 5m)
T+10s:   Worker 1 processing LINE message...
T+30s:   Worker 1 CRASH! (process dies)
T+5m:    Redis lock expires automatically
T+5m+10s Message visible lại trong SQS
T+5m+15s Worker 2 receives message
T+5m+16s Worker 2 acquires lock successfully
```

### **Processed Key TTL: 24 Hours**

**Purpose:**
```
Prevent reprocessing khi worker restart:
  → Key expire sau 24 giờ
  → Old keys tự động cleanup
  → Redis memory không bị full
```

---

## 🚫 Error Handling Scenarios

### **Scenario 1: Redis Connection Error**

```go
locked, err := redisProvider.AcquireProcessingLock(ctx, messageID, workerID, lockTTL)
if err != nil {
    atomic.AddInt64(&errorCount, 1)
    log.Printf("Worker %d: error acquiring lock (message: %s): %v", workerID, messageID, err)
    continue  // Skip message, giữ trong SQS để retry
}
```

**Result:**
- ✅ System error (Redis connection issue)
- ✅ Message giữ trong SQS → retry
- ✅ Không mark processed

---

### **Scenario 2: Worker Acquires Lock but Crashes**

```
T+0s:    Worker 1: SETNX processing:msg-A → SUCCESS
T+10s:   Worker 1 processing...
T+30s:   Worker 1 CRASH!
T+5m:    Redis lock expires (TTL 5m)
T+5m+10s Message visible trong SQS
T+5m+15s Worker 2: SETNX processing:msg-A → SUCCESS
T+5m+16s Worker 2 processes message
```

**Result:**
- ✅ No deadlock (TTL auto-cleanup)
- ✅ Message processed thành công
- ✅ No message lost

---

### **Scenario 3: LINE Push Error**

```go
if err := lineProvider.PushMessage(ctx, message.UserID, message.Message); err != nil {
    atomic.AddInt64(&lineErrorCount, 1)
    log.Printf("Worker %d: error sending LINE message (message: %s): %v", workerID, messageID, err)

    redisProvider.ReleaseProcessingLock(ctx, messageID)
    // KHÔNG delete SQS → message sẽ retry
    continue
}
```

**Result:**
- ✅ LINE error (transient network issue)
- ✅ Release lock (để worker khác có thể retry)
- ✅ Message giữ trong SQS → SQS visibility timeout retry

---

## 📊 Redis Keys Summary

| Key Pattern | Value | TTL | Purpose | When Created | When Deleted |
|-------------|-------|-----|---------|-------------|--------------|
| `processing:{messageID}` | `{workerID}:{timestamp}` | 5 min | Lock message đang xử lý | Worker acquires | Worker releases OR TTL expires |
| `processed:{messageID}` | `"1"` | 24h | Track message đã processed | Success path | TTL expires (auto-cleanup) |
| `user:{userID}:campaign:{date}` | `"1"` | 7 days | Deduplication per user/day | Success path | TTL expires (auto-cleanup) |

---

## 🎯 Best Practices

### **1. Always Release Lock**

```go
defer redisProvider.ReleaseProcessingLock(ctx, messageID)
```

**Why:**
- ✅ Prevent deadlock nếu error occur
- ✅ Ensure lock luôn released
- ✅ Go pattern: defer runs even if panic

---

### **2. Use TTL for All Keys**

```go
redis.Set(key, value, ttl)  // Always set TTL
```

**Why:**
- ✅ Prevent memory leaks (old keys expire)
- ✅ Auto-cleanup without manual intervention
- ✅ Safe if worker crashes

---

### **3. Atomic Operations (SETNX, EXISTS, DEL)**

```go
// ✅ Good: SETNX (atomic)
acquired := redis.SetNX(key, value, ttl)

// ❌ Bad: GET + SET (not atomic)
if !redis.Exists(key) {
    redis.Set(key, value, ttl)
    // Race condition: 2 workers có thể cả 2 pass here
}
```

---

### **4. Handle Errors Gracefully**

```go
locked, err := redisProvider.AcquireProcessingLock(...)
if err != nil {
    // Redis error → keep message in SQS for retry
    continue
}
if !locked {
    // Already processing → skip
    duplicateSkipped++
    continue
}
```

---

## 🔍 Debugging Redis Issues

### **Check Redis Keys:**
```bash
# Connect to Redis
redis-cli

# Check processing lock
GET processing:msg-001
# Output: "1:1715100000" (if locked) or nil

# Check if processed
EXISTS processed:msg-001
# Output: 1 (true) or 0 (false)

# Check TTL
TTL processing:msg-001
# Output: 287 (seconds remaining) or -2 (key not exist)
```

### **Monitor Redis:**
```bash
# Watch keys in real-time
redis-cli MONITOR

# Output:
# 1699999999 [0 127.0.0.1:12345] "SETNX" "processing:msg-001" ...
# 1699999999 [0 127.0.0.1:12345] "SET" "processed:msg-001" ...
```

---

## ✅ Summary

### **Redis Operations:**
1. **SETNX** - Acquire lock (atomic)
2. **EXISTS** - Check status
3. **SET** - Mark processed
4. **DEL** - Release lock
5. **TTL** - Auto-cleanup

### **Redis Keys:**
1. `processing:{messageID}` - Distributed lock (5m TTL)
2. `processed:{messageID}` - Track processed (24h TTL)
3. `user:{userID}:campaign:{date}` - Deduplication (7d TTL)

### **Benefits:**
- ✅ Prevent duplicate processing
- ✅ Auto-retry via SQS
- ✅ Auto-cleanup via TTL
- ✅ Thread-safe via Redis atomic operations
- ✅ Fault-tolerant if worker crash
