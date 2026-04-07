# messageWorker() - Chi Tiết Giải Thích Workflow

## 📋 Overview

`messageWorker()` là core function xử lý message từ SQS với distributed lock để prevent duplicate processing.

**Signature Function:**
```go
func messageWorker(
    ctx context.Context,           // Context cho cancellation
    wg *sync.WaitGroup,         // WaitGroup để track goroutine
    workerID int,                // ID của worker (để log)
    sqsProvider *providers.SQSProvider,         // SQS client
    redisProvider *providers.RedisProvider,       // Redis client
    lineProvider *providers.LINEProvider,         // LINE API client
    messageChannel <-chan providers.SQSMessage,   // Channel nhận message
    processedCount *int64,       // Counter messages processed thành công
    errorCount *int64,          // Counter system errors
    lineErrorCount *int64,       // Counter LINE errors
    duplicateSkipped *int64,     // Counter messages bị skip
    lockTTL time.Duration,       // TTL cho processing lock
)
```

---

## 🔄 Main Loop Structure

### **Loop Initialization**
```go
defer wg.Done()  // Đảm bảo goroutine được cleanup khi exit

for {
    msg, ok := <-messageChannel  // Block đợi message từ channel
    if !ok {
        return  // Channel closed, goroutine exit
    }
    // Process message...
}
```

**Tại sao dùng channel?**
- ✅ **Decoupling**: Message receiver không cần đợi worker xong
- ✅ **Buffering**: Channel buffered (100) prevents blocking
- ✅ **Load balancing**: Workers tự động pull từ channel

---

## 📝 Step-by-Step Processing Workflow

---

### **STEP 1: Validate Message**

```go
if msg.ParsedMessage == nil {
    atomic.AddInt64(errorCount, 1)
    messageID := "unknown"
    if msg.MessageId != nil {
        messageID = *msg.MessageId
    }
    log.Printf("Worker %d: received message with nil parsed content (messageID: %s)", workerID, messageID)
    sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle)
    continue
}
```

**Giải thích:**
1. **Check**: `msg.ParsedMessage` là nil?
   - Nếu nil → JSON parsing failed hoặc message format không đúng
2. **Action**:
   - Increment `errorCount` (system error)
   - Extract messageID nếu có (hoặc "unknown")
   - Log lỗi với workerID
   - **Delete message từ SQS** (message invalid, không thể process)
   - `continue` (bỏ qua, xử lý message tiếp theo)

**Tại sao delete từ SQS?**
- ❌ Message format sai, không thể process
- ✅ Delete để message không bị stuck trong queue
- ✅ Không làm load hệ thống

---

### **STEP 2: Extract Message Content**

```go
message := msg.ParsedMessage
messageID := message.MessageID
```

**Giải thích:**
1. **Extract parsed message**: `msg.ParsedMessage` chứa:
   - `message.MessageID` → Unique ID
   - `message.UserID` → User ID để gửi LINE
   - `message.Message` → Nội dung message

2. **Extract messageID**: Sử dụng `messageID` trong các Redis operations

---

### **STEP 3: Check Duplicate Processing (Redis EXISTS)**

```go
isProcessed, err := redisProvider.CheckProcessed(ctx, messageID)
if err != nil {
    atomic.AddInt64(errorCount, 1)
    log.Printf("Worker %d: error checking processed status (message: %s): %v", workerID, messageID, err)
    continue
}
if isProcessed {
    log.Printf("Worker %d: message %s already processed, skipping", workerID, messageID)
    sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle)
    atomic.AddInt64(duplicateSkipped, 1)
    continue
}
```

**Redis Operation (CheckProcessed):**
```redis
EXISTS processed:{messageID}
```

**Giải thích:**

#### **3a. Redis Error Handling**
```go
if err != nil {
    atomic.AddInt64(&errorCount, 1)
    log.Printf("Worker %d: error checking processed status (message: %s): %v", ...)
    continue
}
```
- **Error scenario**: Redis connection failed, timeout
- **Action**: Increment `errorCount`, log error, `continue`
- **Why continue**: Giữ message trong SQS để retry (worker khác có thể process)

#### **3b. Already Processed Handling**
```go
if isProcessed {
    log.Printf("Worker %d: message %s already processed, skipping", ...)
    sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle)
    atomic.AddInt64(duplicateSkipped, 1)
    continue
}
```
- **Scenario**: Redis key `processed:{messageID}` đã tồn tại
- **Meaning**: Message đã được process thành công trước đó
- **Action**:
  1. **Delete message từ SQS**: Message invalid vì đã process
  2. **Increment `duplicateSkipped`**: Track số message bị skip
  3. **Continue**: Xử lý message tiếp theo

