# Worker Pool Resilience Patterns - Analysis & Trade-offs

**Date:** 2026-04-08
**Context:** sub-service/main.go#69-73
**Problem:** Worker created via for loop, no restart mechanism when worker dies

---

## Current Code Issue

```go
// Current implementation - workers can die and never restart
for i := 0; i < workerCount; i++ {
    wg.Add(1)
    go messageWorker(ctx, &wg, i, ...)
}
```

**Failure modes:**
1. Panic → goroutine exits, worker lost forever
2. Context cancellation → graceful exit but no restart
3. Runtime error → same as panic

---

## Pattern Comparison Matrix

| Pattern | Lines | Complexity | Panic Recovery | Auto Restart | CB Protection | Best For |
|---------|-------|------------|----------------|--------------|---------------|----------|
| 1. Basic Recovery | 5 | ⭐ | ✅ | ❌ | ❌ | Quick fix |
| 2. Restart Loop | 20 | ⭐⭐ | ✅ | ✅ | ❌ | Production |
| 3. Circuit Breaker | 80 | ⭐⭐⭐ | ✅ | ✅ | ✅ | Critical systems |
| 4. Worker Pool Abstraction | 200+ | ⭐⭐⭐⭐⭐ | ✅ | ✅ | ✅ | Libraries |

---

## Pattern 1: Basic Panic Recovery (Simplest)

### Implementation
```go
func messageWorker(...) {
    defer func() {
        if r := recover(); r != nil {
            atomic.AddInt64(&panicCount, 1)
            log.Printf("Worker %d panicked: %v\nStack: %s", workerID, r, debug.Stack())
        }
        wg.Done()
    }()
    // ... existing worker logic
}
```

### Trade-offs
| Aspect | Pros | Cons |
|--------|------|------|
| **Code** | 5 lines, zero refactoring | Worker dies permanently |
| **Reliability** | Prevents crash | Gradual worker loss |
| **Monitoring** | Easy to count panics | No auto-remediation |
| **Testing** | No test needed | Not testable |

### When to Use
- Quick fix for production incident
- Worker panics are extremely rare
- Manual intervention acceptable

### When to Avoid
- High availability requirement
- Unknown code quality (panics likely)

---

## Pattern 2: Supervisor Restart Loop (Recommended)

### Implementation
```go
func startSupervisedWorker(ctx context.Context, workerID int, ...) {
    defer wg.Done()

    for {
        // Check graceful shutdown
        if ctx.Err() != nil {
            log.Printf("Worker %d: graceful shutdown", workerID)
            return
        }

        // Run worker with panic recovery
        func() {
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("Worker %d panic recovered: %v\n%s",
                        workerID, r, debug.Stack())
                    atomic.AddInt64(&panicCount, 1)
                }
            }()
            messageWorker(ctx, workerID, ...)
        }()

        // Backoff before restart
        select {
        case <-time.After(1 * time.Second):
        case <-ctx.Done():
            return
        }
    }
}

// In main()
for i := 0; i < workerCount; i++ {
    wg.Add(1)
    go startSupervisedWorker(ctx, i, ...)
}
```

### Trade-offs
| Aspect | Pros | Cons |
|--------|------|------|
| **Code** | ~20 lines | Need to refactor worker signature |
| **Reliability** | Auto-restart workers | No circuit breaker |
| **Availability** | Always N workers | Flapping workers spam log |
| **Monitoring** | Panic counter available | Need external alerting |
| **Shutdown** | Clean graceful exit | Slightly complex |

### When to Use
- **Production systems (default choice)**
- Want reliability without over-engineering
- Panics are bugs to fix, not transient failures

### When to Avoid
- Need per-worker circuit breaking
- Regulatory requirements demand maximum safety

---

## Pattern 2+: Hybrid with Panic Warning

### Implementation (Pattern 2 + Global Monitor)

```go
// Global state
var panicCount atomic.Int64
var firstPanicTime atomic.Value // time.Time

func incrementPanicCounter() {
    count := panicCount.Add(1)
    if count == 1 {
        firstPanicTime.Store(time.Now())
    }

    // Warning threshold: 10 panics in 1 minute
    if count >= 10 {
        first := firstPanicTime.Load().(time.Time)
        if time.Since(first) < 1*time.Minute {
            log.Printf("CRITICAL: %d panics in %v! BUG DETECTED!",
                count, time.Since(first))
            // Send alert here (PagerDuty, Slack, etc.)
        }
    }
}

func messageWorker(...) {
    defer func() {
        if r := recover(); r != nil {
            incrementPanicCounter()
            log.Printf("Worker %d panic: %v", workerID, r)
        }
    }()
    // ...
}
```

### Trade-offs
| Aspect | Pros | Cons |
|--------|------|------|
| **Code** | +15 lines | Global state |
| **Visibility** | Panic surge alerting | No worker isolation |
| **Cost** | Very cheap | Might miss gradual issues |
| **Ops** | Easy to monitor | Need alert integration |

### When to Use
- Want Pattern 2 but need observability
- Small team, can't afford complex CB
- Alert-driven operations model

---

## Pattern 3: Per-Worker Circuit Breaker

