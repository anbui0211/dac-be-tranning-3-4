package providers

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Provider struct {
	client *s3.Client
	bucket string
}

func NewS3Provider(ctx context.Context) (*S3Provider, error) {
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretKey := os.Getenv("S3_SECRET_KEY")
	bucket := os.Getenv("S3_BUCKET")
	region := os.Getenv("AWS_REGION")

	if endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT is required")
	}
	if accessKey == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY is required")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("S3_SECRET_KEY is required")
	}
	if bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &S3Provider{
		client: client,
		bucket: bucket,
	}, nil
}

func (p *S3Provider) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}

	result, err := p.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}
	defer result.Body.Close()

	data := make([]byte, 0)
	buf := make([]byte, 1024*1024)

	for {
		n, err := result.Body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	return data, nil
}

func (p *S3Provider) DownloadFileStream(ctx context.Context, key string) (*s3.GetObjectOutput, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}

	result, err := p.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	return result, nil
}

func (p *S3Provider) GetClient() *s3.Client {
	return p.client
}

func (p *S3Provider) GetBucket() string {
	return p.bucket
}

func (p *S3Provider) DeleteAllObjects(ctx context.Context) error {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(p.bucket),
	}

	var deletedCount int

	for {
		result, err := p.client.ListObjectsV2(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		if len(result.Contents) == 0 {
			break
		}

		for _, obj := range result.Contents {
			_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(p.bucket),
				Key:    obj.Key,
			})
			if err != nil {
				return fmt.Errorf("failed to delete object %s: %w", *obj.Key, err)
			}
			deletedCount++
		}

		if *result.IsTruncated {
			input.ContinuationToken = result.NextContinuationToken
		} else {
			break
		}
	}

	return nil
}

func (p *S3Provider) EmptyBucket(ctx context.Context) error {
	err := p.DeleteAllObjects(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (p *S3Provider) UploadFile(ctx context.Context, key string, data []byte) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}

	_, err := p.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return nil
}
