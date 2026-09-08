package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/USSTM/cv-backend/internal/queue"
	"github.com/USSTM/cv-backend/internal/rbac"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (s Server) GetUsers(ctx context.Context, request api.GetUsersRequestObject) (api.GetUsersResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	// Check permission
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUsers401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageUsers, nil)
	if err != nil {
		logger.Error("Error checking manage_users permission",
			"user_id", user.ID,
			"permission", rbac.ManageUsers,
			"error", err)
		return api.GetUsers500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetUsers403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	users, err := s.db.Queries().GetAllUsers(ctx)
	if err != nil {
		logger.Error("Failed to get users", "error", err)
		return api.GetUsers500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	// Convert database users to API response format
	var response api.GetUsers200JSONResponse
	for _, user := range users {
		userUUID := user.ID

		// Get user roles from database
		roles, err := s.db.Queries().GetUserRoles(ctx, &user.ID)
		if err != nil {
			logger.Error("Failed to get user roles",
				"user_id", user.ID,
				"error", err)
		}

		// Default to member role, upgrade if user has higher roles
		role := api.Member
		for _, userRole := range roles {
			switch userRole.RoleName.String {
			case "global_admin":
				role = api.Admin
			case "approver":
				role = api.Approver
			case "group_admin":
				role = api.GroupAdmin
			}
		}

		userResponse := api.User{
			Id:    userUUID,
			Email: types.Email(user.Email),
			Role:  role,
		}
		response = append(response, userResponse)
	}

	return response, nil
}

func (s Server) InviteUser(ctx context.Context, request api.InviteUserRequestObject) (api.InviteUserResponseObject, error) {
	if request.Body == nil {
		return api.InviteUser400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
	}
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.InviteUser401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	req := request.Body
	scopeStr := string(req.Scope)
	if !isInvitableRole(req.RoleName) || (scopeStr != "global" && scopeStr != "group") {
		return api.InviteUser400JSONResponse(ValidationErr("role_name and scope must be valid", nil).Create()), nil
	}

	var scopeID uuid.UUID
	if req.ScopeId != nil {
		scopeID = *req.ScopeId
	}

	if scopeStr == "global" && scopeID != uuid.Nil {
		return api.InviteUser400JSONResponse(ValidationErr("Scope ID must be empty for global scope", nil).Create()), nil
	}

	if scopeStr == "group" {
		if scopeID == uuid.Nil {
			return api.InviteUser400JSONResponse(ValidationErr("Scope ID must be provided for group scope", nil).Create()), nil
		}

		// check if the group exists with error handling, if it does exist, then do nothing
		_, err := s.db.Queries().GetGroupByID(ctx, scopeID)
		if err != nil {
			return api.InviteUser404JSONResponse(NotFound("Group").Create()), nil
		}
	}

	// Only global administrators may invite elevated roles or create global
	// invitations. Group admins are limited to adding regular members to a group
	// they administer.
	isGlobalAdmin, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageUsers, nil)
	if err != nil {
		return api.InviteUser500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !isGlobalAdmin {
		if scopeStr != "group" || req.RoleName != rbac.RoleMember {
			return api.InviteUser403JSONResponse(PermissionDenied("Only global administrators may invite this role or scope").Create()), nil
		}
		hasGroupPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroupUsers, &scopeID)
		if err != nil {
			return api.InviteUser500JSONResponse(InternalError("Internal server error").Create()), nil
		}
		if !hasGroupPermission {
			return api.InviteUser403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
		}
	}

	// generates a random code for the sign-up link (just a random string of 32 characters)
	code, err := generateRandomCode(32)
	if err != nil {
		return api.InviteUser500JSONResponse(InternalError("Failed to generate sign-up code").Create()), nil
	}

	// this makes it so that if the scopeID is uuid.Nil, it will be nil in the database (instead of 0000000-0000-0000-0000-000000000000)
	var scopeIDPtr *uuid.UUID
	if scopeID != uuid.Nil {
		scopeIDPtr = &scopeID
	} else {
		scopeIDPtr = nil
	}

	params := db.CreateSignUpCodeParams{
		Code:      code,
		Email:     string(req.Email),
		RoleName:  req.RoleName,
		Scope:     db.ScopeType(scopeStr),
		ScopeID:   scopeIDPtr,
		CreatedBy: user.ID,
	}

	signupCode, err := s.db.Queries().CreateSignUpCode(ctx, params)
	if err != nil {
		return api.InviteUser500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}
	if _, err = s.queue.Enqueue(queue.TypeEmailDelivery, queue.EmailDeliveryPayload{
		To:      string(req.Email),
		Subject: "Your Campus Vault invitation",
		Body: fmt.Sprintf(
			"You have been invited to join Campus Vault.\n\nYour invitation code is: %s\n\nAccept it using the invitation acceptance page, then request a one-time login code using this email address. This invitation expires in 7 days.",
			signupCode.Code,
		),
	}); err != nil {
		return api.InviteUser500JSONResponse(InternalError("Failed to send invitation email").Create()), nil
	}

	return api.InviteUser201JSONResponse{Code: &signupCode.Code}, nil
}

