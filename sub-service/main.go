package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"sub-service/providers"

	"github.com/gin-gonic/gin"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Sub-Service...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sqsProvider, err := providers.NewSQSProvider(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize SQS provider: %v", err)
	}

	redisProvider, err := providers.NewRedisProvider(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Redis provider: %v", err)
	}

	lineProvider, err := providers.NewLINEProvider()
	if err != nil {
		log.Fatalf("Failed to initialize LINE provider: %v", err)
	}

	defer redisProvider.Close()
	log.Println("Providers initialized (SQS, Redis, LINE)")

	workerCount := 50
	if wc := os.Getenv("SQS_WORKER_COUNT"); wc != "" {
		var parsed int
		if _, err := fmt.Sscanf(wc, "%d", &parsed); err == nil && parsed > 0 {
			workerCount = parsed
		}
	}
	log.Printf("Starting %d workers", workerCount)

	lockTTL := 5 * time.Minute
	if lt := os.Getenv("PROCESSING_LOCK_TTL"); lt != "" {
		if d, err := time.ParseDuration(lt); err == nil {
			lockTTL = d
		}
	}
	log.Printf("Processing lock TTL: %v", lockTTL)

	var processedCount int64
	var errorCount int64
	var lineErrorCount int64
	var duplicateSkipped int64
	var panicCount int64

	messageChannel := make(chan providers.SQSMessage, 100)

	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go startSupervisedWorker(ctx, &wg, i, sqsProvider, redisProvider, lineProvider, messageChannel, &processedCount, &errorCount, &lineErrorCount, &duplicateSkipped, &panicCount, lockTTL)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			log.Printf("Metrics - Processed: %d, Errors: %d, LINE Errors: %d, Duplicates: %d, Panics: %d, Goroutines: %d",
				atomic.LoadInt64(&processedCount),
				atomic.LoadInt64(&errorCount),
				atomic.LoadInt64(&lineErrorCount),
				atomic.LoadInt64(&duplicateSkipped),
				atomic.LoadInt64(&panicCount),
				runtime.NumGoroutine())
		}
	}()

	go func() {
		maxMessages := int32(10)
		waitTimeSeconds := int32(20)
		sqsErrorCount := 0

		for {
			messages, err := sqsProvider.ReceiveMessages(ctx, maxMessages, waitTimeSeconds)
			if err != nil {
				sqsErrorCount++
				if sqsErrorCount%10 == 0 {
					log.Printf("Error receiving messages (total errors: %d): %v", sqsErrorCount, err)
				}
				time.Sleep(1 * time.Second)
				continue
			}

			if len(messages) == 0 {
				continue
			}

			for _, msg := range messages {
				messageChannel <- msg
			}
		}
	}()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "sub-service",
			"metrics": gin.H{
				"processed":   atomic.LoadInt64(&processedCount),
				"errors":      atomic.LoadInt64(&errorCount),
				"line_errors": atomic.LoadInt64(&lineErrorCount),
				"duplicates":  atomic.LoadInt64(&duplicateSkipped),
				"panics":      atomic.LoadInt64(&panicCount),
			},
			"workers":    workerCount,
			"goroutines": runtime.NumGoroutine(),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func startSupervisedWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	workerID int,
	sqsProvider *providers.SQSProvider,
	redisProvider *providers.RedisProvider,
	lineProvider *providers.LINEProvider,
	messageChannel <-chan providers.SQSMessage,
	processedCount *int64,
	errorCount *int64,
	lineErrorCount *int64,
	duplicateSkipped *int64,
	panicCount *int64,
	lockTTL time.Duration,
) {
	defer wg.Done()

	log.Printf("Worker %d: starting supervised worker", workerID)

	for {
		if ctx.Err() != nil {
			log.Printf("Worker %d: graceful shutdown", workerID)
			return
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(panicCount, 1)
					log.Printf("Worker %d: panic recovered - %v\n%s", workerID, r, debug.Stack())
				}
			}()
			messageWorker(ctx, workerID, sqsProvider, redisProvider, lineProvider, messageChannel, processedCount, errorCount, lineErrorCount, duplicateSkipped, lockTTL)
		}()

		select {
		case <-time.After(1 * time.Second):
			log.Printf("Worker %d: restarting after backoff", workerID)
		case <-ctx.Done():
			log.Printf("Worker %d: shutdown during backoff", workerID)
			return
		}
	}
}

func messageWorker(
	ctx context.Context,
	workerID int,
	sqsProvider *providers.SQSProvider,
	redisProvider *providers.RedisProvider,
	lineProvider *providers.LINEProvider,
	messageChannel <-chan providers.SQSMessage,
	processedCount *int64,
	errorCount *int64,
	lineErrorCount *int64,
	duplicateSkipped *int64,
	lockTTL time.Duration,
) {
	for {
		msg, ok := <-messageChannel
		if !ok {
			return
		}

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

		message := msg.ParsedMessage
		messageID := message.MessageID

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

		if err := lineProvider.PushMessage(ctx, message.UserID, message.Message); err != nil {
			atomic.AddInt64(lineErrorCount, 1)
			log.Printf("Worker %d: error sending LINE message (message: %s): %v", workerID, messageID, err)

			redisProvider.ReleaseProcessingLock(ctx, messageID)

			continue
		}

		if err := redisProvider.MarkProcessed(ctx, messageID, message.UserID); err != nil {
			atomic.AddInt64(errorCount, 1)
			log.Printf("Worker %d: error marking processed (message: %s): %v", workerID, messageID, err)
			redisProvider.ReleaseProcessingLock(ctx, messageID)
			continue
		}

		redisProvider.ReleaseProcessingLock(ctx, messageID)

		if err := sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
			atomic.AddInt64(errorCount, 1)
			log.Printf("Worker %d: error deleting message from SQS (message: %s): %v", workerID, messageID, err)
			continue
		}

		atomic.AddInt64(processedCount, 1)
		log.Printf("Worker %d: successfully processed message %s", workerID, messageID)
	}
}
