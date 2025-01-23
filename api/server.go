package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/USSTM/cv-backend/api/oas"
)

type Server struct {
	token string
}

func NewServer() Server {
	return Server{
		token: "banana",
	}
}

func (s Server) LoginUser(w http.ResponseWriter, r *http.Request) {
	log.Println("Potato")
	resp := oas.LoginResponse{
		Token: &s.token,
	}

	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s Server) PingProtected(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	bearer := strings.Split(authHeader, "Bearer ")

	if authHeader == "" || bearer[1] != s.token {
		resp := oas.Error{
			401,
			"Unauthorized!",
		}
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(resp)

		return
	}

	resp := oas.PingResponse{
		Message:   "PONG!",
		Timestamp: time.Now(),
	}

	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(resp)
}
