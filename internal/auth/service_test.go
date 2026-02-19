package auth_test

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	sharedQueue *testutil.TestQueue
	sharedDB    *testutil.TestDatabase
)

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(0)
	}

	t := &testing.T{}
	sharedQueue = testutil.NewTestQueue(t)
	sharedDB = testutil.NewTestDatabase(t)
	sharedDB.RunMigrations(t)

	code := m.Run()

	if sharedDB.Pool() != nil {
		sharedDB.Pool().Close()
	}
	sharedQueue.Close()

	os.Exit(code)
}

func newTestAuthService(t *testing.T) *auth.AuthService {
	t.Helper()
	jwtSvc, err := auth.NewJWTService([]byte("test-signing-key"), "test-issuer", 15*time.Minute)
	require.NoError(t, err)

	return auth.NewAuthService(sharedQueue.Redis, jwtSvc, sharedDB.Queries(), config.AuthConfig{
		OTPExpiry:      5 * time.Minute,
		OTPCooldown:    60 * time.Second,
		OTPMaxAttempts: 3,
		RefreshExpiry:  7 * 24 * time.Hour,
	})
}

func TestAuthService_RequestOTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()

	t.Run("returns OTP code", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("otp-code@example.com").Create()
		code, err := svc.RequestOTP(ctx, user.Email)

		require.NoError(t, err)
		assert.Len(t, code, 6)
		assert.Regexp(t, `^\d{6}$`, code)
	})

	t.Run("unknown email returns ErrUserNotFound", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		svc := newTestAuthService(t)

		_, err := svc.RequestOTP(ctx, "ghost@example.com")
		assert.ErrorIs(t, err, auth.ErrUserNotFound)
	})

	t.Run("cooldown blocks second request", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("otp-cooldown@example.com").Create()
		_, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)

		_, err = svc.RequestOTP(ctx, user.Email)
		assert.ErrorIs(t, err, auth.ErrOTPCooldown)
	})

	t.Run("different emails are independent", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		userA := sharedDB.NewUser(t).WithEmail("otp-a@example.com").Create()
		userB := sharedDB.NewUser(t).WithEmail("otp-b@example.com").Create()

		_, err := svc.RequestOTP(ctx, userA.Email)
		require.NoError(t, err)

		_, err = svc.RequestOTP(ctx, userB.Email)
		require.NoError(t, err)
	})
}

func TestAuthService_VerifyOTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()

	t.Run("valid code returns token pair", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("verify@example.com").Create()

		code, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)

		access, refresh, err := svc.VerifyOTP(ctx, user.Email, code)

		require.NoError(t, err)
		assert.NotEmpty(t, access)
		assert.NotEmpty(t, refresh)
		assert.Len(t, refresh, 64) // 32 bytes as hex
	})

	t.Run("wrong code returns ErrOTPInvalid", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("wrong@example.com").Create()

		_, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)

		_, _, err = svc.VerifyOTP(ctx, user.Email, "000000")
		assert.ErrorIs(t, err, auth.ErrOTPInvalid)
	})

	t.Run("max attempts deletes OTP", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("attempts@example.com").Create()

		_, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)

		// 3 wrong attempts (OTP_MAX_ATTEMPTS=3 means 3 allowed failures)
		for i := 0; i < 3; i++ {
			_, _, err = svc.VerifyOTP(ctx, user.Email, "000000")
			assert.ErrorIs(t, err, auth.ErrOTPInvalid)
		}

		// 4th wrong attempt triggers lockout
		_, _, err = svc.VerifyOTP(ctx, user.Email, "000000")
		assert.ErrorIs(t, err, auth.ErrOTPMaxAttempts)

		// OTP is deleted; further attempts return ErrOTPInvalid
		_, _, err = svc.VerifyOTP(ctx, user.Email, "000000")
		assert.ErrorIs(t, err, auth.ErrOTPInvalid)
	})

	t.Run("email normalization routes to same OTP", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("normalize@example.com").Create()

		code, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)

		// verify with mixed-case email should succeed
		access, refresh, err := svc.VerifyOTP(ctx, "Normalize@Example.COM", code)
		require.NoError(t, err)
		assert.NotEmpty(t, access)
		assert.NotEmpty(t, refresh)
	})

	t.Run("code can only be used once", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("once@example.com").Create()

		code, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)

		_, _, err = svc.VerifyOTP(ctx, user.Email, code)
		require.NoError(t, err)

		// second use of same code
		_, _, err = svc.VerifyOTP(ctx, user.Email, code)
		assert.ErrorIs(t, err, auth.ErrOTPInvalid)
	})

	t.Run("no OTP requested returns ErrOTPInvalid", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("no-otp@example.com").Create()

		_, _, err := svc.VerifyOTP(ctx, user.Email, "123456")
		assert.ErrorIs(t, err, auth.ErrOTPInvalid)
	})
}

func TestAuthService_Refresh(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()

	t.Run("valid refresh token returns new pair", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("refresh@example.com").Create()
		code, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)
		_, refresh, err := svc.VerifyOTP(ctx, user.Email, code)
		require.NoError(t, err)

		newAccess, newRefresh, err := svc.Refresh(ctx, refresh)

		require.NoError(t, err)
		assert.NotEmpty(t, newAccess)
		assert.NotEmpty(t, newRefresh)
		assert.NotEqual(t, refresh, newRefresh)
	})

	t.Run("old token rejected after rotation", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("rotate@example.com").Create()
		code, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)
		_, refresh, err := svc.VerifyOTP(ctx, user.Email, code)
		require.NoError(t, err)

		_, _, err = svc.Refresh(ctx, refresh)
		require.NoError(t, err)

		_, _, err = svc.Refresh(ctx, refresh)
		assert.ErrorIs(t, err, auth.ErrRefreshInvalid)
	})

	t.Run("invalid token returns ErrRefreshInvalid", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		svc := newTestAuthService(t)

		_, _, err := svc.Refresh(ctx, "not-a-real-token")
		assert.ErrorIs(t, err, auth.ErrRefreshInvalid)
	})
}

func TestAuthService_Logout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx := context.Background()

	t.Run("logout revokes refresh token", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("logout@example.com").Create()
		code, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)
		_, refresh, err := svc.VerifyOTP(ctx, user.Email, code)
		require.NoError(t, err)

		err = svc.Logout(ctx, refresh)
		require.NoError(t, err)

		_, _, err = svc.Refresh(ctx, refresh)
		assert.ErrorIs(t, err, auth.ErrRefreshInvalid)
	})

	t.Run("logout is idempotent", func(t *testing.T) {
		sharedQueue.Cleanup(t)
		sharedDB.CleanupDatabase(t)
		svc := newTestAuthService(t)

		user := sharedDB.NewUser(t).WithEmail("logout2@example.com").Create()
		code, err := svc.RequestOTP(ctx, user.Email)
		require.NoError(t, err)
		_, refresh, err := svc.VerifyOTP(ctx, user.Email, code)
		require.NoError(t, err)

		require.NoError(t, svc.Logout(ctx, refresh))
		require.NoError(t, svc.Logout(ctx, refresh)) // second logout is fine
	})
}
