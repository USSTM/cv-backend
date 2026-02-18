package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/logging"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrOTPCooldown    = errors.New("please wait before requesting another OTP")
	ErrOTPInvalid     = errors.New("invalid or expired OTP")
	ErrOTPMaxAttempts = errors.New("maximum OTP attempts exceeded")
	ErrRefreshInvalid = errors.New("invalid or expired refresh token")
	ErrUserNotFound   = errors.New("user not found")
)

// AuthService handles passwordless OTP authentication and rotating refresh tokens.
type AuthService struct {
	store          *redisStore
	jwt            *JWTService
	db             *db.Queries
	otpExpiry      time.Duration
	otpCooldown    time.Duration
	otpMaxAttempts int
	refreshExpiry  time.Duration
}

func NewAuthService(redisClient *redis.Client, jwtSvc *JWTService, queries *db.Queries, cfg config.AuthConfig) *AuthService {
	return &AuthService{
		store:          newRedisStore(redisClient),
		jwt:            jwtSvc,
		db:             queries,
		otpExpiry:      cfg.OTPExpiry,
		otpCooldown:    cfg.OTPCooldown,
		otpMaxAttempts: cfg.OTPMaxAttempts,
		refreshExpiry:  cfg.RefreshExpiry,
	}
}

// generates 6-digit OTP and return the plaintext code
func (s *AuthService) RequestOTP(ctx context.Context, email string) (string, error) {
	if _, err := s.db.GetUserByEmail(ctx, email); err != nil {
		return "", ErrUserNotFound
	}

	on, err := s.store.isOnCooldown(ctx, email)
	if err != nil {
		return "", fmt.Errorf("checking OTP cooldown: %w", err)
	}
	if on {
		return "", ErrOTPCooldown
	}

	code, err := generateOTPCode()
	if err != nil {
		return "", fmt.Errorf("generating OTP: %w", err)
	}

	hash := hashString(code)

	if err := s.store.storeOTPHash(ctx, email, hash, s.otpExpiry); err != nil {
		return "", fmt.Errorf("storing OTP: %w", err)
	}

	if err := s.store.setCooldown(ctx, email, s.otpCooldown); err != nil {
		return "", fmt.Errorf("setting OTP cooldown: %w", err)
	}

	return code, nil
}

// checks the OTP code and returns a new access + refresh token pair.
func (s *AuthService) VerifyOTP(ctx context.Context, email, code string) (accessToken, refreshToken string, err error) {
	storedHash, err := s.store.getOTPHash(ctx, email)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", "", ErrOTPInvalid
		}
		return "", "", fmt.Errorf("retrieving OTP hash: %w", err)
	}

	attempts, err := s.store.incrOTPAttempts(ctx, email, s.otpExpiry)
	if err != nil {
		return "", "", fmt.Errorf("incrementing OTP attempts: %w", err)
	}

	if attempts > int64(s.otpMaxAttempts) {
		_ = s.store.deleteOTP(ctx, email)
		return "", "", ErrOTPMaxAttempts
	}

	if hashString(code) != storedHash {
		if attempts >= int64(s.otpMaxAttempts) {
			_ = s.store.deleteOTP(ctx, email)
			return "", "", ErrOTPMaxAttempts
		}
		return "", "", ErrOTPInvalid
	}

	// remove otp after verifying
	if err := s.store.deleteOTP(ctx, email); err != nil {
		return "", "", fmt.Errorf("deleting OTP: %w", err)
	}

	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", ErrUserNotFound
	}

	return s.issueTokenPair(ctx, user.ID)
}

// rotates refresh token and returns new pair
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (newAccess, newRefresh string, err error) {
	hash := hashString(refreshToken)

	userIDStr, err := s.store.getRefreshToken(ctx, hash)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", "", ErrRefreshInvalid
		}
		return "", "", fmt.Errorf("retrieving refresh token: %w", err)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return "", "", fmt.Errorf("invalid user ID in refresh token: %w", err)
	}

	if err := s.store.deleteRefreshToken(ctx, hash); err != nil {
		return "", "", fmt.Errorf("deleting refresh token: %w", err)
	}

	newAccess, newRefresh, err = s.issueTokenPair(ctx, userID)
	if err != nil {
		return "", "", err
	}

	logging.Info("refresh token rotated", "user_id", userID)
	return newAccess, newRefresh, nil
}

// logs out user
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	hash := hashString(refreshToken)

	userIDStr, err := s.store.getRefreshToken(ctx, hash)
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("looking up refresh token: %w", err)
	}

	if err := s.store.deleteRefreshToken(ctx, hash); err != nil {
		return fmt.Errorf("deleting refresh token: %w", err)
	}

	if userIDStr != "" {
		logging.Info("user logged out", "user_id", userIDStr)
	}
	return nil
}

// generates a JWT access token and a random refresh token
func (s *AuthService) issueTokenPair(ctx context.Context, userID uuid.UUID) (accessToken, refreshToken string, err error) {
	accessToken, err = s.jwt.GenerateToken(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("generating access token: %w", err)
	}

	rawRefresh, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("generating refresh token: %w", err)
	}

	hash := hashString(rawRefresh)
	if err := s.store.storeRefreshToken(ctx, hash, userID.String(), s.refreshExpiry); err != nil {
		return "", "", fmt.Errorf("storing refresh token: %w", err)
	}

	return accessToken, rawRefresh, nil
}

// returns random 6-digit string
func generateOTPCode() (string, error) {
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// returns 32 random bytes as a hex string (64 chars).
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
