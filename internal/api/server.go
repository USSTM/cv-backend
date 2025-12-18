package api

type Server struct {
	db            DatabaseService
	queue         RedisQueueService
	jwtService    JWTService
	authenticator AuthenticatorService
}

func NewServer(db DatabaseService, queue RedisQueueService, jwtService JWTService, authenticator AuthenticatorService) *Server {
	return &Server{
		db:            db,
		queue:         queue,
		jwtService:    jwtService,
		authenticator: authenticator,
	}
}
