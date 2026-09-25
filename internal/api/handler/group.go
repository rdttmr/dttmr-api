package handler

import (
	"log/slog"
	"net/http"

	"git.dittmar.dev/robin/dttmr-api/internal/api/request"
	"git.dittmar.dev/robin/dttmr-api/internal/api/response"
	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

type GroupHandler struct {
	GroupService *domain.GroupService
}

func NewGroupHandler(groupService *domain.GroupService) *GroupHandler {
	return &GroupHandler{GroupService: groupService}
}

// CreateGroup handles the creation of a group
//
// @Summary Create group route
// @Description Create a group and associate the authenticated user to it
// @Tags Group
// @Accept json
// @Produce json
// @Param payload body request.CreateGroupPayload true "Create group payload"
// @Success 201 {object} domain.Group
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to create group"
// @Router /groups [post]
func (h *GroupHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.CreateGroupPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode create group payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create group")
		return
	}

	group, err := h.GroupService.CreateGroup(ctx, authContext.UserID, payload.Name)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create group", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create group")
		return
	}

	slog.InfoContext(ctx, "created group successfully", slog.String("group_id", group.ID))
	response.JSON(ctx, w, http.StatusCreated, group)

}

// DeleteGroup handles the deletion of a group
//
// @Summary Delete group route
// @Description Deletes a group, cascading to user associations
// @Tags Group
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 204
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to delete group"
// @Router /groups/{id} [delete]
func (h *GroupHandler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	groupID := r.PathValue("id")
	if groupID == "" {
		slog.ErrorContext(ctx, "failed to read group id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete list")
		return
	}

	err = h.GroupService.DeleteGroup(ctx, authContext.UserID, groupID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete group", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete group")
		return
	}

	slog.InfoContext(ctx, "deleted group successfully", slog.String("group_id", groupID))
	response.Status(w, http.StatusNoContent)

}

// SetGroupName handles updating "name" of a group
//
// @Summary Updates "name" of group
// @Description Update an existing group, setting the "name" field
// @Tags Group
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param payload body request.SetGroupNamePayload true "Update group name payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to set group name"
// @Router /groups/{id}/name [post]
func (h *GroupHandler) SetGroupName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	groupID := r.PathValue("id")
	if groupID == "" {
		slog.ErrorContext(ctx, "failed to read group id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	payload, err := request.DecodeJSON[request.SetGroupNamePayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode set group name payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.GroupService.SetGroupName(ctx, authContext.UserID, groupID, payload.Name)
	if err != nil {
		slog.ErrorContext(ctx, "failed to set group name", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to set group name")
		return
	}

	slog.InfoContext(ctx, "updated group name successfully", slog.String("group_id", groupID))
	response.Status(w, http.StatusNoContent)
}

// GetGroups handles fetching groups for the current user
//
// @Summary Returns all groups of the user
// @Description Retrieve all groups the user is a part of
// @Tags Group
// @Accept json
// @Produce json
// @Success 200 {object} []domain.Group
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to read groups"
// @Router /groups [get]
func (h *GroupHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	lists, err := h.GroupService.GetGroups(ctx, authContext.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get groups", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to read groups")
		return
	}

	response.JSON(ctx, w, http.StatusOK, lists)

}

// GetGroupMembers handles return all members of a group
//
// @Summary Returns all members from a group
// @Description Retrieve all members of a group
// @Tags Group
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} []domain.User
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to read group members"
// @Router /groups/{id}/members [get]
func (h *GroupHandler) GetGroupMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	groupID := r.PathValue("id")
	if groupID == "" {
		slog.ErrorContext(ctx, "failed to read group id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	members, err := h.GroupService.GetGroupMembers(ctx, authContext.UserID, groupID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to read group members", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to read group members")
		return
	}

	response.JSON(ctx, w, http.StatusOK, members)

}

// ShareGroup handles the creation of a group invite
//
// @Summary Share group route
// @Description Share a group, by creating an invite for it.
// @Tags Group
// @Accept json
// @Produce json
// @Param id path string true "Group ID"
// @Success 200 {object} domain.GroupInvite
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to share group"
// @Router /groups/{id}/share [post]
func (h *GroupHandler) ShareGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	groupID := r.PathValue("id")
	if groupID == "" {
		slog.ErrorContext(ctx, "failed to read group id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create recipe")
		return
	}

	invite, err := h.GroupService.ShareGroup(ctx, authContext.UserID, groupID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create group invite", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to share group")
		return
	}

	slog.InfoContext(ctx, "shared recipe successfully", slog.String("recipe_id", groupID))
	response.JSON(ctx, w, http.StatusOK, invite)
}

// JoinGroup handles joining a group
//
// @Summary Join a group route
// @Description Join a group, by using the shared code.
// @Tags Group
// @Accept json
// @Produce json
// @Param code path string true "Share code"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to join group"
// @Router /groups/join/{code} [post]
func (h *GroupHandler) JoinGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	code := r.PathValue("code")
	if code == "" {
		slog.ErrorContext(ctx, "failed to read code from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create recipe")
		return
	}

	groupID, err := h.GroupService.JoinGroup(ctx, authContext.UserID, code)
	if err != nil {
		slog.ErrorContext(ctx, "failed to join group by share code",
			slog.Any("error", err),
			slog.String("code", code))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to join group")
		return
	}

	slog.InfoContext(ctx, "joined group successfully",
		slog.String("user_id", authContext.UserID),
		slog.String("group_id", groupID))
	response.Status(w, http.StatusNoContent)
}

func (h *GroupHandler) LeaveGroup(w http.ResponseWriter, r *http.Request) {

}

// SetDefaultGroup handles setting a group your default
//
// @Summary Set default group route
// @Description Set group as default.
// @Tags Group
// @Accept json
// @Produce json
// @Param code path string true "Group ID"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to set default group"
// @Router /groups/{id}/default [post]
func (h *GroupHandler) SetDefaultGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	groupID := r.PathValue("id")
	if groupID == "" {
		slog.ErrorContext(ctx, "failed to read group id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create recipe")
		return
	}

	err := h.GroupService.SetDefaultGroupID(ctx, authContext.UserID, groupID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to set default group", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to set default group")
		return
	}

	slog.InfoContext(ctx, "set default group successfully",
		slog.String("user_id", authContext.UserID),
		slog.String("group_id", groupID))
	response.Status(w, http.StatusNoContent)
}
