package api

import (
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/database"
)

type Server struct {
	db            *database.Database
	jwtService    *auth.JWTService
	authenticator *auth.Authenticator
}

func NewServer(db *database.Database, jwtService *auth.JWTService, authenticator *auth.Authenticator) *Server {
	return &Server{
		db:            db,
		jwtService:    jwtService,
		authenticator: authenticator,
	}
}
