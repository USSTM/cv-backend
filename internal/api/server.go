package api

type Server struct {
	db                  DatabaseService
	queue               RedisQueueService
	authService         AuthService
	authenticator       AuthenticatorService
	emailService        EmailService
	s3Service           S3Service
	notificationService NotificationService
}

func NewServer(db DatabaseService, queue RedisQueueService, authService AuthService, authenticator AuthenticatorService, emailService EmailService, s3Service S3Service, notificationService NotificationService) *Server {
	return &Server{
		db:                  db,
		queue:               queue,
		authService:         authService,
		authenticator:       authenticator,
		emailService:        emailService,
		s3Service:           s3Service,
		notificationService: notificationService,
	}
}
