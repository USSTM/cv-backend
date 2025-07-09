package api

type Server struct {
	db            DatabaseService
	jwtService    JWTService
	authenticator AuthenticatorService
}

func NewServer(db DatabaseService, jwtService JWTService, authenticator AuthenticatorService) *Server {
	return &Server{
		db:            db,
		jwtService:    jwtService,
		authenticator: authenticator,
	}
}
