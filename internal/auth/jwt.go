package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type JWTService struct {
	signingKey jwk.Key
	issuer     string
	expiry     time.Duration
}

type TokenClaims struct {
	UserID uuid.UUID `json:"user_id"`
}

func NewJWTService(signingKey []byte, issuer string, expiry time.Duration) (*JWTService, error) {
	key, err := jwk.FromRaw(signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWK: %w", err)
	}

	if err := key.Set(jwk.AlgorithmKey, jwa.HS256); err != nil {
		return nil, fmt.Errorf("failed to set algorithm: %w", err)
	}

	return &JWTService{
		signingKey: key,
		issuer:     issuer,
		expiry:     expiry,
	}, nil
}

func (s *JWTService) GenerateToken(ctx context.Context, userID uuid.UUID) (string, error) {
	now := time.Now()
	
	token, err := jwt.NewBuilder().
		Issuer(s.issuer).
		Subject(userID.String()).
		IssuedAt(now).
		Expiration(now.Add(s.expiry)).
		Claim("user_id", userID.String()).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build token: %w", err)
	}

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.HS256, s.signingKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return string(signed), nil
}

func (s *JWTService) ValidateToken(ctx context.Context, tokenString string) (*TokenClaims, error) {
	parsedToken, err := jwt.Parse([]byte(tokenString), jwt.WithKey(jwa.HS256, s.signingKey), jwt.WithIssuer(s.issuer))
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if err := jwt.Validate(parsedToken); err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	userIDStr, ok := parsedToken.Get("user_id")
	if !ok {
		return nil, fmt.Errorf("user_id claim not found")
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return nil, fmt.Errorf("invalid user_id format: %w", err)
	}

	return &TokenClaims{
		UserID: userID,
	}, nil
}