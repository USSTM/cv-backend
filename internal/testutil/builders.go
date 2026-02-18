package testutil

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/USSTM/cv-backend/internal/rbac"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// TestItem represents a test item
type TestItem struct {
	ID          uuid.UUID
	Name        string
	Description string
	Type        string
	Stock       int
	Urls        []string
}

// TestSignUpCode represents a test sign-up code
type TestSignUpCode struct {
	ID        uuid.UUID
	Code      string
	RoleName  string
	Scope     string
	ScopeID   uuid.UUID
	CreatedAt time.Time
	UsedAt    time.Time
	ExpiresAt time.Time
	CreatedBy uuid.UUID
}

// TestGroup represents a test group
type TestGroup struct {
	ID          uuid.UUID
	Name        string
	Description string
}

// TestUser represents a test user
type TestUser struct {
	ID    uuid.UUID
	Email string
	Roles []UserRole
}

// UserRole represents a role assignment with scope
type UserRole struct {
	RoleName string
	Scope    string // "global" or "group"
	GroupID  *uuid.UUID
}

// GroupBuilder provides a fluent interface for creating test groups
type GroupBuilder struct {
	name        string
	description string
	testDB      *TestDatabase
	t           *testing.T
}

// NewGroup creates a new group builder
func (tdb *TestDatabase) NewGroup(t *testing.T) *GroupBuilder {
	return &GroupBuilder{
		name:        "Test Group",
		description: "Test group description",
		testDB:      tdb,
		t:           t,
	}
}

// WithName sets the group name
func (gb *GroupBuilder) WithName(name string) *GroupBuilder {
	gb.name = name
	return gb
}

// WithDescription sets the group description
func (gb *GroupBuilder) WithDescription(desc string) *GroupBuilder {
	gb.description = desc
	return gb
}

// Create creates the group in the database and returns the TestGroup
func (gb *GroupBuilder) Create() *TestGroup {
	ctx := context.Background()

	group, err := gb.testDB.Queries().CreateGroup(ctx, db.CreateGroupParams{
		Name:        gb.name,
		Description: pgtype.Text{String: gb.description, Valid: gb.description != ""},
	})
	require.NoError(gb.t, err, "Failed to create group")

	return &TestGroup{
		ID:          group.ID,
		Name:        group.Name,
		Description: group.Description.String,
	}
}

// UserBuilder provides a fluent interface for creating test users
type UserBuilder struct {
	email  string
	roles  []UserRole
	testDB *TestDatabase
	t      *testing.T
}

// NewUser creates a new user builder
func (tdb *TestDatabase) NewUser(t *testing.T) *UserBuilder {
	return &UserBuilder{
		email:  "test@example.com",
		roles:  []UserRole{},
		testDB: tdb,
		t:      t,
	}
}

// WithEmail sets the user's email
func (ub *UserBuilder) WithEmail(email string) *UserBuilder {
	ub.email = email
	return ub
}

// AsMember adds the member role with global scope
func (ub *UserBuilder) AsMember() *UserBuilder {
	ub.roles = append(ub.roles, UserRole{
		RoleName: rbac.RoleMember,
		Scope:    "global",
		GroupID:  nil,
	})
	return ub
}

// AsMemberOf adds the member role scoped to a specific group
func (ub *UserBuilder) AsMemberOf(group *TestGroup) *UserBuilder {
	ub.roles = append(ub.roles, UserRole{
		RoleName: rbac.RoleMember,
		Scope:    "group",
		GroupID:  &group.ID,
	})
	return ub
}

// AsGroupAdminOf adds the group_admin role scoped to a specific group
func (ub *UserBuilder) AsGroupAdminOf(group *TestGroup) *UserBuilder {
	ub.roles = append(ub.roles, UserRole{
		RoleName: rbac.RoleGroupAdmin,
		Scope:    "group",
		GroupID:  &group.ID,
	})
	return ub
}

// AsApprover adds the approver role with global scope
func (ub *UserBuilder) AsApprover() *UserBuilder {
	ub.roles = append(ub.roles, UserRole{
		RoleName: rbac.RoleApprover,
		Scope:    "global",
		GroupID:  nil,
	})
	return ub
}

// AsGlobalAdmin adds the global_admin role with global scope
func (ub *UserBuilder) AsGlobalAdmin() *UserBuilder {
	ub.roles = append(ub.roles, UserRole{
		RoleName: rbac.RoleGlobalAdmin,
		Scope:    "global",
		GroupID:  nil,
	})
	return ub
}

