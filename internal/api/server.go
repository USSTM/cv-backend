package api

type Server struct {
	db            DatabaseService
	queue         RedisQueueService
	jwtService    JWTService
	authenticator AuthenticatorService
	emailService  EmailService
	s3Service     S3Service
}

func NewServer(db DatabaseService, queue RedisQueueService, jwtService JWTService, authenticator AuthenticatorService, emailService EmailService, s3Service S3Service) *Server {
	return &Server{
		db:            db,
		queue:         queue,
		jwtService:    jwtService,
		authenticator: authenticator,
		emailService:  emailService,
		s3Service:     s3Service,
	}
}
