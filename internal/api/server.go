package api

type Server struct {
	db            DatabaseService
	queue         RedisQueueService
	authService   AuthService
	authenticator AuthenticatorService
	emailService  EmailService
	s3Service     S3Service
	dispatcher    NotificationDispatcherService
}

func NewServer(db DatabaseService, queue RedisQueueService, authService AuthService, authenticator AuthenticatorService, emailService EmailService, s3Service S3Service, dispatcher NotificationDispatcherService) *Server {
	return &Server{
		db:            db,
		queue:         queue,
		authService:   authService,
		authenticator: authenticator,
		emailService:  emailService,
		s3Service:     s3Service,
		dispatcher:    dispatcher,
	}
}
