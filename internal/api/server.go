package api

import "time"

type CookieConfig struct {
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
	Secure        bool
}

type Server struct {
	db            DatabaseService
	queue         RedisQueueService
	authService   AuthService
	authenticator AuthenticatorService
	emailService  EmailService
	s3Service     S3Service
	dispatcher    NotificationDispatcherService
	cookies       CookieConfig
}

func NewServer(db DatabaseService, queue RedisQueueService, authService AuthService, authenticator AuthenticatorService, emailService EmailService, s3Service S3Service, dispatcher NotificationDispatcherService, cookieConfigs ...CookieConfig) *Server {
	cookies := CookieConfig{AccessExpiry: 15 * time.Minute, RefreshExpiry: 7 * 24 * time.Hour}
	if len(cookieConfigs) > 0 {
		cookies = cookieConfigs[0]
	}
	return &Server{
		db:            db,
		queue:         queue,
		authService:   authService,
		authenticator: authenticator,
		emailService:  emailService,
		s3Service:     s3Service,
		dispatcher:    dispatcher,
		cookies:       cookies,
	}
}
