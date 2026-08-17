package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	internalauth "github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/queue"
	"github.com/USSTM/cv-backend/internal/rbac"
)

type verifyOTPCookieResponse struct {
	api.VerifyOTP200JSONResponse
	cookies []http.Cookie
}

func (response verifyOTPCookieResponse) VisitVerifyOTPResponse(w http.ResponseWriter) error {
	setCookies(w, response.cookies)
	return response.VerifyOTP200JSONResponse.VisitVerifyOTPResponse(w)
}

type refreshTokenCookieResponse struct {
	api.RefreshToken200JSONResponse
	cookies []http.Cookie
}

func (response refreshTokenCookieResponse) VisitRefreshTokenResponse(w http.ResponseWriter) error {
	setCookies(w, response.cookies)
	return response.RefreshToken200JSONResponse.VisitRefreshTokenResponse(w)
}

type logoutCookieResponse struct {
	api.Logout200JSONResponse
	cookies []http.Cookie
}

func (response logoutCookieResponse) VisitLogoutResponse(w http.ResponseWriter) error {
	setCookies(w, response.cookies)
	return response.Logout200JSONResponse.VisitLogoutResponse(w)
}

func setCookies(w http.ResponseWriter, cookies []http.Cookie) {
	for _, cookie := range cookies {
		http.SetCookie(w, &cookie)
	}
}

