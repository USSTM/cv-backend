package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/USSTM/cv-backend/api"
	"github.com/USSTM/cv-backend/api/oas"
	"github.com/go-chi/chi/v5"
)

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=../api/config.yaml ../api/swagger.yaml
//

func main() {
	server := api.NewServer()

	r := chi.NewMux()

	oas.HandlerFromMux(server, r)

	s := &http.Server{
		Handler: r,
		Addr:    "0.0.0.0:8080",
	}

	fmt.Println("Banana")

	log.Fatal(s.ListenAndServe())
}
