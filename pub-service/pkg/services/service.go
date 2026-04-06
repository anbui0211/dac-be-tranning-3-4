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

	"pub-service/pkg/repository"
	"pub-service/providers"
)

type Service interface {
	ProcessBatch(ctx context.Context) error
	UploadAssets(ctx context.Context) (map[string]string, error)
	CleanupAndReset(ctx context.Context) error
}

type ServiceImpl struct {
	s3Provider   *providers.S3Provider
	sqsProvider  *providers.SQSProvider
	scheduleRepo repository.MessageScheduleRepository
	workerCount  int
}

func NewService(s3Provider *providers.S3Provider, sqsProvider *providers.SQSProvider, scheduleRepo repository.MessageScheduleRepository, workerCount int) Service {
	return &ServiceImpl{
		s3Provider:   s3Provider,
		sqsProvider:  sqsProvider,
		scheduleRepo: scheduleRepo,
		workerCount:  workerCount,
	}
}

func (s *ServiceImpl) ProcessBatch(ctx context.Context) error {
	log.Printf("Starting batch processing for message schedules")

	schedules, err := s.scheduleRepo.GetAllWithContents(ctx)
	if err != nil {
		return fmt.Errorf("failed to get message schedules: %w", err)
	}

	messageChannel := make(chan *providers.Message, 100)
	messageCount := make(chan int, 1)
	batchErrorCount := make(chan int, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go s.batchWorker(ctx, messageChannel, messageCount, batchErrorCount, &wg)

	startTime := time.Now()
	msgCounter := 1
	totalRows := 0
	errorCount := 0

	for _, schedule := range schedules {
		if schedule.Content == nil {
			errorCount++
			continue
		}

		result, err := s.s3Provider.DownloadFileStream(ctx, schedule.Segment)
		if err != nil {
			errorCount++
			continue
		}
		defer result.Body.Close()

		reader := csv.NewReader(result.Body)

		_, err = reader.Read()
		if err != nil {
			errorCount++
			continue
		}

		for {
			row, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				errorCount++
				continue
			}

			if len(row) < 1 {
				continue
			}

			messageID := fmt.Sprintf("msg_%02d", msgCounter)
			msg := &providers.Message{
				MessageID: messageID,
				UserID:    row[0],
				Message:   schedule.Content.Content,
			}

			messageChannel <- msg
			msgCounter++
			totalRows++
		}
	}

	close(messageChannel)
	wg.Wait()

	totalMessages := <-messageCount
	totalBatchErrors := <-batchErrorCount

	duration := time.Since(startTime)
	totalErrors := errorCount + totalBatchErrors
	log.Printf("Batch processing completed - Schedules: %d, Rows: %d, Messages: %d, Errors: %d (download: %d, batch: %d), Duration: %v",
		len(schedules), totalRows, totalMessages, totalErrors, errorCount, totalBatchErrors, duration)

	return nil
}

func (s *ServiceImpl) batchWorker(ctx context.Context, messageChannel <-chan *providers.Message, messageCount chan<- int, batchErrorCount chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()

	batchSize := 10
	batch := make([]*providers.Message, 0, batchSize)
	errorCount := 0

	for msg := range messageChannel {
		batch = append(batch, msg)

		if len(batch) >= batchSize {
			if err := s.sqsProvider.BatchSendMessage(ctx, batch); err != nil {
				errorCount++
			} else {
				messageCount <- len(batch)
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := s.sqsProvider.BatchSendMessage(ctx, batch); err != nil {
			errorCount++
		} else {
			messageCount <- len(batch)
		}
	}

	close(messageCount)
	batchErrorCount <- errorCount
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
			failCount++
			continue
		}

		if err := s.s3Provider.UploadFile(ctx, fileName, data); err != nil {
			failCount++
			continue
		}

		results[fileName] = "success"
		successCount++
	}

	if failCount > 0 {
		log.Printf("Asset upload completed: %d succeeded, %d failed", successCount, failCount)
	} else {
		log.Printf("Asset upload completed: %d files uploaded successfully", successCount)
	}

	return results, nil
}

func (s *ServiceImpl) CleanupAndReset(ctx context.Context) error {
	log.Println("Starting cleanup and reset...")

	if err := s.sqsProvider.PurgeQueue(ctx); err != nil {
		return fmt.Errorf("failed to purge SQS queue: %w", err)
	}

	if err := s.s3Provider.EmptyBucket(ctx); err != nil {
		return fmt.Errorf("failed to empty S3 bucket: %w", err)
	}

	assetsDir := "assets"
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}

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
	}

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
	}

	log.Printf("Cleanup and reset completed - Generated and uploaded %d CSV files to S3", len(csvFiles))
	return nil
}
