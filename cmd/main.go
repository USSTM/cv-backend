package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/container"
	"github.com/USSTM/cv-backend/internal/logging"
	appmiddleware "github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/swagger"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	cfg := config.Load()

	// Initialize structured logging before anything else (so we can log errors)
	if err := logging.Init(&cfg.Logging); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	logging.Info("Logger initialized successfully",
		"level", cfg.Logging.Level,
		"format", cfg.Logging.Format,
		"filename", cfg.Logging.Filename)

	// create container after (so we can log errors with structured logger)
	c, err := container.New(*cfg)
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}
	defer c.Cleanup()

	r := chi.NewMux()

	// Get the embedded OpenAPI spec
	spec, err := genapi.GetSwagger()
	if err != nil {
		logging.Error("Failed to load OpenAPI spec", "error", err)
	}

	corsHandler := appmiddleware.NewCORSHandler(&c.Config.CORS)
	r.Use(corsHandler)

	// Add request context and logging middlewares AFTER CORS
	r.Use(appmiddleware.RequestContext)
	r.Use(appmiddleware.LoggingMiddleware)

	// group swagger ui routes away from actual API
	r.Group(func(r chi.Router) {
		// Swagger UI routes
		r.Get("/swagger.json", swagger.ServeSwaggerJSON)
		r.Get("/docs/*", httpSwagger.Handler(
			httpSwagger.URL("/swagger.json"),
		))
	})

	// authentication middleware and API
	r.Group(func(r chi.Router) {
		r.Use(middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: c.Authenticator.Authenticate,
			},
		}))

		// strict handler
		strictHandler := genapi.NewStrictHandler(c.Server, nil)
		genapi.HandlerFromMux(strictHandler, r)
	})

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Server.Port)
	s := &http.Server{
		Handler: r,
		Addr:    addr,
	}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logging.Info("Shutting down server...")
		c.Cleanup()
		os.Exit(0)
	}()

	logging.Info("Server starting", "address", addr)
	if err := s.ListenAndServe(); err != nil {
		logging.Error("Server failed", "error", err)
		log.Fatal(err)
	}
}