func isInvitableRole(role string) bool {
	switch role {
	case rbac.RoleGlobalAdmin, rbac.RoleApprover, rbac.RoleGroupAdmin, rbac.RoleMember:
		return true
	default:
		return false
	}
}

func (s Server) GetUsersByGroup(ctx context.Context, request api.GetUsersByGroupRequestObject) (api.GetUsersByGroupResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUsersByGroup401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageGroupUsers, &request.GroupId)
	if err != nil {
		logger.Error("Error checking manage_group_users permission", "error", err)
		return api.GetUsersByGroup500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetUsersByGroup403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	_, err = s.db.Queries().GetGroupByID(ctx, request.GroupId)
	if err != nil {
		return api.GetUsersByGroup404JSONResponse(NotFound("Group").Create()), nil
	}

	users, err := s.db.Queries().GetUsersByGroup(ctx, &request.GroupId)
	if err != nil {
		logger.Error("Failed to get users by group", "error", err)
		return api.GetUsersByGroup500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}

	if len(users) == 0 {
		return api.GetUsersByGroup404JSONResponse(
			NewError(CodeResourceNotFound, "No users found in the specified group").Create(),
		), nil
	}

	var response api.GetUsersByGroup200JSONResponse
	for _, user := range users {
		groupUser := api.GroupUser{
			Id:       user.ID,
			Email:    types.Email(user.Email),
			RoleName: user.RoleName.String,
			Scope:    string(user.Scope),
		}

		if user.ScopeID != nil {
			groupUser.ScopeId = user.ScopeID
		}

		response = append(response, groupUser)
	}

	return response, nil
}