**Tại sao cần step này?**
- ✅ **Prevent reprocessing**: Worker restart không process lại message
- ✅ **At-least-once delivery**: Đảm bảo message process tối đa 1 lần
- ✅ **Consistency**: Track message đã processed trong 24h (TTL)

---

### **STEP 4: Acquire Distributed Lock (Redis SETNX)**

```go
locked, err := redisProvider.AcquireProcessingLock(ctx, messageID, workerID, lockTTL)
if err != nil {
    atomic.AddInt64(errorCount, 1)
    log.Printf("Worker %d: error acquiring lock (message: %s): %v", workerID, messageID, err)
    continue
}
if !locked {
    log.Printf("Worker %d: message %s already locked by another worker, skipping", workerID, messageID)
    atomic.AddInt64(duplicateSkipped, 1)
    continue
}
```

**Redis Operation (AcquireProcessingLock):**
```redis
SETNX processing:{messageID} "{workerID}:{timestamp}" EX 300
```

**Giải thích:**

#### **4a. Redis Error Handling**
```go
if err != nil {
    atomic.AddInt64(&errorCount, 1)
    log.Printf("Worker %d: error acquiring lock (message: %s): %v", ...)
    continue
}
```
- **Error scenario**: Redis connection failed
- **Action**: Increment `errorCount`, log error, `continue`
- **Why continue**: Giữ message trong SQS để retry

#### **4b. Lock Acquisition Failed**
```go
if !locked {
    log.Printf("Worker %d: message %s already locked by another worker, skipping", ...)
    atomic.AddInt64(duplicateSkipped, 1)
    continue
}
```
- **Scenario**: SETNX returned FALSE
- **Meaning**: Worker khác đã acquire lock cho message này
- **Action**:
  1. **Increment `duplicateSkipped`**: Track duplicate processing attempt
  2. **Continue**: Skip message này

**What is SETNX?**
- **SET** **N**ot E**X**ists** = "SET if Not Exists"
- **Atomic operation**: Chỉ 1 worker có thể acquire lock
- **Returns**:
  - TRUE → Lock acquired (key not exist)
  - FALSE → Lock not acquired (key already exist)

**Example Timeline:**
```
T+0ms:   Worker 1: SETNX processing:msg-001 "1:1715100000" EX 300
          → SUCCESS (return TRUE) → Lock acquired

T+5ms:   Worker 2: SETNX processing:msg-001 "2:1715100005" EX 300
          → FAIL (return FALSE) → Worker 1 đang xử lý

T+10ms:  Worker 3: SETNX processing:msg-001 "3:1715100010" EX 300
          → FAIL (return FALSE) → Worker 1 đang xử lý
```

**Tại sao cần TTL (Time To Live)?**
```go
EX 300  // 300 seconds = 5 minutes
```
- ✅ **Auto-cleanup**: Nếu worker crash, lock expire sau 5 phút
- ✅ **Prevent deadlock**: Message không bị lock mãi
- ✅ **Safety**: Worker khác có thể acquire lock sau TTL expire

**Example Scenario - Worker Crash:**
```
T+0s:    Worker 1: SETNX processing:msg-A → SUCCESS
          Worker 1: Acquire lock (TTL 5m)

T+10s:   Worker 1: Processing LINE message...

T+30s:   Worker 1: CRASH! (process dies)

T+5m:    Redis: TTL expire → Key "processing:msg-A" deleted
          Worker 1 no longer holding lock

T+5m+10s: Worker 2: SETNX processing:msg-A → SUCCESS
          Worker 2: Acquire lock (worker 1 crashed)
          Worker 2: Process message A
```

---

### **STEP 5: Process Business Logic (LINE Push)**

```go
if err := lineProvider.PushMessage(ctx, message.UserID, message.Message); err != nil {
    atomic.AddInt64(lineErrorCount, 1)
    log.Printf("Worker %d: error sending LINE message (message: %s): %v", workerID, messageID, err)

    redisProvider.ReleaseProcessingLock(ctx, messageID)

    continue
}
```

**LINE Operation:**
```go
lineProvider.PushMessage(ctx, message.UserID, message.Message)
```
**Sends HTTP POST to:**
```http
POST http://fake-line-server:3000/v2/bot/message/push
Content-Type: application/json
Authorization: Bearer mock_token

{
  "to": message.UserID,
  "messages": [
    {
      "type": "text",
      "text": message.Message
    }
  ]
}
```

**Giải thích:**