func (s Server) RequestOTP(ctx context.Context, request api.RequestOTPRequestObject) (api.RequestOTPResponseObject, error) {
	if request.Body == nil {
		return api.RequestOTP400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
	}

	logger := middleware.GetLoggerFromContext(ctx)
	email := string(request.Body.Email)

	code, err := s.authService.RequestOTP(ctx, email)
	if err != nil {
		if errors.Is(err, internalauth.ErrUserNotFound) {
			return api.RequestOTP200JSONResponse{Message: "A login code has been sent if your email is registered."}, nil
		}
		if errors.Is(err, internalauth.ErrOTPCooldown) {
			logger.Warn("OTP request blocked by cooldown", "email", email)
			return api.RequestOTP429JSONResponse(ValidationErr("Please wait before requesting another code.", nil).Create()), nil
		}
		logger.Error("Failed to generate OTP", "email", email, "error", err)
		return api.RequestOTP500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	_, err = s.queue.Enqueue(queue.TypeEmailDelivery, queue.EmailDeliveryPayload{
		To:      email,
		Subject: "Your Campus Vault login code",
		Body:    fmt.Sprintf("Your one-time login code is: %s\n\nThis code expires in %d minutes.", code, int(s.authService.OTPExpiry().Minutes())),
	})
	if err != nil {
		logger.Error("Failed to enqueue OTP email", "email", email, "error", err)
		return api.RequestOTP500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	logger.Info("OTP requested", "email", email)
	return api.RequestOTP200JSONResponse{Message: "A login code has been sent if your email is registered."}, nil
}

func (s Server) VerifyOTP(ctx context.Context, request api.VerifyOTPRequestObject) (api.VerifyOTPResponseObject, error) {
	if request.Body == nil {
		return api.VerifyOTP400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
	}

	logger := middleware.GetLoggerFromContext(ctx)
	email := string(request.Body.Email)
	code := request.Body.Code

	accessToken, refreshToken, err := s.authService.VerifyOTP(ctx, email, code)
	if err != nil {
		if errors.Is(err, internalauth.ErrOTPInvalid) {
			logger.Warn("OTP verification failed: invalid code", "email", email)
			return api.VerifyOTP400JSONResponse(ValidationErr("Invalid or expired code.", nil).Create()), nil
		}
		if errors.Is(err, internalauth.ErrOTPMaxAttempts) {
			logger.Warn("OTP verification failed: max attempts exceeded", "email", email)
			return api.VerifyOTP400JSONResponse(ValidationErr("Invalid or expired code.", nil).Create()), nil
		}
		if errors.Is(err, internalauth.ErrUserNotFound) {
			logger.Warn("OTP verification failed: user deleted between OTP request and verify", "email", email)
			return api.VerifyOTP400JSONResponse(ValidationErr("Invalid or expired code.", nil).Create()), nil
		}
		logger.Error("Failed to verify OTP", "email", email, "error", err)
		return api.VerifyOTP500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	logger.Info("User authenticated via OTP", "email", email)
	return verifyOTPCookieResponse{
		VerifyOTP200JSONResponse: api.VerifyOTP200JSONResponse{
			Message: "Authenticated successfully.",
		},
		cookies: s.authCookies(accessToken, refreshToken),
	}, nil
}

func (s Server) RefreshToken(ctx context.Context, request api.RefreshTokenRequestObject) (api.RefreshTokenResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)
	refreshToken := refreshTokenFromRequest(ctx, request)
	if refreshToken == "" {
		return api.RefreshToken400JSONResponse(ValidationErr("Refresh token is required", nil).Create()), nil
	}

	accessToken, refreshToken, err := s.authService.Refresh(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, internalauth.ErrRefreshInvalid) {
			logger.Warn("Refresh token rejected: invalid or expired")
			return api.RefreshToken401JSONResponse(Unauthorized("Invalid or expired refresh token.").Create()), nil
		}
		logger.Error("Failed to refresh token", "error", err)
		return api.RefreshToken500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	return refreshTokenCookieResponse{
		RefreshToken200JSONResponse: api.RefreshToken200JSONResponse{
			Message: "Session refreshed successfully.",
		},
		cookies: s.authCookies(accessToken, refreshToken),
	}, nil
}

func (s Server) Logout(ctx context.Context, request api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	if request.Body == nil {
		return api.Logout400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
	}

	logger := middleware.GetLoggerFromContext(ctx)
	refreshToken := refreshTokenFromLogoutRequest(ctx, request)
	if refreshToken == "" {
		return api.Logout400JSONResponse(ValidationErr("Refresh token is required", nil).Create()), nil
	}

	if err := s.authService.Logout(ctx, refreshToken); err != nil {
		logger.Error("Failed to logout", "error", err)
		return api.Logout500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	return logoutCookieResponse{
		Logout200JSONResponse: api.Logout200JSONResponse{Message: "Logged out successfully."},
		cookies:               s.expiredAuthCookies(),
	}, nil
}

const (
	accessTokenCookieName  = "access_token"
	refreshTokenCookieName = "refresh_token"
)

func (s Server) authCookies(accessToken, refreshToken string) []http.Cookie {
	return []http.Cookie{
		s.authCookie(accessTokenCookieName, accessToken, s.cookies.AccessExpiry),
		s.authCookie(refreshTokenCookieName, refreshToken, s.cookies.RefreshExpiry),
	}
}

func (s Server) expiredAuthCookies() []http.Cookie {
	return []http.Cookie{
		s.authCookie(accessTokenCookieName, "", -time.Hour),
		s.authCookie(refreshTokenCookieName, "", -time.Hour),
	}
}

func (s Server) authCookie(name, value string, expiry time.Duration) http.Cookie {
	now := time.Now()
	return http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  now.Add(expiry),
		MaxAge:   int(expiry.Seconds()),
		HttpOnly: true,
		Secure:   s.cookies.Secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func refreshTokenFromRequest(ctx context.Context, request api.RefreshTokenRequestObject) string {
	if request.Body != nil && request.Body.RefreshToken != "" {
		return request.Body.RefreshToken
	}
	return middleware.GetRefreshTokenFromContext(ctx)
}

func refreshTokenFromLogoutRequest(ctx context.Context, request api.LogoutRequestObject) string {
	if request.Body != nil && request.Body.RefreshToken != "" {
		return request.Body.RefreshToken
	}
	return middleware.GetRefreshTokenFromContext(ctx)
}

func (s Server) PingProtected(ctx context.Context, request api.PingProtectedRequestObject) (api.PingProtectedResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := internalauth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.PingProtected401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ViewOwnData, nil)
	if err != nil {
		logger.Error("Error checking view_own_data permission",
			"user_id", user.ID,
			"permission", rbac.ViewOwnData,
			"error", err)
		return api.PingProtected500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.PingProtected401JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	return api.PingProtected200JSONResponse{
		Message:   "PONG! Hello " + user.Email,
		Timestamp: time.Now(),
	}, nil
}
