package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

type SeedData struct {
	Groups    []Group    `yaml:"groups"`
	Items     []Item     `yaml:"items"`
	Users     []User     `yaml:"users"`
	UserRoles []UserRole `yaml:"user_roles"`
}

type Group struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Item struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Stock       int      `yaml:"stock"`
	Description string   `yaml:"description"`
	URLs        []string `yaml:"urls"`
}

type User struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
}

type UserRole struct {
	UserEmail string  `yaml:"user_email"`
	RoleName  string  `yaml:"role_name"`
	Scope     string  `yaml:"scope"`
	GroupName *string `yaml:"group_name,omitempty"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return errors.New("command required")
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "seed":
		return seedCommand(args)
	case "nuke":
		return nukeCommand(args)
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func seedCommand(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	file := fs.String("file", "", "YAML file to seed from")
	dir := fs.String("dir", "", "Directory of YAML files to seed from")
	dryRun := fs.Bool("dry-run", false, "Validate files without making seedDB changes")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	files, err := resolveFiles(*file, *dir)
	if err != nil {
		return err
	}

	seedData, err := loadSeedData(files)
	if err != nil {
		return fmt.Errorf("failed to load seed data: %w", err)
	}

	if *dryRun {
		fmt.Println("dry run: validating data structure")
		return validateSeedData(seedData)
	}

	cfg := config.Load()
	seedDB, err := database.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("seedDB connection failed: %w", err)
	}
	defer seedDB.Close()

	fmt.Printf("seeding seedDB from %d file(s)\n", len(files))
	return applySeedData(context.Background(), seedDB.Queries(), seedData)
}

func nukeCommand(args []string) error {
	fs := flag.NewFlagSet("nuke", flag.ExitOnError)
	force := fs.Bool("force", false, "Skip confirmation prompt")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	if !*force && !confirmNuke() {
		fmt.Println("operation cancelled")
		return nil
	}

	return nukeDatabase()
}

func resolveFiles(file, dir string) ([]string, error) {
	if file == "" && dir == "" {
		return nil, errors.New("must specify either --file or --dir")
	}

	if file != "" && dir != "" {
		return nil, errors.New("cannot specify both --file and --dir")
	}

	if file != "" {
		return []string{file}, nil
	}

	return findYAMLFiles(dir)
}

func findYAMLFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && isYAMLFile(path) {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory %s: %w", dir, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no YAML files found in directory: %s", dir)
	}

	return files, nil
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func loadSeedData(files []string) (*SeedData, error) {
	combined := &SeedData{}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		var fileData SeedData
		if err := yaml.Unmarshal(data, &fileData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML in %s: %w", file, err)
		}

		// Combine data from all files
		combined.Groups = append(combined.Groups, fileData.Groups...)
		combined.Items = append(combined.Items, fileData.Items...)
		combined.Users = append(combined.Users, fileData.Users...)
		combined.UserRoles = append(combined.UserRoles, fileData.UserRoles...)
	}

	return combined, nil
}

func validateSeedData(data *SeedData) error {
	fmt.Printf("  Groups: %d\n", len(data.Groups))
	fmt.Printf("  Items: %d\n", len(data.Items))
	fmt.Printf("  Users: %d\n", len(data.Users))
	fmt.Printf("  User Roles: %d\n", len(data.UserRoles))
	fmt.Println("data structure is valid")
	return nil
}

func applySeedData(ctx context.Context, queries *db.Queries, data *SeedData) error {
	// Create groups first
	groupIDs := make(map[string]uuid.UUID)
	for _, group := range data.Groups {
		params := db.CreateGroupParams{
			Name:        group.Name,
			Description: pgtype.Text{String: group.Description, Valid: true},
		}
		groupResult, err := queries.CreateGroup(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create group %s: %w", group.Name, err)
		}
		groupIDs[group.Name] = groupResult.ID
		fmt.Printf("created group: %s\n", group.Name)
	}

	// Create items
	for _, item := range data.Items {
		params := db.CreateItemParams{
			Name:        item.Name,
			Type:        db.ItemType(item.Type),
			Stock:       int32(item.Stock),
			Description: pgtype.Text{String: item.Description, Valid: true},
			Urls:        item.URLs,
		}
		if _, err := queries.CreateItem(ctx, params); err != nil {
			return fmt.Errorf("failed to create item %s: %w", item.Name, err)
		}
		fmt.Printf("created item: %s\n", item.Name)
	}

	// Create users
	userIDs := make(map[string]uuid.UUID)
	for _, user := range data.Users {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password for %s: %w", user.Email, err)
		}

		params := db.CreateUserParams{
			Email:        user.Email,
			PasswordHash: string(hashedPassword),
		}
		userResult, err := queries.CreateUser(ctx, params)
		if err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Email, err)
		}
		userIDs[user.Email] = userResult.ID
		fmt.Printf("created user: %s\n", user.Email)
	}

	// Create user roles
	for _, userRole := range data.UserRoles {
		userID, exists := userIDs[userRole.UserEmail]
		if !exists {
			return fmt.Errorf("user %s not found for role assignment", userRole.UserEmail)
		}

		var scopeID *uuid.UUID
		if userRole.GroupName != nil {
			groupID, exists := groupIDs[*userRole.GroupName]
			if !exists {
				return fmt.Errorf("group %s not found for user role", *userRole.GroupName)
			}
			scopeID = &groupID
		}

		params := db.CreateUserRoleParams{
			UserID:   &userID,
			RoleName: pgtype.Text{String: userRole.RoleName, Valid: true},
			Scope:    db.ScopeType(userRole.Scope),
			ScopeID:  scopeID,
		}
		if err := queries.CreateUserRole(ctx, params); err != nil {
			return fmt.Errorf("failed to create user role for %s: %w", userRole.UserEmail, err)
		}
		fmt.Printf("assigned role %s to user: %s\n", userRole.RoleName, userRole.UserEmail)
	}

	fmt.Println("seeding completed")
	return nil
}

func nukeDatabase() error {
	cfg := config.Load()
	
	// Open database connection for goose
	sqlDB, err := goose.OpenDBWithDriver("postgres", cfg.Database.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			fmt.Printf("warning: failed to close database: %v\n", err)
		}
	}()

	fmt.Println("resetting database with goose...")

	// Reset database (down all migrations)
	fmt.Println("rolling back all migrations...")
	if err := goose.Reset(sqlDB, "db/migrations"); err != nil {
		return fmt.Errorf("failed to reset migrations: %w", err)
	}

	// Apply all migrations (back up to current state)
	fmt.Println("applying all migrations...")
	if err := goose.Up(sqlDB, "db/migrations"); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	fmt.Println("database reset complete - ready for seeding")
	return nil
}

func confirmNuke() bool {
	fmt.Print("warning: this will delete all data from the database. are you sure? (yes/no): ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}

	return strings.ToLower(strings.TrimSpace(response)) == "yes"
}

func printUsage() {
	fmt.Println("Seeder Tool - Database seeding utility for Campus Vault")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  seeder <command> [flags]")
	fmt.Println()
	fmt.Println("COMMANDS:")
	fmt.Println("  seed        Seed database from YAML files")
	fmt.Println("  nuke        Delete all data from database")
	fmt.Println("  help        Show this help message")
	fmt.Println()
	fmt.Println("SEED FLAGS:")
	fmt.Println("  --file      Path to a single YAML file")
	fmt.Println("  --dir       Path to directory containing YAML files")
	fmt.Println("  --dry-run   Validate files without making database changes")
	fmt.Println()
	fmt.Println("NUKE FLAGS:")
	fmt.Println("  --force     Skip confirmation prompt")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  seeder seed --file dev-data.yaml")
	fmt.Println("  seeder seed --dir ./seed-data/")
	fmt.Println("  seeder seed --dir ./seed-data/ --dry-run")
	fmt.Println("  seeder nuke")
	fmt.Println("  seeder nuke --force")
}
