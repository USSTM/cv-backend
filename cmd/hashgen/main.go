package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run main.go <email> <password> <connection_string> [role] [scope] [group_id]")
		fmt.Println("Example: go run main.go user@example.com mypassword \"postgres://username:password@localhost/dbname\"")
		fmt.Println("         go run main.go admin@example.com adminpass \"postgres://...\" global_admin global")
		fmt.Println("         go run main.go member@example.com memberpass \"postgres://...\" member group 12345678-1234-1234-1234-123456789012")
		fmt.Println("")
		fmt.Println("Available roles: global_admin, approver, group_admin, member")
		fmt.Println("Available scopes: global, group")
		os.Exit(1)
	}

	email := os.Args[1]
	password := os.Args[2]
	connString := os.Args[3]
	
	var roleName, scope, groupID string
	if len(os.Args) >= 5 {
		roleName = os.Args[4]
	}
	if len(os.Args) >= 6 {
		scope = os.Args[5]
	}
	if len(os.Args) >= 7 {
		groupID = os.Args[6]
	}

	// Generate bcrypt hash
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating hash: %v\n", err)
		os.Exit(1)
	}

	// Connect to database
	conn, err := pgx.Connect(context.Background(), connString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	ctx := context.Background()
	
	// Start transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start transaction: %v\n", err)
		os.Exit(1)
	}
	defer tx.Rollback(ctx)

	// Insert user
	var userID string
	err = tx.QueryRow(ctx, 
		"INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id",
		email, string(hash)).Scan(&userID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("User created successfully: %s (ID: %s)\n", email, userID)

	// If role is specified, assign it
	if roleName != "" {
		// Get role ID
		var roleID int
		err = tx.QueryRow(ctx, "SELECT id FROM roles WHERE name = $1", roleName).Scan(&roleID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to find role '%s': %v\n", roleName, err)
			os.Exit(1)
		}

		// Validate scope and group_id combination
		if scope == "" {
			scope = "global" // default scope
		}
		
		if scope == "global" && groupID != "" {
			fmt.Fprintf(os.Stderr, "Cannot specify group_id with global scope\n")
			os.Exit(1)
		}
		
		if scope == "group" && groupID == "" {
			fmt.Fprintf(os.Stderr, "Must specify group_id with group scope\n")
			os.Exit(1)
		}

		// Insert user role
		var query string
		var args []interface{}
		
		if scope == "global" {
			query = "INSERT INTO user_roles (user_id, role_id, scope, scope_id) VALUES ($1, $2, $3, NULL)"
			args = []interface{}{userID, roleID, scope}
		} else {
			query = "INSERT INTO user_roles (user_id, role_id, scope, scope_id) VALUES ($1, $2, $3, $4)"
			args = []interface{}{userID, roleID, scope, groupID}
		}

		_, err = tx.Exec(ctx, query, args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to assign role: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Role assigned: %s (scope: %s", roleName, scope)
		if groupID != "" {
			fmt.Printf(", group: %s", groupID)
		}
		fmt.Println(")")
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to commit transaction: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("User creation completed successfully!")
}