package handler

import (
	"log/slog"
	"net/http"

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
// @Router /lists [post]
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

	list, err := h.ListService.Create(ctx, authContext.UserID, payload.Name, payload.UserIDs)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create list", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create list")
		return
	}

	slog.InfoContext(ctx, "created list successfully", slog.Any("list_id", list.ID))
	response.JSON(ctx, w, http.StatusCreated, list)
}

// GetLists handles fetching lists for the current user
//
// @Summary Returns all lists of the user
// @Description Retrieve all lists the user is a part of
// @Tags List
// @Accept json
// @Produce json
// @Success 200 {object} []domain.List
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to read lists"
// @Router /lists [get]
func (h *ListHandler) GetLists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	lists, err := h.ListService.GetLists(ctx, authContext.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to set list item completed", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to read list items")
		return
	}

	response.JSON(ctx, w, http.StatusOK, lists)
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
// @Router /lists/user [post]
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
	response.Status(ctx, w, http.StatusNoContent)
}

// RemoveUserFromList handles the removal of a user association to a list
//
// @Summary Remove a user to the given list
// @Description Unassociate a user from a list
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.RemoveUserFromListPayload true "Remove user from list payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 500 {object} response.ErrorResponse "failed to remove user from list"
// @Router /lists/user [delete]
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
	response.Status(ctx, w, http.StatusNoContent)
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
// @Router /lists/item [post]
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

	slog.InfoContext(ctx, "created list item successfully", slog.Any("list_item_id", item.ID))
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
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to update list item"
// @Router /lists/item [put]
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
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.ListService.UpdateListItem(ctx, authContext.UserID, payload.ListItemID, payload.Title, payload.IsCompleted)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update list item", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to update list item")
		return
	}

	slog.InfoContext(ctx, "updated list item successfully", slog.Any("list_item_id", payload.ListItemID))
	response.Status(ctx, w, http.StatusNoContent)
}

// SetListItemCompleted handles updating "is_completed" of a list item
//
// @Summary Updates "is_completed" of list item
// @Description Update an existing list item, setting the "is_completed" field
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.SetListItemCompletedPayload true "Update list item is completed payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to update list item"
// @Router /lists/items/{id} [post]
func (h *ListHandler) SetListItemCompleted(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	listItemID := r.PathValue("id")
	if listItemID == "" {
		slog.ErrorContext(ctx, "failed to read list item id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	payload, err := request.DecodeJSON[request.SetListItemCompletedPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode set list item completed payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.ListService.SetListItemCompleted(ctx, authContext.UserID, listItemID, payload.IsCompleted)
	if err != nil {
		slog.ErrorContext(ctx, "failed to set list item completed", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to set list item completed")
		return
	}

	slog.InfoContext(ctx, "updated list item completed successful", slog.Any("list_item_id", listItemID))
	response.Status(ctx, w, http.StatusNoContent)
}

// GetListItems handles return all list items of a list
//
// @Summary Returns all items from a list
// @Description Retrieve all list items of a list
// @Tags List
// @Accept json
// @Produce json
// @Success 200 {object} []domain.ListItem
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to read list items"
// @Router /lists/{id} [get]
func (h *ListHandler) GetListItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	listID := r.PathValue("id")
	if listID == "" {
		slog.ErrorContext(ctx, "failed to read list id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	items, err := h.ListService.GetListItems(ctx, authContext.UserID, listID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to read list items", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to read list items")
		return
	}

	response.JSON(ctx, w, http.StatusOK, items)
}
