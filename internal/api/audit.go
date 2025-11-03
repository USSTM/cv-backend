package api

import (
	"context"
	"time"

	"github.com/USSTM/cv-backend/generated/api"
	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (s Server) GetUserTakingHistory(ctx context.Context, request api.GetUserTakingHistoryRequestObject) (api.GetUserTakingHistoryResponseObject, error) {
	authenticatedUser, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetUserTakingHistory401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	targetUserID := request.UserId
	groupIDFilter := request.Params.GroupId

	// Check access permissions
	canView, err := s.canViewUserTakingHistory(ctx, authenticatedUser.ID, targetUserID, groupIDFilter)
	if err != nil {
		return api.GetUserTakingHistory500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !canView {
		return api.GetUserTakingHistory403JSONResponse{Code: 403, Message: "Insufficient permissions to view this user's data"}, nil
	}

	// basic pagination attempt
	limit := int64(50)
	offset := int64(0)
	if request.Params.Limit != nil {
		limit = int64(*request.Params.Limit)
	}
	if request.Params.Offset != nil {
		offset = int64(*request.Params.Offset)
	}

	var response []api.TakingHistoryResponse

	// If group is provided, use filtered query
	if groupIDFilter != nil {
		filteredTakings, err := s.db.Queries().GetTakingHistoryByUserIdWithGroupFilter(ctx, db.GetTakingHistoryByUserIdWithGroupFilterParams{
			UserID:  targetUserID,
			GroupID: *groupIDFilter,
			Limit:   limit,
			Offset:  offset,
		})
		if err != nil {
			return api.GetUserTakingHistory500JSONResponse{Code: 500, Message: "Failed to get history"}, nil
		}
		for _, taking := range filteredTakings {
			response = append(response, api.TakingHistoryResponse{
				Id:       taking.ID,
				UserId:   taking.UserID,
				GroupId:  taking.GroupID,
				ItemId:   taking.ItemID,
				ItemName: taking.Name,
				Quantity: int(taking.Quantity),
				TakenAt:  taking.TakenAt.Time,
			})
		}
	} else {
		takings, err := s.db.Queries().GetTakingHistoryByUserId(ctx, db.GetTakingHistoryByUserIdParams{
			UserID: targetUserID,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return api.GetUserTakingHistory500JSONResponse{Code: 500, Message: "Failed to get history"}, nil
		}
		for _, taking := range takings {
			response = append(response, api.TakingHistoryResponse{
				Id:       taking.ID,
				UserId:   taking.UserID,
				GroupId:  taking.GroupID,
				ItemId:   taking.ItemID,
				ItemName: taking.Name,
				Quantity: int(taking.Quantity),
				TakenAt:  taking.TakenAt.Time,
			})
		}
	}

	// no response, empty array
	if len(response) == 0 {
		return api.GetUserTakingHistory200JSONResponse([]api.TakingHistoryResponse{}), nil
	}

	// else, return response
	return api.GetUserTakingHistory200JSONResponse(response), nil
}

// determine if authenticated user can view target user's taking history
func (s Server) canViewUserTakingHistory(ctx context.Context, authenticatedUserID, targetUserID uuid.UUID, groupIDFilter *uuid.UUID) (bool, error) {
	// Case 1: User viewing their own data
	if authenticatedUserID == targetUserID {
		hasPermission, err := s.authenticator.CheckPermission(ctx, authenticatedUserID, "view_own_data", nil)
		if err != nil {
			return false, err
		}
		return hasPermission, nil
	}

	// Case 2: User has global view_all_data permission
	hasGlobalPermission, err := s.authenticator.CheckPermission(ctx, authenticatedUserID, "view_all_data", nil)
	if err != nil {
		return false, err
	}
	if hasGlobalPermission {
		return true, nil
	}

	// Case 3: Group admin with view_group_data permission
	// Requires groupId parameter to specify which group's data to view
	if groupIDFilter != nil {
		hasGroupPermission, err := s.authenticator.CheckPermission(ctx, authenticatedUserID, "view_group_data", groupIDFilter)
		if err != nil {
			return false, err
		}
		return hasGroupPermission, nil
	}

	// Group admins must specify groupId parameter to view other users' data
	return false, nil
}

// admin only handler
func (s Server) GetItemTakingHistory(ctx context.Context, request api.GetItemTakingHistoryRequestObject) (api.GetItemTakingHistoryResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetItemTakingHistory401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_all_data", nil)
	if err != nil {
		return api.GetItemTakingHistory500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !hasPermission {
		return api.GetItemTakingHistory403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	// basic pagination
	limit := int64(50)
	offset := int64(0)
	if request.Params.Limit != nil {
		limit = int64(*request.Params.Limit)
	}
	if request.Params.Offset != nil {
		offset = int64(*request.Params.Offset)
	}

	takings, err := s.db.Queries().GetTakingHistoryByItemId(ctx, db.GetTakingHistoryByItemIdParams{
		ItemID: request.ItemId,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return api.GetItemTakingHistory500JSONResponse{Code: 500, Message: "Failed to get history"}, nil
	}

	var response []api.ItemTakingHistoryResponse
	for _, taking := range takings {
		response = append(response, api.ItemTakingHistoryResponse{
			Id:        taking.ID,
			UserId:    taking.UserID,
			UserEmail: openapi_types.Email(taking.UserEmail),
			GroupId:   taking.GroupID,
			ItemId:    taking.ItemID,
			Quantity:  int(taking.Quantity),
			TakenAt:   taking.TakenAt.Time,
		})
	}

	if len(response) == 0 {
		return api.GetItemTakingHistory200JSONResponse([]api.ItemTakingHistoryResponse{}), nil
	}

	return api.GetItemTakingHistory200JSONResponse(response), nil
}

// admin only handler
func (s Server) GetItemTakingStats(ctx context.Context, request api.GetItemTakingStatsRequestObject) (api.GetItemTakingStatsResponseObject, error) {
	user, ok := auth.GetAuthenticatedUser(ctx)
	if !ok {
		return api.GetItemTakingStats401JSONResponse{Code: 401, Message: "Unauthorized"}, nil
	}

	hasPermission, err := s.authenticator.CheckPermission(ctx, user.ID, "view_all_data", nil)
	if err != nil {
		return api.GetItemTakingStats500JSONResponse{Code: 500, Message: "Internal server error"}, nil
	}
	if !hasPermission {
		return api.GetItemTakingStats403JSONResponse{Code: 403, Message: "Insufficient permissions"}, nil
	}

	minDate := time.Unix(0, 0)                            // 1970-01-01
	maxDate := time.Now().Add(100 * 365 * 24 * time.Hour) // ~100 years from current time

	startDate := pgtype.Timestamp{Time: minDate, Valid: true}
	endDate := pgtype.Timestamp{Time: maxDate, Valid: true}

	if !request.Params.StartDate.IsZero() {
		startDate = pgtype.Timestamp{Time: request.Params.StartDate, Valid: true}
	}

	if !request.Params.EndDate.IsZero() {
		endDate = pgtype.Timestamp{Time: request.Params.EndDate, Valid: true}
	}

	stats, err := s.db.Queries().GetTakingStats(ctx, db.GetTakingStatsParams{
		ItemID:    request.ItemId,
		TakenAt:   startDate,
		TakenAt_2: endDate,
	})
	if err != nil {
		return api.GetItemTakingStats500JSONResponse{Code: 500, Message: "Failed to get stats"}, nil
	}

	// Convert pgtype types to request types
	var totalQuantity int64
	if stats.TotalQuantity.Valid {
		totalQuantity = stats.TotalQuantity.Int64
	}

	var firstTaking, lastTaking *time.Time
	if stats.FirstTaking.Valid {
		firstTaking = &stats.FirstTaking.Time
	}
	if stats.LastTaking.Valid {
		lastTaking = &stats.LastTaking.Time
	}

	return api.GetItemTakingStats200JSONResponse{
		ItemId:        request.ItemId,
		TotalTakings:  int(stats.TotalTakings),
		TotalQuantity: int(totalQuantity),
		UniqueUsers:   int(stats.UniqueUsers),
		FirstTaking:   firstTaking,
		LastTaking:    lastTaking,
	}, nil
}
