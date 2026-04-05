package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pub-service/pkg/handlers"
	"pub-service/providers"

	"github.com/gin-gonic/gin"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Pub-Service...")

	ctx := context.Background()

	s3Provider, err := providers.NewS3Provider(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize S3 provider: %v", err)
	}
	log.Println("S3 provider initialized")

	sqsProvider, err := providers.NewSQSProvider(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize SQS provider: %v", err)
	}
	log.Println("SQS provider initialized")

	workerCount := 20
	if wc := os.Getenv("SQS_WORKER_COUNT"); wc != "" {
		var parsed int
		if _, err := fmt.Sscanf(wc, "%d", &parsed); err == nil && parsed > 0 {
			workerCount = parsed
		}
	}

	handler := handlers.NewHandler(s3Provider, sqsProvider, workerCount)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pub-service"})
	})

	router.POST("/batch", func(c *gin.Context) {
		var request struct {
			CSVFile string `json:"csv_file" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		log.Printf("Processing batch for file: %s", request.CSVFile)

		if err := handler.ProcessBatch(ctx, request.CSVFile); err != nil {
			log.Printf("Error processing batch: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Batch processing started"})
	})

	router.GET("/files", func(c *gin.Context) {
		files, err := s3Provider.ListFiles(ctx)
		if err != nil {
			log.Printf("Error listing files: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"files": files})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