#### **5a. LINE Success (HTTP 200 OK)**
- **Action**: Continue to next step (Step 6)
- **No error handling needed**: Proceed normally

#### **5b. LINE Error (HTTP 500/Timeout)**
```go
if err != nil {
    atomic.AddInt64(lineErrorCount, 1)
    log.Printf("Worker %d: error sending LINE message (message: %s): %v", ...)

    redisProvider.ReleaseProcessingLock(ctx, messageID)

    continue
}
```
- **Scenario**: LINE API returned error hoặc timeout
- **Action**:
  1. **Increment `lineErrorCount`**: Track LINE errors
  2. **Log error**: Debug information
  3. **Release lock**: `DEL processing:{messageID}`
  4. **Continue**: Bỏ qua, KHÔNG delete message từ SQS

**Tại sao không delete từ SQS khi LINE error?**
- ✅ **Retry mechanism**: SQS visibility timeout sẽ retry message
- ✅ **Transient errors**: Network issues, LINE downtime có thể recover
- ✅ **No message lost**: Message giữ trong SQS cho đến khi success

**Example Timeline - LINE Error:**
```
T+0s:    Worker 1: SETNX processing:msg-001 → SUCCESS
          Worker 1: Processing LINE message...

T+2s:    LINE API: HTTP 500 Internal Server Error
          Worker 1: Error sending LINE message
          Worker 1: Release lock (DEL processing:msg-001)
          Worker 1: Continue (keep message in SQS)

T+30s:   SQS: Visibility timeout expire
          Message becomes visible again

T+35s:   Worker 2: Receive message 001
          Worker 2: SETNX processing:msg-001 → SUCCESS
          Worker 2: Retry LINE message
```

---

### **STEP 6: Mark Processed (Redis SET)**

```go
if err := redisProvider.MarkProcessed(ctx, messageID, message.UserID); err != nil {
    atomic.AddInt64(errorCount, 1)
    log.Printf("Worker %d: error marking processed (message: %s): %v", workerID, messageID, err)
    redisProvider.ReleaseProcessingLock(ctx, messageID)
    continue
}
```

**Redis Operations (MarkProcessed):**
```redis
SET processed:{messageID} "1" EX 86400
SET user:{userID}:campaign:{date} "1" EX 604800
```

**Giải thích:**

#### **6a. Redis Error Handling**
```go
if err != nil {
    atomic.AddInt64(errorCount, 1)
    log.Printf("Worker %d: error marking processed (message: %s): %v", ...)

    redisProvider.ReleaseProcessingLock(ctx, messageID)

    continue
}
```
- **Scenario**: Redis connection failed
- **Action**:
  1. **Increment `errorCount`**: System error
  2. **Log error**: Debug information
  3. **Release lock**: Let other workers retry
  4. **Continue**: Keep message in SQS

#### **6b. Success Path**
- **Scenario**: Both Redis SET operations succeed
- **Action**: Continue to next step (Step 7)
- **Redis keys created**:
  - `processed:{messageID}` → "1" (TTL 24h)
  - `user:{userID}:campaign:{date}` → "1" (TTL 7 days)

**Why 2 keys?**

| Key | Purpose | TTL | Prevents |
|-----|---------|-----|----------|
| `processed:{messageID}` | Track message processed | 24h | Same message reprocessing |
| `user:{userID}:campaign:{date}` | Deduplicate per user/day | 7 days | Multiple messages same user/day |

**Example:**
```redis
processed:msg-001 = "1" (expires 2025-04-08)
processed:msg-002 = "1" (expires 2025-04-08)
user:john:campaign:2025-04-07 = "1" (expires 2025-04-14)
```

---

### **STEP 7: Release Processing Lock (Redis DEL)**

```go
redisProvider.ReleaseProcessingLock(ctx, messageID)
```

**Redis Operation:**
```redis
DEL processing:{messageID}
```

**Giải thích:**

- **Always execute**: Nơi này luôn chạy (không có error check)
- **Delete lock key**: Remove `processing:{messageID}` from Redis
- **Purpose**: Release lock để worker khác có thể process message này (nếu cần)

**When lock is released?**
- ✅ **Success path**: Sau khi mark processed (Step 6)
- ✅ **Error path**: Sau khi Redis error (Step 6a)
- ✅ **LINE error path**: Sau khi LINE push fail (Step 5b)

