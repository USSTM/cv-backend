package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/google/uuid"
)

type contextKey string

const (
	UserIDKey     contextKey = "user_id"
	UserClaimsKey contextKey = "user_claims"
)

type AuthenticatedUser struct {
	ID          uuid.UUID
	Email       string
	Permissions []db.GetUserPermissionsRow
	Roles       []db.GetUserRolesRow
}

type Authenticator struct {
	jwtService *JWTService
	queries    *db.Queries
}

func NewAuthenticator(jwtService *JWTService, queries *db.Queries) *Authenticator {
	return &Authenticator{
		jwtService: jwtService,
		queries:    queries,
	}
}

func (a *Authenticator) Authenticate(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
	if input.SecuritySchemeName != "BearerAuth" {
		return fmt.Errorf("authentication service missing")
	}

	authHeader := input.RequestValidationInput.Request.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("authorization header missing")
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return fmt.Errorf("invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	claims, err := a.jwtService.ValidateToken(ctx, token)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	user, err := a.queries.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	permissions, err := a.queries.GetUserPermissions(ctx, &claims.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user permissions: %w", err)
	}

	roles, err := a.queries.GetUserRoles(ctx, &claims.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user roles: %w", err)
	}

	authenticatedUser := &AuthenticatedUser{
		ID:          claims.UserID,
		Email:       user.Email,
		Permissions: permissions,
		Roles:       roles,
	}

	*input.RequestValidationInput.Request = *input.RequestValidationInput.Request.WithContext(
		context.WithValue(ctx, UserIDKey, claims.UserID),
	)
	*input.RequestValidationInput.Request = *input.RequestValidationInput.Request.WithContext(
		context.WithValue(input.RequestValidationInput.Request.Context(), UserClaimsKey, authenticatedUser),
	)

	return nil
}

func (a *Authenticator) CheckPermission(ctx context.Context, userID uuid.UUID, permission string, scopeID *uuid.UUID) (bool, error) {
	hasPermission, err := a.queries.CheckUserPermission(ctx, db.CheckUserPermissionParams{
		UserID:  &userID,
		Name:    permission,
		ScopeID: scopeID,
	})
	if err != nil {
		return false, err
	}
	return hasPermission, nil
}

func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(UserIDKey).(uuid.UUID)
	return userID, ok
}

func GetAuthenticatedUser(ctx context.Context) (*AuthenticatedUser, bool) {
	user, ok := ctx.Value(UserClaimsKey).(*AuthenticatedUser)
	return user, ok
}
