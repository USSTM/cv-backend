package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/USSTM/cv-backend/internal/container"
	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

func main() {
	c, err := container.New()
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}
	defer c.Cleanup()

	r := chi.NewMux()
	
	// Get the embedded OpenAPI spec
	spec, err := genapi.GetSwagger()
	if err != nil {
		log.Fatalf("Failed to load OpenAPI spec: %v", err)
	}
	
	// Add authentication middleware
	validator := middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: c.Authenticator.Authenticate,
		},
	})
	r.Use(validator)
	
	genapi.HandlerFromMux(c.Server, r)

	addr := fmt.Sprintf("0.0.0.0:%s", c.Config.Server.Port)
	s := &http.Server{
		Handler: r,
		Addr:    addr,
	}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")
		c.Cleanup()
		os.Exit(0)
	}()

	log.Printf("Server starting on %s", addr)
	log.Fatal(s.ListenAndServe())
}
