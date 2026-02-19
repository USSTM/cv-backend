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
	"time"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/USSTM/cv-backend/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"gopkg.in/yaml.v3"
)

type SeedData struct {
	Groups       []Group        `yaml:"groups"`
	Items        []Item         `yaml:"items"`
	Users        []User         `yaml:"users"`
	UserRoles    []UserRole     `yaml:"user_roles"`
	Availability []Availability `yaml:"availability"`
	Borrowings   []Borrowing    `yaml:"borrowings"`
	Requests     []Request      `yaml:"requests"`
	Bookings     []Booking      `yaml:"bookings"`
	CartItems    []CartItem     `yaml:"cart_items"`
	ItemTakings  []ItemTaking   `yaml:"item_takings"`
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
	Email string `yaml:"email"`
}

type UserRole struct {
	UserEmail string  `yaml:"user_email"`
	RoleName  string  `yaml:"role_name"`
	Scope     string  `yaml:"scope"`
	GroupName *string `yaml:"group_name,omitempty"`
}

type Availability struct {
	UserEmail     string `yaml:"user_email"`
	Date          string `yaml:"date"`            // "2025-02-05"
	TimeSlotStart string `yaml:"time_slot_start"` // "09:00:00"
}

type Borrowing struct {
	UserEmail          string  `yaml:"user_email"`
	GroupName          string  `yaml:"group_name"`
	ItemName           string  `yaml:"item_name"`
	Quantity           int     `yaml:"quantity"`
	BorrowedAt         *string `yaml:"borrowed_at,omitempty"` // ISO8601, defaults to NOW()
	DueDate            string  `yaml:"due_date"`              // ISO8601
	ReturnedAt         *string `yaml:"returned_at,omitempty"` // ISO8601, null = active
	BeforeCondition    string  `yaml:"before_condition"`      // "good", "pristine", etc.
	BeforeConditionURL string  `yaml:"before_condition_url"`
	AfterCondition     *string `yaml:"after_condition,omitempty"`
	AfterConditionURL  *string `yaml:"after_condition_url,omitempty"`
}

type Request struct {
	UserEmail                 string  `yaml:"user_email"`
	GroupName                 string  `yaml:"group_name"`
	ItemName                  string  `yaml:"item_name"`
	Quantity                  int     `yaml:"quantity"`
	Status                    string  `yaml:"status"` // "pending", "approved", "denied", "fulfilled"
	RequestedAt               *string `yaml:"requested_at,omitempty"`
	ReviewedByEmail           *string `yaml:"reviewed_by_email,omitempty"`
	ReviewedAt                *string `yaml:"reviewed_at,omitempty"`
	FulfilledAt               *string `yaml:"fulfilled_at,omitempty"`
	PreferredAvailabilityDate *string `yaml:"preferred_availability_date,omitempty"`
	PreferredTimeSlotStart    *string `yaml:"preferred_time_slot_start,omitempty"`
}

type Booking struct {
	RequesterEmail       string  `yaml:"requester_email"`
	ManagerEmail         string  `yaml:"manager_email"`
	ItemName             string  `yaml:"item_name"`
	GroupName            string  `yaml:"group_name"`
	AvailabilityDate     string  `yaml:"availability_date"`      // date from Availability
	AvailabilityTimeSlot string  `yaml:"availability_time_slot"` // time_slot_start
	PickupDate           string  `yaml:"pickup_date"`            // ISO8601
	PickupLocation       string  `yaml:"pickup_location"`
	ReturnDate           string  `yaml:"return_date"` // ISO8601
	ReturnLocation       string  `yaml:"return_location"`
	Status               string  `yaml:"status"` // "pending_confirmation", "confirmed", "cancelled"
	ConfirmedAt          *string `yaml:"confirmed_at,omitempty"`
	ConfirmedByEmail     *string `yaml:"confirmed_by_email,omitempty"`
}

type CartItem struct {
	UserEmail string `yaml:"user_email"`
	GroupName string `yaml:"group_name"`
	ItemName  string `yaml:"item_name"`
	Quantity  int    `yaml:"quantity"`
}

