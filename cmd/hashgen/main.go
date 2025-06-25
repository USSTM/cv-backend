package main

import (
	"context"
	"fmt"
	"os"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <email> <password> <role>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s admin@torontomu.ca mypassword admin\n", os.Args[0])
		os.Exit(1)
	}

	email := os.Args[1]
	password := os.Args[2]
	role := os.Args[3]

	// Generate bcrypt hash
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating hash: %v\n", err)
		os.Exit(1)
	}

	// Connect to database
	cfg := config.Load()
	database, err := database.New(&cfg.Database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Insert user
	ctx := context.Background()
	user, err := database.Queries().CreateUser(ctx, db.CreateUserParams{
		Email:        email,
		PasswordHash: pgtype.Text{String: string(hash), Valid: true},
		Role:         db.UserRole(role),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User created successfully: %s (%s)\n", user.Email, user.Role)
}