**Example Timeline:**
```
T+0s:    Worker 1: SETNX processing:msg-001 → SUCCESS
          Worker 1: Lock acquired

T+5s:    Worker 1: Process LINE message → SUCCESS
          Worker 1: Mark processed → SUCCESS
          Worker 1: DEL processing:msg-001 → Lock released

T+6s:    Worker 2: SETNX processing:msg-001 → SUCCESS
          Worker 2: Lock acquired (worker 1 released)
```

---

### **STEP 8: Delete Message from SQS**

```go
if err := sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
    atomic.AddInt64(errorCount, 1)
    log.Printf("Worker %d: error deleting message from SQS (message: %s): %v", workerID, messageID, err)
    continue
}
```

**SQS Operation:**
```go
sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle)
```
**Calls AWS SQS API:**
```http
DELETE /?Action=DeleteMessage
&ReceiptHandle={receiptHandle}
```

**Giải thích:**

#### **8a. SQS Error Handling**
```go
if err != nil {
    atomic.AddInt64(errorCount, 1)
    log.Printf("Worker %d: error deleting message from SQS (message: %s): %v", ...)

    continue
}
```
- **Scenario**: AWS SQS API failed
- **Action**:
  1. **Increment `errorCount`**: System error
  2. **Log error**: Debug information
  3. **Continue**: Message sẽ visible lại (SQS visibility timeout)

#### **8b. Success Path**
```go
atomic.AddInt64(processedCount, 1)
log.Printf("Worker %d: successfully processed message %s", workerID, messageID)
```
- **Scenario**: Delete thành công
- **Action**:
  1. **Increment `processedCount`**: Track success messages
  2. **Log success**: Audit trail
  3. **Loop continues**: Chờ message tiếp theo

**Why delete only here?**
- ✅ **At-least-once delivery**: Đảm bảo message process xong rồi mới delete
- ✅ **Error recovery**: Nếu process fail, message giữ trong SQS
- ✅ **Consistency**: Message không bị lost

---

## 🎯 Complete Flow Example

### **Success Path Scenario:**

```
Time    Worker 1                               Redis                              SQS              LINE
─────────────────────────────────────────────────────────────────────────────
T+0ms   Receive message msg-001
        ↓ Extract: messageID="msg-001"
        ↓ Check processed: EXISTS processed:msg-001
                                      → FALSE (not exists)
        ↓ Acquire lock: SETNX processing:msg-001
                                      → TRUE (success)
                                      → Key created: "1:1715100000"
                                      → TTL: 300s
        ↓ Process LINE: PushMessage("john", "Hello")
                                                                                 → SUCCESS (200 OK)
        ↓ Mark processed: SET processed:msg-001
                                      → SUCCESS
                                      → Key created: "1"
                                      → TTL: 86400s
                                      SET user:john:campaign:2025-04-07
                                      → SUCCESS
                                      → Key created: "1"
                                      → TTL: 604800s
        ↓ Release lock: DEL processing:msg-001
                                      → SUCCESS
                                      → Key deleted
        ↓ Delete SQS: DeleteMessage(receiptHandle)
                                                           → SUCCESS
                                                           → Message removed from queue
        ↓ Increment processedCount++
        ↓ Log: "successfully processed message msg-001"
        ↓ Loop continues...
```

---

### **Duplicate Processing Prevention Scenario:**

```
Time    Worker 1                               Worker 2                           Redis
────────────────────────────────────────────────────────────────────────
T+0ms   Receive message msg-002
        ↓ Check processed: EXISTS processed:msg-002
                                      → FALSE (not exists)
        ↓ Acquire lock: SETNX processing:msg-002
                                      → TRUE (success)
                                      → Key created: "1:1715100000"
                                      → TTL: 300s
        ↓ Process LINE...
        ↓
        ↓                                        Receive message msg-002
                                                  ↓ Check processed: EXISTS processed:msg-002
                                                  → FALSE (not exists)
                                                  ↓ Acquire lock: SETNX processing:msg-002
                                                  → FALSE (key exists!)
                                                  → Worker 1 đang xử lý
                                                  ↓ Increment duplicateSkipped++
                                                  ↓ Log: "already locked, skipping"
                                                  ↓ Continue (skip message)
```

**Result:**
- ✅ Worker 1 processes message msg-002
- ✅ Worker 2 skips message msg-002
- ✅ No duplicate processing
- ✅ Worker 2 receives next message from SQS

---

### **Error Handling Scenario (LINE Error):**

