package rbac

// Permission constants for RBAC checks
// These match the permissions defined in db/migrations/20250626025312_seed_roles_permissions.sql
const (
	// View permissions
	ViewAllData   = "view_all_data"   // View all data system-wide
	ViewOwnData   = "view_own_data"   // View user's own data
	ViewGroupData = "view_group_data" // View group-scoped data
	ViewItems     = "view_items"      // View item catalog

	// Management permissions
	ManageCart          = "manage_cart"           // Manage shopping cart
	ManageItems         = "manage_items"          // CRUD operations on items
	ManageGroups        = "manage_groups"         // CRUD operations on groups
	ManageUsers         = "manage_users"          // CRUD operations on users
	ManageGroupUsers    = "manage_group_users"    // Manage users within group
	ManageTimeSlots     = "manage_time_slots"     // Manage availability/time slots
	ManageAllBookings   = "manage_all_bookings"   // Manage all bookings system-wide
	ManageGroupBookings = "manage_group_bookings" // Manage group-scoped bookings

	// Action permissions
	RequestItems       = "request_items"        // Request/borrow items
	ApproveAllRequests = "approve_all_requests" // Approve high-value item requests
)

// Checkout result statuses (application-level, not persisted in DB)
const (
	CheckoutStatusCompleted       = "completed"        // LOW item successfully taken
	CheckoutStatusBorrowed        = "borrowed"         // MEDIUM item successfully borrowed
	CheckoutStatusPendingApproval = "pending_approval" // HIGH item request created
)

// Role names (match roles in db/migrations/20250626025312_seed_roles_permissions.sql)
const (
	RoleGlobalAdmin = "global_admin" // System administrator
	RoleApprover    = "approver"     // VP Operations (approves requests)
	RoleGroupAdmin  = "group_admin"  // Group-level administrator
	RoleMember      = "member"       // Regular user
)
