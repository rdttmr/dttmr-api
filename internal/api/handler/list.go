package handler

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/robindittmar/dttmr-api/internal/api/request"
	"github.com/robindittmar/dttmr-api/internal/api/response"
	"github.com/robindittmar/dttmr-api/internal/domain"
)

type ListHandler struct {
	ListService *domain.ListService
}

func NewListHandler(listService *domain.ListService) *ListHandler {
	return &ListHandler{ListService: listService}
}

// CreateList handles the creation of a list
//
// @Summary Create list route
// @Description Create a list and associate user(s) to it
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.CreateListPayload true "Create list payload"
// @Success 201 {object} domain.List
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to create list"
// @Router /api/v1/lists [post]
func (h *ListHandler) CreateList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.CreateListPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode create list payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create list")
		return
	}

	// TODO: should this be in ListService?
	if !slices.Contains(payload.UserIDs, authContext.UserID) {
		payload.UserIDs = append(payload.UserIDs, authContext.UserID)
	}

	list, err := h.ListService.Create(ctx, payload.Name, payload.UserIDs)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create list", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create list")
		return
	}

	slog.InfoContext(ctx, "created list successfully", slog.Any("list_id", list.ID))
	response.JSON(ctx, w, http.StatusCreated, list)
}

// AddUserToList handles the user association to a list
//
// @Summary Add a user to the given list
// @Description Associate a user with a list
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.AddUserToListPayload true "Add user to list payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to add user to list"
// @Router /api/v1/lists/user [post]
func (h *ListHandler) AddUserToList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.AddUserToListPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode add user to list payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to add user to list")
		return
	}

	err = h.ListService.AddUserToList(ctx, authContext.UserID, payload.ListID, payload.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to add user to list", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to add user to list")
		return
	}

	slog.InfoContext(ctx, "added user to list successfully", slog.Any("list_id", payload.ListID), slog.Any("user_id", payload.UserID))
	response.JSON(ctx, w, http.StatusNoContent, nil)
}

// RemoveUserFromList handles the removal of a user association to a list
//
// @Summary Remove a user to the given list
// @Description Unassociate a user from a list
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.RemoveUserFromList true "Remove user from list payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to remove user from list"
// @Router /api/v1/lists/user [delete]
func (h *ListHandler) RemoveUserFromList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.RemoveUserFromListPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode remove user from list payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to remove user from list")
		return
	}

	err = h.ListService.RemoveUserFromList(ctx, authContext.UserID, payload.ListID, payload.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to remove user from list", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to remove user from list")
		return
	}

	slog.InfoContext(ctx, "removed user from list successfully", slog.Any("list_id", payload.ListID), slog.Any("user_id", payload.UserID))
	response.JSON(ctx, w, http.StatusNoContent, nil)
}

// CreateListItem handles creation of a new list item on a given list
//
// @Summary Create list item
// @Description Create a new list item on a given list
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.CreateListItemPayload true "Create list item payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to create list item"
// @Router /api/v1/lists/item [post]
func (h *ListHandler) CreateListItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.CreateListItemPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode create list item payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create list item")
		return
	}

	item, err := h.ListService.CreateListItem(ctx, authContext.UserID, payload.ListID, payload.Title)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create list item", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create list item")
		return
	}

	response.JSON(ctx, w, http.StatusCreated, item)
}

// UpdateListItem handles updating of a list item
//
// @Summary Update list item
// @Description Update an existing list item
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.UpdateListItemPayload true "Update list item payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to update list item"
// @Router /api/v1/lists/item [put]
func (h *ListHandler) UpdateListItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.UpdateListItemPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode update list item payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to update list item")
		return
	}

	err = h.ListService.UpdateListItem(ctx, payload.ListItemID, authContext.UserID, payload.Title, payload.IsCompleted)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update list item", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to update list item")
		return
	}

	response.JSON(ctx, w, http.StatusNoContent, nil)
}
