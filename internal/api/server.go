package api

type Server struct {
	db            DatabaseService
	queue         RedisQueueService
	jwtService    JWTService
	authenticator AuthenticatorService
	emailService  EmailService
}

func NewServer(db DatabaseService, queue RedisQueueService, jwtService JWTService, authenticator AuthenticatorService, emailService EmailService) *Server {
	return &Server{
		db:            db,
		queue:         queue,
		jwtService:    jwtService,
		authenticator: authenticator,
		emailService:  emailService,
	}
}
