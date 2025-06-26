package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/database"
)

type Server struct {
	token string
	db    *database.Database
}

func NewServer(db *database.Database) *Server {
	return &Server{
		token: "banana",
		db:    db,
	}
}

func (s Server) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req api.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	user, err := s.db.Queries().GetUserByEmail(ctx, string(req.Email))
	if err != nil {
		log.Printf("User not found: %v", err)
		resp := api.Error{
			Code:    400,
			Message: "Invalid email or password.",
		}
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	log.Printf("User found: %s", user.Email)
	resp := api.LoginResponse{
		Token: &s.token,
	}

	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s Server) PingProtected(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	bearer := strings.Split(authHeader, "Bearer ")

	if authHeader == "" || bearer[1] != s.token {
		resp := api.Error{
			Code:    401,
			Message: "Unauthorized!",
		}
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(resp)

		return
	}

	resp := api.PingResponse{
		Message:   "PONG!",
		Timestamp: time.Now(),
	}

	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(resp)
}
