package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSProvider struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSProvider(ctx context.Context) (*SQSProvider, error) {
	endpoint := os.Getenv("SQS_ENDPOINT")
	queueURL := os.Getenv("SQS_QUEUE_URL")
	region := os.Getenv("AWS_REGION")

	if endpoint == "" {
		return nil, fmt.Errorf("SQS_ENDPOINT is required")
	}
	if queueURL == "" {
		return nil, fmt.Errorf("SQS_QUEUE_URL is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &SQSProvider{
		client:   client,
		queueURL: queueURL,
	}, nil
}

type Message struct {
	MessageID string `json:"message_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
}

func (p *SQSProvider) ReceiveMessages(ctx context.Context, maxMessages int32, waitTimeSeconds int32) ([]SQSMessage, error) {
	input := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(p.queueURL),
		MaxNumberOfMessages: maxMessages,
		WaitTimeSeconds:     waitTimeSeconds,
		AttributeNames:      []types.QueueAttributeName{types.QueueAttributeNameAll},
	}

	result, err := p.client.ReceiveMessage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to receive messages from SQS: %w", err)
	}

	messages := make([]SQSMessage, 0, len(result.Messages))
	for _, msg := range result.Messages {
		sqsMsg := SQSMessage{
			MessageId:     msg.MessageId,
			ReceiptHandle: msg.ReceiptHandle,
			Body:          msg.Body,
		}

		if msg.Body != nil {
			var parsedMsg Message
			if err := json.Unmarshal([]byte(*msg.Body), &parsedMsg); err == nil {
				sqsMsg.ParsedMessage = &parsedMsg
			}
		}

		messages = append(messages, sqsMsg)
	}

	return messages, nil
}

func (p *SQSProvider) DeleteMessage(ctx context.Context, receiptHandle *string) error {
	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.queueURL),
		ReceiptHandle: receiptHandle,
	}

	_, err := p.client.DeleteMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete message from SQS: %w", err)
	}

	return nil
}

func (p *SQSProvider) DeleteMessageBatch(ctx context.Context, receiptHandles []*string) error {
	if len(receiptHandles) == 0 {
		return nil
	}

	batchSize := 10
	for i := 0; i < len(receiptHandles); i += batchSize {
		end := i + batchSize
		if end > len(receiptHandles) {
			end = len(receiptHandles)
		}

		entries := make([]types.DeleteMessageBatchRequestEntry, 0, end-i)
		for j := i; j < end; j++ {
			id := fmt.Sprintf("msg-%d", j)
			entry := types.DeleteMessageBatchRequestEntry{
				Id:            &id,
				ReceiptHandle: receiptHandles[j],
			}
			entries = append(entries, entry)
		}

		input := &sqs.DeleteMessageBatchInput{
			QueueUrl: aws.String(p.queueURL),
			Entries:  entries,
		}

		_, err := p.client.DeleteMessageBatch(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to delete batch messages from SQS: %w", err)
		}
	}

	return nil
}

func (p *SQSProvider) GetClient() *sqs.Client {
	return p.client
}

func (p *SQSProvider) GetQueueURL() string {
	return p.queueURL
}

type SQSMessage struct {
	MessageId     *string
	ReceiptHandle *string
	Body          *string
	ParsedMessage *Message
}
