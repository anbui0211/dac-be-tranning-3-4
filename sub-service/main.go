package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
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
	log.Println("SQS provider initialized")

	redisProvider, err := providers.NewRedisProvider(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize Redis provider: %v", err)
	}

	defer redisProvider.Close()
	log.Println("Redis provider initialized")

	workerCount := 50
	if wc := os.Getenv("SQS_WORKER_COUNT"); wc != "" {
		var parsed int
		if _, err := fmt.Sscanf(wc, "%d", &parsed); err == nil && parsed > 0 {
			workerCount = parsed
		}
	}
	log.Printf("Starting %d workers", workerCount)

	var processedCount int64
	var duplicateCount int64
	var errorCount int64

	messageChannel := make(chan providers.SQSMessage, 100)

	var wg sync.WaitGroup

	shutdown := make(chan struct{})

	shutdownWorkers := func() {
		log.Println("Shutting down workers...")
		close(shutdown)
		wg.Wait()
		log.Println("All workers stopped")
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go messageWorker(ctx, &wg, i, sqsProvider, redisProvider, messageChannel, shutdown, &processedCount, &duplicateCount, &errorCount)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Printf("Metrics - Processed: %d, Duplicates: %d, Errors: %d, Goroutines: %d",
					atomic.LoadInt64(&processedCount),
					atomic.LoadInt64(&duplicateCount),
					atomic.LoadInt64(&errorCount),
					runtime.NumGoroutine())
			case <-shutdown:
				return
			}
		}
	}()

	go func() {
		maxMessages := int32(10)
		waitTimeSeconds := int32(20)

		for {
			select {
			case <-shutdown:
				log.Println("Stopping SQS receiver...")
				close(messageChannel)
				return
			default:
				messages, err := sqsProvider.ReceiveMessages(ctx, maxMessages, waitTimeSeconds)
				if err != nil {
					log.Printf("Error receiving messages: %v", err)
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
		}
	}()

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"service":    "sub-service",
			"processed":  atomic.LoadInt64(&processedCount),
			"duplicates": atomic.LoadInt64(&duplicateCount),
			"errors":     atomic.LoadInt64(&errorCount),
			"goroutines": runtime.NumGoroutine(),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		log.Printf("Server starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	shutdownWorkers()
	log.Println("Server exited")
}

func messageWorker(
	ctx context.Context,
	wg *sync.WaitGroup,
	workerID int,
	sqsProvider *providers.SQSProvider,
	redisProvider *providers.RedisProvider,
	messageChannel <-chan providers.SQSMessage,
	shutdown <-chan struct{},
	processedCount *int64,
	duplicateCount *int64,
	errorCount *int64,
) {
	defer wg.Done()

	log.Printf("Worker %d started", workerID)

	for {
		select {
		case <-shutdown:
			log.Printf("Worker %d shutting down", workerID)
			return
		case msg, ok := <-messageChannel:
			if !ok {
				log.Printf("Worker %d channel closed", workerID)
				return
			}

			if msg.ParsedMessage == nil {
				atomic.AddInt64(errorCount, 1)
				log.Printf("Worker %d: received message with nil parsed content", workerID)
				sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle)
				continue
			}

			message := msg.ParsedMessage

			isDuplicate, err := redisProvider.CheckDuplicate(ctx, message.MessageID, message.UserID)
			if err != nil {
				atomic.AddInt64(errorCount, 1)
				log.Printf("Worker %d: error checking duplicate for user %s: %v", workerID, message.UserID, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if isDuplicate {
				atomic.AddInt64(duplicateCount, 1)
				log.Printf("Worker %d: duplicate message for user %s, skipping", workerID, message.UserID)
				redisProvider.MarkTechnicalProcessed(ctx, message.MessageID)
				sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle)
				continue
			}

			log.Printf("Worker %d: processing user %s", workerID, message.UserID)
			log.Printf("Worker %d: message: %s", workerID, message.Message)

			if err := redisProvider.MarkProcessed(ctx, message.MessageID, message.UserID); err != nil {
				atomic.AddInt64(errorCount, 1)
				log.Printf("Worker %d: error marking processed for user %s: %v", workerID, message.UserID, err)
				continue
			}

			if err := sqsProvider.DeleteMessage(ctx, msg.ReceiptHandle); err != nil {
				atomic.AddInt64(errorCount, 1)
				log.Printf("Worker %d: error deleting message for user %s: %v", workerID, message.UserID, err)
				continue
			}

			atomic.AddInt64(processedCount, 1)
		}
	}
}
