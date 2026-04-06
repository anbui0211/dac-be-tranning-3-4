package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"pub-service/providers"

	"github.com/google/uuid"
)

type Service interface {
	ProcessBatch(ctx context.Context) error
	UploadAssets(ctx context.Context) (map[string]string, error)
	CleanupAndReset(ctx context.Context) error
}

type ServiceImpl struct {
	s3Provider  *providers.S3Provider
	sqsProvider *providers.SQSProvider
	workerCount int
}

func NewService(s3Provider *providers.S3Provider, sqsProvider *providers.SQSProvider, workerCount int) Service {
	return &ServiceImpl{
		s3Provider:  s3Provider,
		sqsProvider: sqsProvider,
		workerCount: workerCount,
	}
}

func (s *ServiceImpl) ProcessBatch(ctx context.Context) error {
	csvKey := "segment_01"
	log.Printf("Starting batch processing for file: %s", csvKey)

	startTime := time.Now()
	totalRows := 0
	totalMessages := 0
	errorCount := 0

	result, err := s.s3Provider.DownloadFileStream(ctx, csvKey)
	if err != nil {
		return fmt.Errorf("failed to download file from S3: %w", err)
	}
	defer result.Body.Close()

	reader := csv.NewReader(result.Body)

	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV headers: %w", err)
	}

	log.Printf("CSV Headers: %v", headers)

	rowChannel := make(chan []string, 1000)
	messageChannel := make(chan *providers.Message, 100)
	messageCount := make(chan int, s.workerCount)

	var wg sync.WaitGroup

	for i := 0; i < s.workerCount; i++ {
		wg.Add(1)
		go s.rowWorker(ctx, rowChannel, messageChannel, &wg)
	}

	batchWG := sync.WaitGroup{}
	batchWG.Add(1)
	go s.batchWorker(ctx, messageChannel, messageCount, &batchWG)

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errorCount++
			log.Printf("Error reading CSV row: %v", err)
			continue
		}

		if len(row) < 1 {
			log.Printf("Skipping invalid row: %v", row)
			continue
		}

		rowChannel <- row
		totalRows++
	}

	close(rowChannel)
	wg.Wait()

	close(messageChannel)

	for count := range messageCount {
		totalMessages += count
	}

	batchWG.Wait()

	duration := time.Since(startTime)
	log.Printf("Batch processing completed")
	log.Printf("Total rows: %d", totalRows)
	log.Printf("Total messages sent: %d", totalMessages)
	log.Printf("Errors: %d", errorCount)
	log.Printf("Duration: %v", duration)

	return nil
}

func (s *ServiceImpl) rowWorker(ctx context.Context, rowChannel <-chan []string, messageChannel chan<- *providers.Message, wg *sync.WaitGroup) {
	defer wg.Done()

	for row := range rowChannel {
		messageID := uuid.New().String()

		msg := &providers.Message{
			MessageID: messageID,
			UserID:    row[0],
			Message:   row[0],
		}

		messageChannel <- msg
	}
}

func (s *ServiceImpl) batchWorker(ctx context.Context, messageChannel <-chan *providers.Message, messageCount chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()

	batchSize := 10
	batch := make([]*providers.Message, 0, batchSize)

	for msg := range messageChannel {
		batch = append(batch, msg)

		if len(batch) >= batchSize {
			if err := s.sqsProvider.BatchSendMessage(ctx, batch); err != nil {
				log.Printf("Error sending batch: %v", err)
			} else {
				messageCount <- len(batch)
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := s.sqsProvider.BatchSendMessage(ctx, batch); err != nil {
			log.Printf("Error sending final batch: %v", err)
		} else {
			messageCount <- len(batch)
		}
	}

	close(messageCount)
}

func (s *ServiceImpl) UploadAssets(ctx context.Context) (map[string]string, error) {
	assetsDir := "assets"
	files, err := os.ReadDir(assetsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read assets directory: %w", err)
	}

	results := make(map[string]string)
	successCount := 0
	failCount := 0

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		filePath := filepath.Join(assetsDir, fileName)

		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("Error reading file %s: %v", fileName, err)
			failCount++
			continue
		}

		if err := s.s3Provider.UploadFile(ctx, fileName, data); err != nil {
			log.Printf("Error uploading file %s: %v", fileName, err)
			failCount++
			continue
		}

		results[fileName] = "success"
		successCount++
		log.Printf("Uploaded: %s", fileName)
	}

	log.Printf("Asset upload completed: %d succeeded, %d failed", successCount, failCount)

	return results, nil
}

func (s *ServiceImpl) CleanupAndReset(ctx context.Context) error {
	log.Println("Starting cleanup and reset...")

	log.Println("Purging SQS queue...")
	if err := s.sqsProvider.PurgeQueue(ctx); err != nil {
		return fmt.Errorf("failed to purge SQS queue: %w", err)
	}
	log.Println("SQS queue purged successfully")

	log.Println("Deleting all objects from S3 bucket...")
	if err := s.s3Provider.EmptyBucket(ctx); err != nil {
		return fmt.Errorf("failed to empty S3 bucket: %w", err)
	}
	log.Println("S3 bucket emptied successfully")

	assetsDir := "assets"
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}
	log.Println("Assets directory ready")

	csvFiles := []string{"segment_01", "segment_02"}
	for _, fileName := range csvFiles {
		filePath := filepath.Join(assetsDir, fileName+".csv")

		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create CSV file %s: %w", fileName, err)
		}

		writer := csv.NewWriter(file)
		defer writer.Flush()

		if err := writer.Write([]string{"User"}); err != nil {
			file.Close()
			return fmt.Errorf("failed to write header to %s: %w", fileName, err)
		}

		for i := 1; i <= 10; i++ {
			userID := fmt.Sprintf("user_%02d", i)
			if err := writer.Write([]string{userID}); err != nil {
				file.Close()
				return fmt.Errorf("failed to write row to %s: %w", fileName, err)
			}
		}

		file.Close()
		log.Printf("Generated CSV file: %s", filePath)
	}

	log.Println("Uploading CSV files to S3...")
	for _, fileName := range csvFiles {
		filePath := filepath.Join(assetsDir, fileName+".csv")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read CSV file %s: %w", fileName, err)
		}

		s3Key := fileName
		if err := s.s3Provider.UploadFile(ctx, s3Key, data); err != nil {
			return fmt.Errorf("failed to upload %s to S3: %w", fileName, err)
		}
		log.Printf("Uploaded %s to S3 as %s", filePath, s3Key)
	}

	log.Println("Cleanup and reset completed successfully")
	return nil
}