type ItemTaking struct {
	UserEmail string  `yaml:"user_email"`
	GroupName string  `yaml:"group_name"`
	ItemName  string  `yaml:"item_name"`
	Quantity  int     `yaml:"quantity"`
	TakenAt   *string `yaml:"taken_at,omitempty"` // ISO8601, defaults to NOW()
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

		// Combine data from all YAML files
		combined.Groups = append(combined.Groups, fileData.Groups...)
		combined.Items = append(combined.Items, fileData.Items...)
		combined.Users = append(combined.Users, fileData.Users...)
		combined.UserRoles = append(combined.UserRoles, fileData.UserRoles...)
		combined.Availability = append(combined.Availability, fileData.Availability...)
		combined.Borrowings = append(combined.Borrowings, fileData.Borrowings...)
		combined.Requests = append(combined.Requests, fileData.Requests...)
		combined.Bookings = append(combined.Bookings, fileData.Bookings...)
		combined.CartItems = append(combined.CartItems, fileData.CartItems...)
		combined.ItemTakings = append(combined.ItemTakings, fileData.ItemTakings...)
	}

	return combined, nil
}

func validateSeedData(data *SeedData) error {
	fmt.Printf("  Groups: %d\n", len(data.Groups))
	fmt.Printf("  Items: %d\n", len(data.Items))
	fmt.Printf("  Users: %d\n", len(data.Users))
	fmt.Printf("  User Roles: %d\n", len(data.UserRoles))
	fmt.Printf("  Availability: %d\n", len(data.Availability))
	fmt.Printf("  Borrowings: %d\n", len(data.Borrowings))
	fmt.Printf("  Requests: %d\n", len(data.Requests))
	fmt.Printf("  Bookings: %d\n", len(data.Bookings))
	fmt.Printf("  Cart Items: %d\n", len(data.CartItems))
	fmt.Printf("  Item Takings: %d\n", len(data.ItemTakings))
	fmt.Println("data structure is valid")
	return nil
}