```
Time    Worker 1                               Redis                              SQS              LINE
───────────────────────────────────────────────────────────────────────────────────
T+0ms   Receive message msg-003
        ↓ Check processed: EXISTS processed:msg-003
                                      → FALSE (not exists)
        ↓ Acquire lock: SETNX processing:msg-003
                                      → TRUE (success)
                                      → Key created: "1:1715100000"
                                      → TTL: 300s
        ↓ Process LINE: PushMessage("mary", "Test")
                                                                                 → ERROR (500 Internal Server Error)
        ↓ Increment lineErrorCount++
        ↓ Log: "error sending LINE message"
        ↓ Release lock: DEL processing:msg-003
                                      → SUCCESS
                                      → Key deleted
        ↓ Continue (KHÔNG delete SQS!)
        ↓
        ↓ Loop continues (chờ message tiếp theo)
        ↓
T+30s   SQS: Visibility timeout expire
                                                           → Message msg-003 visible again
        ↓                                        Receive message msg-003
                                                  ↓ Check processed: EXISTS processed:msg-003
                                                  → FALSE (not exists)
                                                  ↓ Acquire lock: SETNX processing:msg-003
                                                  → TRUE (success)
                                                  ↓ Retry LINE push...
```

**Result:**
- ✅ Message not deleted from SQS (keeps for retry)
- ✅ Worker 2 receives message after SQS visibility timeout
- ✅ Automatic retry mechanism via SQS
- ✅ No message lost

---

## 🔑 Key Concepts Summary

### **1. Distributed Lock (SETNX)**
- **Atomic**: Prevents race condition
- **SETNX**: "SET if Not Exists"
- **Returns**: TRUE (success) or FALSE (failed)
- **Purpose**: Only 1 worker processes each message

### **2. Redis TTL (Time To Live)**
- **Processing lock**: 5 minutes (auto-cleanup if worker crash)
- **Processed key**: 24 hours (prevent reprocessing)
- **Campaign key**: 7 days (user deduplication)
- **Auto-expire**: No manual cleanup needed

### **3. Continue Pattern**
- **Purpose**: Skip current message, continue to next
- **Usage**: All error scenarios
- **Benefit**: Workers never stuck, always processing

### **4. Atomic Operations**
- **SETNX**: Atomic acquire lock
- **EXISTS**: Atomic check status
- **SET**: Atomic mark processed
- **DEL**: Atomic release lock

---

## 📊 Error Handling Strategy

| Scenario | Action | Counter | SQS | Redis Lock |
|----------|--------|---------|------|------------|
| **nil parsed message** | Delete SQS | `errorCount++` | ✅ Delete | ❌ N/A |
| **Redis connection error** | Continue | `errorCount++` | ❌ Keep | ❌ N/A |
| **Already processed** | Delete SQS | `duplicateSkipped++` | ✅ Delete | ❌ N/A |
| **Lock acquisition fail** | Continue | `duplicateSkipped++` | ❌ Keep | ❌ N/A |
| **LINE push error** | Continue | `lineErrorCount++` | ❌ Keep | ✅ Release |
| **Mark processed error** | Continue | `errorCount++` | ❌ Keep | ✅ Release |
| **Delete SQS error** | Continue | `errorCount++` | ❌ Keep | ✅ Released |

---

## ✅ Guarantees Provided

### **1. At-Least-Once Delivery**
- ✅ Message processed tối đa 1 lần
- ✅ Duplicate prevention via Redis lock
- ✅ Redis processed key prevents reprocessing

### **2. Fault Tolerance**
- ✅ Worker crash → TTL auto-cleanup
- ✅ Redis error → Message kept for retry
- ✅ LINE error → Message kept for retry

### **3. Auto-Retry Mechanism**
- ✅ SQS visibility timeout (30s default)
- ✅ Message reappears if not deleted
- ✅ No retry counter needed (simplified)

### **4. No Message Loss**
- ✅ Only delete on successful processing
- ✅ Error scenarios keep message in SQS
- ✅ SQS ensures at-least-once delivery

---

## 🎯 Summary

`messageWorker()` function:
1. **Loop**: Chờ message từ channel
2. **Validate**: Check message format
3. **Check Duplicate**: Redis EXISTS
4. **Acquire Lock**: Redis SETNX (atomic)
5. **Process**: LINE push
6. **Mark Processed**: Redis SET (2 keys)
7. **Release Lock**: Redis DEL
8. **Delete SQS**: AWS API
9. **Track Metrics**: Update counters
10. **Continue**: Loop to next message

**Redis Operations per message:**
- EXISTS (check duplicate)
- SETNX (acquire lock)
- SET (mark processed) x2
- DEL (release lock)
- **Total: 5 Redis operations**

**Complexity:**
- ✅ Simple, linear flow
- ✅ No complex state management
- ✅ Error handling at each step
- ✅ Continue pattern prevents deadlock
