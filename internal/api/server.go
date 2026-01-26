package api

type Server struct {
	db            DatabaseService
	queue         RedisQueueService
	email         LocalStackService
	jwtService    JWTService
	authenticator AuthenticatorService
}

func NewServer(db DatabaseService, queue RedisQueueService, email LocalStackService, jwtService JWTService, authenticator AuthenticatorService) *Server {
	return &Server{
		db:            db,
		queue:         queue,
		email:         email,
		jwtService:    jwtService,
		authenticator: authenticator,
	}
}