func (s Server) GetUserRoleAssignments(ctx context.Context, request api.GetUserRoleAssignmentsRequestObject) (api.GetUserRoleAssignmentsResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)
	actor, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUserRoleAssignments401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}
	allowed, err := s.authenticator.CheckPermission(ctx, actor.ID, rbac.ManageUsers, nil)
	if err != nil {
		logger.Error("Failed to authorize role assignment lookup", "actor_id", actor.ID, "error", err)
		return api.GetUserRoleAssignments500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !allowed {
		return api.GetUserRoleAssignments403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}
	if _, err = s.db.Queries().GetUserByID(ctx, request.UserId); err != nil {
		return api.GetUserRoleAssignments404JSONResponse(NotFound("User").Create()), nil
	}

	roles, err := s.db.Queries().GetUserRoles(ctx, &request.UserId)
	if err != nil {
		logger.Error("Failed to load member role assignments", "user_id", request.UserId, "error", err)
		return api.GetUserRoleAssignments500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	response := make(api.GetUserRoleAssignments200JSONResponse, 0, len(roles))
	for _, role := range roles {
		response = append(response, api.UserRoleAssignment{
			RoleName: role.RoleName.String,
			Scope:    string(role.Scope),
			ScopeId:  role.ScopeID,
		})
	}
	return response, nil
}

func (s Server) GetUserById(ctx context.Context, request api.GetUserByIdRequestObject) (api.GetUserByIdResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	// Check authentication
	currentUser, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUserById401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	// Users can view their own data, or admins can view any user
	canView := currentUser.ID == request.UserId
	if !canView {
		hasPermission, err := s.authenticator.CheckPermission(ctx, currentUser.ID, rbac.ManageUsers, nil)
		if err != nil {
			logger.Error("Error checking manage_users permission", "error", err)
			return api.GetUserById500JSONResponse(InternalError("Internal server error").Create()), nil
		}
		canView = hasPermission
	}

	if !canView {
		return api.GetUserById403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	user, err := s.db.Queries().GetUserByID(ctx, request.UserId)
	if err != nil {
		return api.GetUserById404JSONResponse(NotFound("User").Create()), nil
	}

	roles, err := s.db.Queries().GetUserRoles(ctx, &user.ID)
	if err != nil {
		logger.Error("Failed to get user roles", "error", err)
	}

	userResponse := api.User{
		Id:    user.ID,
		Email: types.Email(user.Email),
		Role:  GetUserRole(roles),
	}

	return api.GetUserById200JSONResponse(userResponse), nil
}

func (s Server) GetUserByEmail(ctx context.Context, request api.GetUserByEmailRequestObject) (api.GetUserByEmailResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)

	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUserByEmail401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, rbac.ManageUsers, nil)
	if err != nil {
		logger.Error("Error checking manage_users permission", "error", err)
		return api.GetUserByEmail500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !hasPermission {
		return api.GetUserByEmail403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}

	foundUser, err := s.db.Queries().GetUserByEmail(ctx, string(request.Email))
	if err != nil {
		return api.GetUserByEmail404JSONResponse(NotFound("User").Create()), nil
	}

	roles, err := s.db.Queries().GetUserRoles(ctx, &foundUser.ID)
	if err != nil {
		logger.Error("Failed to get user roles", "error", err)
	}

	userResponse := api.User{
		Id:    foundUser.ID,
		Email: types.Email(foundUser.Email),
		Role:  GetUserRole(roles),
	}

	return api.GetUserByEmail200JSONResponse(userResponse), nil
}

func (s Server) UpdateUser(ctx context.Context, request api.UpdateUserRequestObject) (api.UpdateUserResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)
	actor, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.UpdateUser401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}
	if request.Body == nil {
		return api.UpdateUser400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
	}

	allowed, err := s.authenticator.CheckPermission(ctx, actor.ID, rbac.ManageUsers, nil)
	if err != nil {
		logger.Error("Failed to authorize member update", "actor_id", actor.ID, "member_id", request.UserId, "error", err)
		return api.UpdateUser500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !allowed {
		return api.UpdateUser403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}
	member, err := s.db.Queries().GetUserByID(ctx, request.UserId)
	if err != nil {
		return api.UpdateUser404JSONResponse(NotFound("User").Create()), nil
	}

	for _, assignment := range []api.UserRoleAssignment{request.Body.Current, request.Body.Replacement} {
		if !isAssignableRole(assignment.RoleName) || (assignment.Scope != "global" && assignment.Scope != "group") {
			return api.UpdateUser400JSONResponse(ValidationErr("Each role must have a valid role_name and scope", nil).Create()), nil
		}
		if assignment.Scope == "global" && assignment.ScopeId != nil {
			return api.UpdateUser400JSONResponse(ValidationErr("Global roles must not have a scope_id", nil).Create()), nil
		}
		if assignment.Scope == "group" {
			if assignment.ScopeId == nil {
				return api.UpdateUser400JSONResponse(ValidationErr("Group roles require a scope_id", nil).Create()), nil
			}
			if _, err := s.db.Queries().GetGroupByID(ctx, *assignment.ScopeId); err != nil {
				return api.UpdateUser400JSONResponse(ValidationErr("scope_id must reference an existing group", nil).Create()), nil
			}
		}
	}
	if actor.ID == request.UserId && isGlobalAdminAssignment(request.Body.Current) && !isGlobalAdminAssignment(request.Body.Replacement) {
		return api.UpdateUser403JSONResponse(PermissionDenied("Administrators cannot remove their own global admin permission").Create()), nil
	}

	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return api.UpdateUser500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	defer tx.Rollback(ctx)
	deleteResult, err := tx.Exec(ctx, `
		DELETE FROM user_roles
		WHERE user_id = $1
		  AND role_name = $2
		  AND scope = $3
		  AND scope_id IS NOT DISTINCT FROM $4`,
		request.UserId,
		request.Body.Current.RoleName,
		request.Body.Current.Scope,
		request.Body.Current.ScopeId,
	)
	if err != nil {
		logger.Error("Failed to update member role", "member_id", request.UserId, "error", err)
		return api.UpdateUser500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}
	if deleteResult.RowsAffected() == 0 {
		return api.UpdateUser404JSONResponse(NotFound("Role assignment").Create()), nil
	}
	if _, err = tx.Exec(ctx, "INSERT INTO user_roles (user_id, role_name, scope, scope_id) VALUES ($1, $2, $3, $4)", request.UserId, request.Body.Replacement.RoleName, request.Body.Replacement.Scope, request.Body.Replacement.ScopeId); err != nil {
		logger.Error("Failed to create updated member role", "member_id", request.UserId, "error", err)
		return api.UpdateUser500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}
	if err = tx.Commit(ctx); err != nil {
		logger.Error("Failed to commit member role update", "member_id", request.UserId, "error", err)
		return api.UpdateUser500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}
	roles, _ := s.db.Queries().GetUserRoles(ctx, &member.ID)
	return api.UpdateUser200JSONResponse(api.User{Id: member.ID, Email: types.Email(member.Email), Role: GetUserRole(roles)}), nil
}

func isAssignableRole(role string) bool {
	switch role {
	case rbac.RoleGlobalAdmin, rbac.RoleApprover, rbac.RoleGroupAdmin, rbac.RoleMember:
		return true
	default:
		return false
	}
}

