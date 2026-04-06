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
	"pub-service/pkg/services"
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

	service := services.NewService(s3Provider, sqsProvider, workerCount)
	log.Println("Service initialized")

	handler := handlers.NewHandler(service)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.GET("/batch", handler.ProcessBatchHandler)
	router.POST("/upload-assets", handler.UploadAssetsHandler)

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