func applySeedData(ctx context.Context, queries *db.Queries, data *SeedData) error {
	// create groups first, not dependent on other tables
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

	// create items second, not dependent on other tables
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

	// create users , not dependent on other tables
	userIDs := make(map[string]uuid.UUID)
	for _, user := range data.Users {
		userResult, err := queries.CreateUser(ctx, user.Email)
		if err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Email, err)
		}
		userIDs[user.Email] = userResult.ID
		fmt.Printf("created user: %s\n", user.Email)
	}

	// create roles, depends on users
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

	// create availability, depends on users
	availabilityIDs := make(map[string]uuid.UUID) // key: "email_date_timeslot"
	for _, avail := range data.Availability {
		userID, exists := userIDs[avail.UserEmail]
		if !exists {
			return fmt.Errorf("user %s not found for availability", avail.UserEmail)
		}

		date, err := time.Parse("2006-01-02", avail.Date)
		if err != nil {
			return fmt.Errorf("invalid date format for availability %s: %w", avail.Date, err)
		}

		// time slot start time (HH:MM:SS)
		parts := strings.Split(avail.TimeSlotStart, ":")
		if len(parts) != 3 {
			return fmt.Errorf("invalid time format: %s", avail.TimeSlotStart)
		}
		hour, _ := time.Parse("15", parts[0])
		minute, _ := time.Parse("04", parts[1])
		second, _ := time.Parse("05", parts[2])

		timeSlot, err := queries.GetTimeSlotByStartTime(ctx, pgtype.Time{
			Microseconds: int64(hour.Hour()*3600+minute.Minute()*60+second.Second()) * 1000000,
			Valid:        true,
		})
		if err != nil {
			return fmt.Errorf("time slot not found for %s: %w", avail.TimeSlotStart, err)
		}

		timeSlotID := timeSlot.ID
		result, err := queries.CreateAvailability(ctx, db.CreateAvailabilityParams{
			ID:         uuid.New(),
			UserID:     &userID,
			TimeSlotID: &timeSlotID,
			Date:       pgtype.Date{Time: date, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create availability for %s on %s: %w", avail.UserEmail, avail.Date, err)
		}

		key := fmt.Sprintf("%s_%s_%s", avail.UserEmail, avail.Date, avail.TimeSlotStart)
		availabilityIDs[key] = result.ID
		fmt.Printf("created availability: %s on %s at %s\n", avail.UserEmail, avail.Date, avail.TimeSlotStart)
	}

	// create requests before borrowings
	requestIDs := make(map[string]uuid.UUID) // key: "email_itemname_status"
	for _, req := range data.Requests {
		userID, exists := userIDs[req.UserEmail]
		if !exists {
			return fmt.Errorf("user %s not found for request", req.UserEmail)
		}

		groupID, exists := groupIDs[req.GroupName]
		if !exists {
			return fmt.Errorf("group %s not found for request", req.GroupName)
		}

		item, err := queries.GetItemByName(ctx, req.ItemName)
		if err != nil {
			return fmt.Errorf("item %s not found for request: %w", req.ItemName, err)
		}

		// use RequestItem query if pending
		if req.Status == "pending" {
			result, err := queries.RequestItem(ctx, db.RequestItemParams{
				UserID:   &userID,
				GroupID:  &groupID,
				ID:       item.ID,
				Quantity: int32(req.Quantity),
			})
			if err != nil {
				return fmt.Errorf("failed to create request for %s: %w", req.UserEmail, err)
			}
			key := fmt.Sprintf("%s_%s_%s", req.UserEmail, req.ItemName, req.Status)
			requestIDs[key] = result.ID
			fmt.Printf("created pending request: %s for %s\n", req.UserEmail, req.ItemName)
		} else {
			// skip non-pending requests in seeding
			fmt.Printf("skipping non-pending request (status: %s) - not yet implemented in seeder\n", req.Status)
		}
	}

	// borrowings
	for _, borrow := range data.Borrowings {
		userID, exists := userIDs[borrow.UserEmail]
		if !exists {
			return fmt.Errorf("user %s not found for borrowing", borrow.UserEmail)
		}

		groupID, exists := groupIDs[borrow.GroupName]
		if !exists {
			return fmt.Errorf("group %s not found for borrowing", borrow.GroupName)
		}

		item, err := queries.GetItemByName(ctx, borrow.ItemName)
		if err != nil {
			return fmt.Errorf("item %s not found for borrowing: %w", borrow.ItemName, err)
		}

		dueDate, err := time.Parse(time.RFC3339, borrow.DueDate)
		if err != nil {
			return fmt.Errorf("invalid due date format for borrowing: %w", err)
		}

		result, err := queries.BorrowItem(ctx, db.BorrowItemParams{
			UserID:             &userID,
			GroupID:            &groupID,
			ID:                 item.ID,
			Quantity:           int32(borrow.Quantity),
			DueDate:            pgtype.Timestamp{Time: dueDate, Valid: true},
			BeforeCondition:    db.Condition(borrow.BeforeCondition),
			BeforeConditionUrl: borrow.BeforeConditionURL,
		})
		if err != nil {
			return fmt.Errorf("failed to create borrowing for %s: %w", borrow.UserEmail, err)
		}

		// returned_at is specified, need to update the borrowing record
		if borrow.ReturnedAt != nil {
			// skip for now
			fmt.Printf("created borrowing: %s borrowed %s (returned_at update not yet implemented)\n",
				borrow.UserEmail, borrow.ItemName)
		} else {
			fmt.Printf("created active borrowing: %s borrowed %s\n", borrow.UserEmail, borrow.ItemName)
		}

		_ = result // trick lint
	}

	// bookings
	for _, booking := range data.Bookings {
		requesterID, exists := userIDs[booking.RequesterEmail]
		if !exists {
			return fmt.Errorf("requester %s not found for booking", booking.RequesterEmail)
		}

		managerID, exists := userIDs[booking.ManagerEmail]
		if !exists {
			return fmt.Errorf("manager %s not found for booking", booking.ManagerEmail)
		}

		groupID, exists := groupIDs[booking.GroupName]
		if !exists {
			return fmt.Errorf("group %s not found for booking", booking.GroupName)
		}

		item, err := queries.GetItemByName(ctx, booking.ItemName)
		if err != nil {
			return fmt.Errorf("item %s not found for booking: %w", booking.ItemName, err)
		}

		availKey := fmt.Sprintf("%s_%s_%s", booking.ManagerEmail,
			booking.AvailabilityDate, booking.AvailabilityTimeSlot)
		availID, exists := availabilityIDs[availKey]
		if !exists {
			return fmt.Errorf("availability not found for booking: %s", availKey)
		}

		// timestamps
		pickupDate, err := time.Parse(time.RFC3339, booking.PickupDate)
		if err != nil {
			return fmt.Errorf("invalid pickup date format: %w", err)
		}

		returnDate, err := time.Parse(time.RFC3339, booking.ReturnDate)
		if err != nil {
			return fmt.Errorf("invalid return date format: %w", err)
		}

		itemID := item.ID
		result, err := queries.CreateBooking(ctx, db.CreateBookingParams{
			ID:             uuid.New(),
			RequesterID:    &requesterID,
			ManagerID:      &managerID,
			ItemID:         &itemID,
			GroupID:        &groupID,
			AvailabilityID: &availID,
			PickUpDate:     pgtype.Timestamp{Time: pickupDate, Valid: true},
			PickUpLocation: booking.PickupLocation,
			ReturnDate:     pgtype.Timestamp{Time: returnDate, Valid: true},
			ReturnLocation: booking.ReturnLocation,
			Status:         db.RequestStatus(booking.Status),
		})
		if err != nil {
			return fmt.Errorf("failed to create booking for %s: %w", booking.RequesterEmail, err)
		}

		fmt.Printf("created booking: %s for %s (status: %s)\n",
			booking.RequesterEmail, booking.ItemName, booking.Status)

		_ = result // trick lint
	}

	// cart items
	for _, cart := range data.CartItems {
		userID, exists := userIDs[cart.UserEmail]
		if !exists {
			return fmt.Errorf("user %s not found for cart item", cart.UserEmail)
		}

		groupID, exists := groupIDs[cart.GroupName]
		if !exists {
			return fmt.Errorf("group %s not found for cart item", cart.GroupName)
		}

		item, err := queries.GetItemByName(ctx, cart.ItemName)
		if err != nil {
			return fmt.Errorf("item %s not found for cart: %w", cart.ItemName, err)
		}

		_, err = queries.AddToCart(ctx, db.AddToCartParams{
			GroupID:  groupID,
			UserID:   userID,
			ItemID:   item.ID,
			Quantity: int32(cart.Quantity),
		})
		if err != nil {
			return fmt.Errorf("failed to add to cart for %s: %w", cart.UserEmail, err)
		}

		fmt.Printf("added to cart: %s - %d x %s\n", cart.UserEmail, cart.Quantity, cart.ItemName)
	}

	// create takiings
	for _, taking := range data.ItemTakings {
		userID, exists := userIDs[taking.UserEmail]
		if !exists {
			return fmt.Errorf("user %s not found for item taking", taking.UserEmail)
		}

		groupID, exists := groupIDs[taking.GroupName]
		if !exists {
			return fmt.Errorf("group %s not found for item taking", taking.GroupName)
		}

		item, err := queries.GetItemByName(ctx, taking.ItemName)
		if err != nil {
			return fmt.Errorf("item %s not found for taking: %w", taking.ItemName, err)
		}

		_, err = queries.RecordItemTaking(ctx, db.RecordItemTakingParams{
			UserID:   userID,
			GroupID:  groupID,
			ItemID:   item.ID,
			Quantity: int32(taking.Quantity),
		})
		if err != nil {
			return fmt.Errorf("failed to record item taking for %s: %w", taking.UserEmail, err)
		}

		fmt.Printf("recorded taking: %s took %d x %s\n", taking.UserEmail, taking.Quantity, taking.ItemName)

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