func isGlobalAdminAssignment(role api.UserRoleAssignment) bool {
	return role.RoleName == rbac.RoleGlobalAdmin && role.Scope == "global" && role.ScopeId == nil
}

func (s Server) DeleteUser(ctx context.Context, request api.DeleteUserRequestObject) (api.DeleteUserResponseObject, error) {
	logger := middleware.GetLoggerFromContext(ctx)
	actor, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.DeleteUser401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	allowed, err := s.authenticator.CheckPermission(ctx, actor.ID, rbac.ManageUsers, nil)
	if err != nil {
		logger.Error("Failed to authorize member deletion", "actor_id", actor.ID, "member_id", request.UserId, "error", err)
		return api.DeleteUser500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !allowed {
		return api.DeleteUser403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}
	if actor.ID == request.UserId {
		return api.DeleteUser403JSONResponse(PermissionDenied("Administrators cannot delete their own account").Create()), nil
	}
	if _, err := s.db.Queries().GetUserByID(ctx, request.UserId); err != nil {
		return api.DeleteUser404JSONResponse(NotFound("User").Create()), nil
	}

	tx, err := s.db.Pool().Begin(ctx)
	if err != nil {
		return api.DeleteUser500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	defer tx.Rollback(ctx)
	// signup_codes.created_by intentionally does not cascade; remove codes created
	// by this member so deleting the member cannot violate that foreign key.
	if _, err = tx.Exec(ctx, "DELETE FROM signup_codes WHERE created_by = $1", request.UserId); err == nil {
		_, err = tx.Exec(ctx, "DELETE FROM users WHERE id = $1", request.UserId)
	}
	if err != nil {
		logger.Error("Failed to delete member", "member_id", request.UserId, "error", err)
		return api.DeleteUser500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}
	if err = tx.Commit(ctx); err != nil {
		return api.DeleteUser500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	return api.DeleteUser204Response{}, nil
}

func (s Server) UpdateUserGroupMembership(ctx context.Context, request api.UpdateUserGroupMembershipRequestObject) (api.UpdateUserGroupMembershipResponseObject, error) {
	actor, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.UpdateUserGroupMembership401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}
	if request.Body == nil {
		return api.UpdateUserGroupMembership400JSONResponse(ValidationErr("Request body is required", nil).Create()), nil
	}
	allowed, err := s.authenticator.CheckPermission(ctx, actor.ID, rbac.ManageGroupUsers, &request.GroupId)
	if err != nil {
		return api.UpdateUserGroupMembership500JSONResponse(InternalError("Internal server error").Create()), nil
	}
	if !allowed {
		return api.UpdateUserGroupMembership403JSONResponse(PermissionDenied("Insufficient permissions").Create()), nil
	}
	if actor.ID == request.UserId && !request.Body.IsMember {
		return api.UpdateUserGroupMembership403JSONResponse(PermissionDenied("Administrators cannot remove their own group membership").Create()), nil
	}
	if _, err := s.db.Queries().GetUserByID(ctx, request.UserId); err != nil {
		return api.UpdateUserGroupMembership404JSONResponse(NotFound("User").Create()), nil
	}
	if _, err := s.db.Queries().GetGroupByID(ctx, request.GroupId); err != nil {
		return api.UpdateUserGroupMembership404JSONResponse(NotFound("Group").Create()), nil
	}
	if request.Body.IsMember {
		_, err = s.db.Pool().Exec(ctx, "INSERT INTO user_roles (user_id, role_name, scope, scope_id) VALUES ($1, 'member', 'group', $2) ON CONFLICT DO NOTHING", request.UserId, request.GroupId)
	} else {
		_, err = s.db.Pool().Exec(ctx, "DELETE FROM user_roles WHERE user_id = $1 AND scope = 'group' AND scope_id = $2", request.UserId, request.GroupId)
	}
	if err != nil {
		return api.UpdateUserGroupMembership500JSONResponse(InternalError("An unexpected error occurred.").Create()), nil
	}
	return api.UpdateUserGroupMembership204Response{}, nil
}

func generateRandomCode(length int) (string, error) {
	const characters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	if length <= 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.Grow(length)

	for i := 0; i < length; i++ {
		b := make([]byte, 1)
		_, err := rand.Read(b)
		if err != nil {
			return "", err
		}
		sb.WriteByte(characters[int(b[0])%len(characters)])
	}

	return sb.String(), nil
}

func GetUserRole(roles []db.GetUserRolesRow) api.UserRole {
	role := api.Member
	for _, userRole := range roles {
		switch userRole.RoleName.String {
		case "global_admin":
			role = api.Admin
		case "approver":
			role = api.Approver
		case "group_admin":
			role = api.GroupAdmin
		}
	}

	return role
}