### Implementation
```go
type WorkerSupervisor struct {
    workerID      int
    maxRestarts   int           // e.g., 5
    restartWindow time.Duration // e.g., 1 minute
    restarts      []time.Time
    mu            sync.Mutex
}

func (s *WorkerSupervisor) shouldRestart() bool {
    s.mu.Lock()
    defer s.mu.Unlock()

    now := time.Now()
    cutoff := now.Add(-s.restartWindow)

    // Remove old restarts outside window
    n := 0
    for _, t := range s.restarts {
        if t.After(cutoff) {
            s.restarts[n] = t
            n++
        }
    }
    s.restarts = s.restarts[:n]

    if len(s.restarts) >= s.maxRestarts {
        log.Printf("Worker %d: CIRCUIT OPEN - %d restarts in %v",
            s.workerID, len(s.restarts), s.restartWindow)
        return false
    }

    s.restarts = append(s.restarts, now)
    return true
}

func (s *WorkerSupervisor) run(ctx context.Context) {
    for {
        if ctx.Err() != nil {
            return
        }

        if !s.shouldRestart() {
            // Circuit open - wait before retry
            select {
            case <-time.After(30 * time.Second):
                log.Printf("Worker %d: attempting circuit reset", s.workerID)
                s.mu.Lock()
                s.restarts = nil // Reset circuit
                s.mu.Unlock()
                continue
            case <-ctx.Done():
                return
            }
        }

        // Run worker
        func() {
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("Worker %d panic: %v", s.workerID, r)
                }
            }()
            messageWorker(ctx, s.workerID, ...)
        }()
    }
}
```

### Trade-offs
| Aspect | Pros | Cons |
|--------|------|------|
| **Reliability** | Full protection | 80+ lines code |
| **Safety** | Prevents cascading failures | Complex to test |
| **Config** | Tunable thresholds | More config surface |
| **Ops** | Self-healing | Need to understand CB |

### When to Use
- **Critical production systems**
- Multi-tenant (one tenant can't affect others)
- Regulatory requirements
- Large team, can maintain complexity

### Configuration Guidelines
```go
// Conservative
maxRestarts: 3
restartWindow: 5 * time.Minute

// Balanced (recommended)
maxRestarts: 5
restartWindow: 1 * time.Minute

// Aggressive
maxRestarts: 10
restartWindow: 30 * time.Second
```

---

## Pattern 4: Worker Pool Library Abstraction

### Overview
Full worker pool library with:
- Generic task interface
- Pluggable middleware
- Metrics middleware
- Dynamic scaling
- Graceful shutdown

### Trade-offs
| Aspect | Pros | Cons |
|--------|------|------|
| **Reusability** | Can use across projects | Overkill for single use |
| **Testability** | Easy to unit test | 200+ lines to write |
| **Flexibility** | Highly configurable | Learning curve |
| **Maintenance** | Long-term benefit | Upfront cost |

### When to Use
- Building a shared library
- Multiple services need worker pools
- Team has capacity for infrastructure work

### When to Avoid
- Single service, specific use case
- Team is small/time-constrained
- YAGNI principle applies

---

## Final Recommendation by Scenario

### Your Case: sub-service with SQS workers

**Recommended: Pattern 2 (Restart Loop)**

**Reasoning:**
1. **KISS**: 20 lines vs 80+ lines
2. **Panic = Bug**: If worker panics, FIX the bug, don't hide it
3. **SQS Visibility Timeout**: If worker dies, SQS auto-requeue message
4. **Already have Redis dedupe**: Duplicate processing is handled
5. **Circuit breaker overkill**: Transient failures (LINE API timeout) should be retried with backoff, not circuit-break

**Code snippet for immediate use:**
```go
func startSupervisedWorker(ctx context.Context, workerID int, ...) {
    defer wg.Done()

    for {
        if ctx.Err() != nil {
            return
        }

        func() {
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("Worker %d panic: %v", workerID, r)
                }
            }()
            messageWorker(ctx, workerID, ...)
        }()

        select {
        case <-time.After(1 * time.Second):
        case <-ctx.Done():
            return
        }
    }
}
```

---

## Migration Path

### Step 1: Add panic recovery (Pattern 1)
- Deploy today, prevents crashes
- Monitor panic count

### Step 2: Add restart loop (Pattern 2)
- Refactor after Step 1 validated
- Test graceful shutdown

### Step 3: Add monitoring (Pattern 2+)
- Add panic surge alerting
- Set up PagerDuty/Slack integration

### Step 4: Consider circuit breaker (Pattern 3)
- Only if panics are frequent
- Or if regulatory requirement

---

## Unresolved Questions

1. **Current panic rate**: Do you know how often workers panic?
2. **Alerting**: What's your alerting setup (PagerDuty, Slack, etc.)?
3. **MTTR**: How fast can you respond to and fix bugs?
4. **SLA**: Do you have uptime requirements that justify circuit breaker?

---

## Next Steps

Choose pattern based on:
1. Current panic frequency (check logs)
2. Team capacity (can you maintain complex code?)
3. Production criticality (what's the impact of downtime?)

**For 99% of cases:** Pattern 2 is the right balance.
