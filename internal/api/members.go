package api

import (
	"context"
	"sort"

	genapi "github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/USSTM/cv-backend/internal/middleware"
	"github.com/oapi-codegen/runtime/types"
)

// Returns authenticated member identity w/enough data to select and switch active group
func (s Server) GetCurrentMember(ctx context.Context, request genapi.GetCurrentMemberRequestObject) (genapi.GetCurrentMemberResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return genapi.GetCurrentMember401JSONResponse(Unauthorized("Authentication required").Create()), nil
	}

	assignments, err := s.db.Queries().GetUserRoles(ctx, &user.ID)
	if err != nil {
		middleware.GetLoggerFromContext(ctx).Error("Failed to load member roles", "user_id", user.ID, "error", err)
		return genapi.GetCurrentMember500JSONResponse(InternalError("Failed to load member roles").Create()), nil
	}

	roles := make([]genapi.MemberRole, 0, len(assignments))
	groupsByID := make(map[string]*genapi.MemberGroup)
	for _, assignment := range assignments {
		role := genapi.MemberRole{
			Name:  assignment.RoleName.String,
			Scope: genapi.MemberRoleScope(assignment.Scope),
		}
		if assignment.ScopeID != nil {
			role.GroupId = assignment.ScopeID
		}
		roles = append(roles, role)

		if assignment.ScopeID == nil {
			continue
		}
		key := assignment.ScopeID.String()
		group := groupsByID[key]
		if group == nil {
			dbGroup, err := s.db.Queries().GetGroupByID(ctx, *assignment.ScopeID)
			if err != nil {
				middleware.GetLoggerFromContext(ctx).Error("Failed to load member group", "group_id", assignment.ScopeID, "error", err)
				return genapi.GetCurrentMember500JSONResponse(InternalError("Failed to load member groups").Create()), nil
			}
			group = &genapi.MemberGroup{Id: dbGroup.ID, Name: dbGroup.Name, Roles: []string{}}
			groupsByID[key] = group
		}
		group.Roles = append(group.Roles, assignment.RoleName.String)
	}

	groups := make([]genapi.MemberGroup, 0, len(groupsByID))
	for _, group := range groupsByID {
		sort.Strings(group.Roles)
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

	return genapi.GetCurrentMember200JSONResponse(genapi.CurrentMember{
		Id:     user.ID,
		Email:  types.Email(user.Email),
		Roles:  roles,
		Groups: groups,
	}), nil
}
