package container

import (
	"log"

	"github.com/USSTM/cv-backend/internal/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/database"
)

type Container struct {
	Config        *config.Config
	Database      *database.Database
	JWTService    *auth.JWTService
	Authenticator *auth.Authenticator
	Server        *api.Server
}

func New() (*Container, error) {
	cfg := config.Load()

	db, err := database.New(&cfg.Database)
	if err != nil {
		return nil, err
	}

	jwtService, err := auth.NewJWTService([]byte(cfg.JWT.SigningKey), cfg.JWT.Issuer, cfg.JWT.Expiry)
	if err != nil {
		return nil, err
	}

	authenticator := auth.NewAuthenticator(jwtService, db.Queries())

	server := api.NewServer(db, jwtService, authenticator)

	log.Printf("Connected to database: %s:%s", cfg.Database.Host, cfg.Database.Port)

	return &Container{
		Config:        cfg,
		Database:      db,
		JWTService:    jwtService,
		Authenticator: authenticator,
		Server:        server,
	}, nil
}

func (c *Container) Cleanup() {
	if c.Database != nil {
		c.Database.Close()
		log.Println("Database connection closed")
	}
}