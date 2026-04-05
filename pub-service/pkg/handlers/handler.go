package handlers

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"pub-service/providers"

	"github.com/google/uuid"
)

type Handler struct {
	s3Provider  *providers.S3Provider
	sqsProvider *providers.SQSProvider
	workerCount int
}

func NewHandler(s3Provider *providers.S3Provider, sqsProvider *providers.SQSProvider, workerCount int) *Handler {
	return &Handler{
		s3Provider:  s3Provider,
		sqsProvider: sqsProvider,
		workerCount: workerCount,
	}
}

func (h *Handler) ProcessBatch(ctx context.Context, csvKey string) error {
	log.Printf("Starting batch processing for file: %s", csvKey)

	startTime := time.Now()
	totalRows := 0
	totalMessages := 0
	errorCount := 0

	result, err := h.s3Provider.DownloadFileStream(ctx, csvKey)
	if err != nil {
		return fmt.Errorf("failed to download file from S3: %w", err)
	}
	defer result.Body.Close()

	gzipReader, err := gzip.NewReader(result.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	reader := csv.NewReader(gzipReader)

	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV headers: %w", err)
	}

	log.Printf("CSV Headers: %v", headers)

	rowChannel := make(chan []string, 1000)
	messageChannel := make(chan *providers.Message, 100)
	messageCount := make(chan int, h.workerCount)

	var wg sync.WaitGroup

	for i := 0; i < h.workerCount; i++ {
		wg.Add(1)
		go h.rowWorker(ctx, rowChannel, messageChannel, &wg)
	}

	batchWG := sync.WaitGroup{}
	batchWG.Add(1)
	go h.batchWorker(ctx, messageChannel, messageCount, &batchWG)

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

func (h *Handler) rowWorker(ctx context.Context, rowChannel <-chan []string, messageChannel chan<- *providers.Message, wg *sync.WaitGroup) {
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

func (h *Handler) batchWorker(ctx context.Context, messageChannel <-chan *providers.Message, messageCount chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()

	batchSize := 10
	batch := make([]*providers.Message, 0, batchSize)

	for msg := range messageChannel {
		batch = append(batch, msg)

		if len(batch) >= batchSize {
			if err := h.sqsProvider.BatchSendMessage(ctx, batch); err != nil {
				log.Printf("Error sending batch: %v", err)
			} else {
				messageCount <- len(batch)
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := h.sqsProvider.BatchSendMessage(ctx, batch); err != nil {
			log.Printf("Error sending final batch: %v", err)
		} else {
			messageCount <- len(batch)
		}
	}

	close(messageCount)
}
