package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client *redis.Client
}

func newRedisStore(client *redis.Client) *redisStore {
	return &redisStore{client: client}
}

// OTP operations

func (r *redisStore) storeOTPHash(ctx context.Context, email, hash string, ttl time.Duration) error {
	pipe := r.client.Pipeline()
	pipe.Set(ctx, otpCodeKey(email), hash, ttl)
	pipe.Del(ctx, otpAttemptsKey(email))
	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisStore) getOTPHash(ctx context.Context, email string) (string, error) {
	val, err := r.client.Get(ctx, otpCodeKey(email)).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *redisStore) deleteOTP(ctx context.Context, email string) error {
	return r.client.Del(ctx, otpCodeKey(email), otpAttemptsKey(email)).Err()
}

func (r *redisStore) incrOTPAttempts(ctx context.Context, email string, ttl time.Duration) (int64, error) {
	pipe := r.client.Pipeline()
	incrCmd := pipe.Incr(ctx, otpAttemptsKey(email))
	pipe.ExpireNX(ctx, otpAttemptsKey(email), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

func (r *redisStore) setCooldown(ctx context.Context, email string, ttl time.Duration) error {
	return r.client.Set(ctx, otpCooldownKey(email), "", ttl).Err()
}

func (r *redisStore) isOnCooldown(ctx context.Context, email string) (bool, error) {
	n, err := r.client.Exists(ctx, otpCooldownKey(email)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Refresh token operations

func (r *redisStore) storeRefreshToken(ctx context.Context, hash, userID string, ttl time.Duration) error {
	return r.client.Set(ctx, refreshTokenKey(hash), userID, ttl).Err()
}

func (r *redisStore) getRefreshToken(ctx context.Context, hash string) (string, error) {
	val, err := r.client.Get(ctx, refreshTokenKey(hash)).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *redisStore) deleteRefreshToken(ctx context.Context, hash string) error {
	return r.client.Del(ctx, refreshTokenKey(hash)).Err()
}

func otpCodeKey(email string) string {
	return fmt.Sprintf("otp:code:%s", email)
}

func otpAttemptsKey(email string) string {
	return fmt.Sprintf("otp:attempts:%s", email)
}

func otpCooldownKey(email string) string {
	return fmt.Sprintf("otp:cooldown:%s", email)
}

func refreshTokenKey(hash string) string {
	return fmt.Sprintf("refresh:token:%s", hash)
}