// WithCustomRole adds a custom role with specified scope
func (ub *UserBuilder) WithCustomRole(roleName, scope string, groupID *uuid.UUID) *UserBuilder {
	ub.roles = append(ub.roles, UserRole{
		RoleName: roleName,
		Scope:    scope,
		GroupID:  groupID,
	})
	return ub
}

// Create creates the user in the database and returns the TestUser
func (ub *UserBuilder) Create() *TestUser {
	ctx := context.Background()

	// Create user in database
	dbUser, err := ub.testDB.Queries().CreateUser(ctx, ub.email)
	require.NoError(ub.t, err, "Failed to create user")

	// Assign roles with proper scoping
	for _, role := range ub.roles {
		err := ub.testDB.Queries().CreateUserRole(ctx, db.CreateUserRoleParams{
			UserID:   &dbUser.ID,
			RoleName: pgtype.Text{String: role.RoleName, Valid: true},
			Scope:    db.ScopeType(role.Scope),
			ScopeID:  role.GroupID,
		})
		require.NoError(ub.t, err, "Failed to assign role %s to user", role.RoleName)
	}

	return &TestUser{
		ID:    dbUser.ID,
		Email: dbUser.Email,
		Roles: ub.roles,
	}
}

// ToAuthenticatedUser converts TestUser to auth.AuthenticatedUser
func (u *TestUser) ToAuthenticatedUser(ctx context.Context, queries *db.Queries) *auth.AuthenticatedUser {
	// Get actual permissions from database
	permissions, err := queries.GetUserPermissions(ctx, &u.ID)
	if err != nil {
		// Return basic structure if query fails
		return &auth.AuthenticatedUser{
			ID:          u.ID,
			Email:       u.Email,
			Permissions: []db.GetUserPermissionsRow{},
			Roles:       []db.GetUserRolesRow{},
		}
	}

	roles, err := queries.GetUserRoles(ctx, &u.ID)
	if err != nil {
		roles = []db.GetUserRolesRow{}
	}

	return &auth.AuthenticatedUser{
		ID:          u.ID,
		Email:       u.Email,
		Permissions: permissions,
		Roles:       roles,
	}
}

// ItemBuilder provides a fluent interface for creating test items
type ItemBuilder struct {
	name        string
	description string
	itemType    string
	stock       int
	urls        []string
	testDB      *TestDatabase
	t           *testing.T
}

// NewItem creates a new item builder
func (tdb *TestDatabase) NewItem(t *testing.T) *ItemBuilder {
	return &ItemBuilder{
		name:        "Test Item",
		description: "Test item description",
		itemType:    "low", // Default to low priority
		stock:       10,
		urls:        []string{},
		testDB:      tdb,
		t:           t,
	}
}

// WithName sets the item name
func (ib *ItemBuilder) WithName(name string) *ItemBuilder {
	ib.name = name
	return ib
}

// WithDescription sets the item description
func (ib *ItemBuilder) WithDescription(desc string) *ItemBuilder {
	ib.description = desc
	return ib
}

// WithType sets the item type (low, medium, high)
func (ib *ItemBuilder) WithType(itemType string) *ItemBuilder {
	ib.itemType = strings.ToLower(itemType)
	return ib
}

// WithStock sets the item stock
func (ib *ItemBuilder) WithStock(stock int) *ItemBuilder {
	ib.stock = stock
	return ib
}

// WithUrls sets the item URLs
func (ib *ItemBuilder) WithUrls(urls []string) *ItemBuilder {
	ib.urls = urls
	return ib
}

// Create creates the item in the database and returns the TestItem
func (ib *ItemBuilder) Create() *TestItem {
	ctx := context.Background()

	item, err := ib.testDB.Queries().CreateItem(ctx, db.CreateItemParams{
		Name:        ib.name,
		Description: pgtype.Text{String: ib.description, Valid: ib.description != ""},
		Type:        db.ItemType(ib.itemType),
		Stock:       int32(ib.stock),
		Urls:        ib.urls,
	})
	require.NoError(ib.t, err, "Failed to create item")

	return &TestItem{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description.String,
		Type:        string(item.Type),
		Stock:       int(item.Stock),
		Urls:        item.Urls,
	}
}

// AssignUserToGroup assigns a user to a group with the specified role
func (tdb *TestDatabase) AssignUserToGroup(t *testing.T, userID, groupID uuid.UUID, roleName string) {
	ctx := context.Background()

	err := tdb.Queries().CreateUserRole(ctx, db.CreateUserRoleParams{
		UserID:   &userID,
		RoleName: pgtype.Text{String: roleName, Valid: true},
		Scope:    db.ScopeTypeGroup,
		ScopeID:  &groupID,
	})
	require.NoError(t, err, "Failed to assign user %s to group %s with role %s", userID, groupID, roleName)
}
