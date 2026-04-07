package providers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisProvider struct {
	client *redis.Client
}

func NewRedisProvider(ctx context.Context) (*RedisProvider, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "",
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &RedisProvider{
		client: client,
	}, nil
}

func (p *RedisProvider) CheckDuplicate(ctx context.Context, messageID string, userID string) (bool, error) {
	today := time.Now().Format("2006-01-02")
	processedKey := fmt.Sprintf("processed:%s", messageID)
	userCampaignKey := fmt.Sprintf("user:%s:campaign:%s", userID, today)

	pipe := p.client.Pipeline()
	cmd1 := pipe.Exists(ctx, processedKey)
	cmd2 := pipe.Exists(ctx, userCampaignKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check duplicates: %w", err)
	}

	exists1 := cmd1.Val() > 0
	exists2 := cmd2.Val() > 0

	return exists1 || exists2, nil
}

func (p *RedisProvider) MarkProcessed(ctx context.Context, messageID string, userID string) error {
	today := time.Now().Format("2006-01-02")
	processedKey := fmt.Sprintf("processed:%s", messageID)
	userCampaignKey := fmt.Sprintf("user:%s:campaign:%s", userID, today)

	pipe := p.client.Pipeline()
	pipe.Set(ctx, processedKey, "1", 24*time.Hour)
	pipe.Set(ctx, userCampaignKey, "1", 7*24*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark processed: %w", err)
	}

	return nil
}

func (p *RedisProvider) MarkTechnicalProcessed(ctx context.Context, messageID string) error {
	processedKey := fmt.Sprintf("processed:%s", messageID)

	if err := p.client.Set(ctx, processedKey, "1", 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to mark technical processed: %w", err)
	}

	return nil
}

func (p *RedisProvider) Exists(ctx context.Context, key string) (bool, error) {
	result, err := p.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check key existence: %w", err)
	}

	return result > 0, nil
}

func (p *RedisProvider) Get(ctx context.Context, key string) (string, error) {
	result, err := p.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("failed to get key: %w", err)
	}

	return result, nil
}

func (p *RedisProvider) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if err := p.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	return nil
}

func (p *RedisProvider) Delete(ctx context.Context, keys ...string) error {
	if err := p.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}

	return nil
}

func (p *RedisProvider) Close() error {
	return p.client.Close()
}

func (p *RedisProvider) GetClient() *redis.Client {
	return p.client
}

func (p *RedisProvider) AcquireProcessingLock(ctx context.Context, messageID string, workerID int, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("processing:%s", messageID)
	lockValue := fmt.Sprintf("%d:%d", workerID, time.Now().Unix())

	acquired, err := p.client.SetNX(ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return acquired, nil
}

func (p *RedisProvider) ReleaseProcessingLock(ctx context.Context, messageID string) error {
	lockKey := fmt.Sprintf("processing:%s", messageID)

	if err := p.client.Del(ctx, lockKey).Err(); err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	return nil
}

func (p *RedisProvider) CheckProcessed(ctx context.Context, messageID string) (bool, error) {
	processedKey := fmt.Sprintf("processed:%s", messageID)

	exists, err := p.client.Exists(ctx, processedKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check processed status: %w", err)
	}

	return exists > 0, nil
}
