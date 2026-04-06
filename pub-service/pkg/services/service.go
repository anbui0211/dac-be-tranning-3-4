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

	model "pub-service/pkg/model"
	"pub-service/providers"

	"github.com/google/uuid"
)

type Service interface {
	ProcessBatch(ctx context.Context) error
	UploadFile(ctx context.Context, fileName string) error
	ListFiles(ctx context.Context) ([]string, error)
	UploadMultiple(ctx context.Context, fileNames []string, concurrency int) (*model.UploadResponse, error)
	UploadAll(ctx context.Context, pattern string, concurrency int) (*model.UploadResponse, error)
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

		if len(row) < 5 {
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

func (s *ServiceImpl) UploadFile(ctx context.Context, fileName string) error {
	log.Printf("Uploading file: %s", fileName)

	data, err := os.ReadFile(fmt.Sprintf("assets/%s", fileName))
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := s.s3Provider.UploadFile(ctx, fileName, data); err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

func (s *ServiceImpl) ListFiles(ctx context.Context) ([]string, error) {
	return s.s3Provider.ListFiles(ctx)
}

func (s *ServiceImpl) rowWorker(ctx context.Context, rowChannel <-chan []string, messageChannel chan<- *providers.Message, wg *sync.WaitGroup) {
	defer wg.Done()

	for row := range rowChannel {
		messageID := uuid.New().String()

		msg := &providers.Message{
			MessageID: messageID,
			UserID:    row[0],
			Message:   row[4],
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

func (s *ServiceImpl) UploadMultiple(ctx context.Context, fileNames []string, concurrency int) (*model.UploadResponse, error) {
	startTime := time.Now()

	if concurrency <= 0 {
		concurrency = 5
	}
	if len(fileNames) == 0 {
		return nil, fmt.Errorf("no files specified")
	}

	fileChannel := make(chan string, len(fileNames))
	resultChannel := make(chan model.UploadResult, len(fileNames))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go s.uploadWorker(ctx, fileChannel, resultChannel, &wg)
	}

	for _, fileName := range fileNames {
		fileChannel <- fileName
	}
	close(fileChannel)

	wg.Wait()
	close(resultChannel)

	results := make([]model.UploadResult, 0, len(fileNames))
	uploaded := 0
	failed := 0

	for result := range resultChannel {
		results = append(results, result)
		if result.Status == "success" {
			uploaded++
		} else {
			failed++
		}
	}

	duration := time.Since(startTime)
	response := &model.UploadResponse{
		Status:     "completed",
		TotalFiles: len(fileNames),
		Uploaded:   uploaded,
		Failed:     failed,
		Results:    results,
		Duration:   duration.String(),
	}

	return response, nil
}

func (s *ServiceImpl) UploadAll(ctx context.Context, pattern string, concurrency int) (*model.UploadResponse, error) {
	startTime := time.Now()

	if concurrency <= 0 {
		concurrency = 5
	}
	if pattern == "" {
		pattern = "*"
	}

	fileNames, err := s.listFilesInAssets(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	if len(fileNames) == 0 {
		return &model.UploadResponse{
			Status:     "completed",
			TotalFiles: 0,
			Uploaded:   0,
			Failed:     0,
			Results:    []model.UploadResult{},
			Duration:   time.Since(startTime).String(),
		}, nil
	}

	return s.UploadMultiple(ctx, fileNames, concurrency)
}

func (s *ServiceImpl) listFilesInAssets(pattern string) ([]string, error) {
	files, err := filepath.Glob(fmt.Sprintf("assets/%s", pattern))
	if err != nil {
		return nil, err
	}

	fileNames := make([]string, 0, len(files))
	for _, file := range files {
		fileName := filepath.Base(file)
		fileNames = append(fileNames, fileName)
	}

	return fileNames, nil
}

func (s *ServiceImpl) uploadWorker(ctx context.Context, fileChannel <-chan string, resultChannel chan<- model.UploadResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for fileName := range fileChannel {
		result := model.UploadResult{
			File: fileName,
		}

		data, err := os.ReadFile(fmt.Sprintf("assets/%s", fileName))
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			resultChannel <- result
			continue
		}

		if err := s.s3Provider.UploadFile(ctx, fileName, data); err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			resultChannel <- result
			continue
		}

		result.Status = "success"
		result.Key = fileName
		resultChannel <- result
	}
}
