package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"git.dittmar.dev/robin/dttmr-api/internal/api/request"
	"git.dittmar.dev/robin/dttmr-api/internal/api/response"
	"git.dittmar.dev/robin/dttmr-api/internal/domain"
)

var (
	ErrNoAccess = errors.New("no access")
)

type ListHandler struct {
	ListService  *domain.ListService
	UserService  *domain.UserService
	GroupService *domain.GroupService
}

func NewListHandler(listService *domain.ListService, userService *domain.UserService, groupService *domain.GroupService) *ListHandler {
	return &ListHandler{ListService: listService, UserService: userService, GroupService: groupService}
}

func (h *ListHandler) ValidateGroupWritePermission(ctx context.Context, w http.ResponseWriter, action string, groupID *string, userID string) error {
	var err error
	if *groupID == "" {
		*groupID, err = h.GroupService.GetDefaultGroupID(ctx, userID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get default group id", slog.Any("error", err))
			response.Error(ctx, w, http.StatusInternalServerError, fmt.Sprintf("failed to %s", action))
			return err
		}
	} else {
		err := h.GroupService.UserHasWritePermission(ctx, userID, *groupID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNoWritePermissions) {
				slog.ErrorContext(ctx, "user has no permission to write",
					slog.String("action", action),
					slog.String("user_id", userID),
					slog.String("group_id", *groupID))
				response.Error(ctx, w, http.StatusForbidden, "user has no write permission")
			} else {
				slog.ErrorContext(ctx, "failed to get users role", slog.Any("error", err))
				response.Error(ctx, w, http.StatusInternalServerError, fmt.Sprintf("failed to %s", action))
			}

			return err
		}
	}

	return nil
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

	list, err := h.ListService.CreateList(ctx, authContext.UserID, payload.GroupID, payload.Name)
	if err != nil {
		if errors.Is(err, domain.ErrUserNoWritePermissions) {
			slog.WarnContext(ctx, "user has no write permission",
				slog.Any("error", err),
				slog.String("user_id", authContext.UserID),
				slog.String("group_id", payload.GroupID))
			response.Error(ctx, w, http.StatusForbidden, "user has no write permission")
		} else {
			slog.ErrorContext(ctx, "failed to create list", slog.Any("error", err))
			response.Error(ctx, w, http.StatusInternalServerError, "failed to create list")
		}
		return
	}

	slog.InfoContext(ctx, "created list successfully", slog.String("list_id", list.ID))
	response.JSON(ctx, w, http.StatusCreated, list)
}

// DeleteList handles the deletion of a list
//
// @Summary Delete list route
// @Description Deletes a list, cascading to user associations and items
// @Tags List
// @Accept json
// @Produce json
// @Param id path int true "List ID"
// @Success 204
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to delete list"
// @Router /lists/{id} [delete]
func (h *ListHandler) DeleteList(w http.ResponseWriter, r *http.Request) {
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
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete list")
		return
	}

	err = h.ListService.DeleteList(ctx, authContext.UserID, listID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete list", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete list")
		return
	}

	slog.InfoContext(ctx, "deleted list successfully", slog.String("list_id", listID))
	response.Status(w, http.StatusNoContent)
}

// SetListName handles updating "name" of a list
//
// @Summary Updates "name" of list
// @Description Update an existing list, setting the "name" field
// @Tags List
// @Accept json
// @Produce json
// @Param id path int true "List ID"
// @Param payload body request.SetListNamePayload true "Update list name payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to set list name"
// @Router /lists/{id}/name [post]
func (h *ListHandler) SetListName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	listID := r.PathValue("id")
	if listID == "" {
		slog.ErrorContext(ctx, "failed to read list id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	payload, err := request.DecodeJSON[request.SetListNamePayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode set list name payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.ListService.SetListName(ctx, authContext.UserID, listID, payload.Name)
	if err != nil {
		slog.ErrorContext(ctx, "failed to set list name", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to set list name")
		return
	}

	slog.InfoContext(ctx, "updated list name successfully", slog.String("list_id", listID))
	response.Status(w, http.StatusNoContent)
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
		slog.ErrorContext(ctx, "failed to get lists", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to read lists")
		return
	}

	response.JSON(ctx, w, http.StatusOK, lists)
}

// OrderLists handles re-ordering a users lists
//
// @Summary Order lists of a user
// @Description Re-assigns the display order of all users lists
// @Tags List
// @Accept json
// @Produce json
// @Param payload body request.OrderListsPayload true "Order lists payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to order lists"
// @Router /lists/order [post]
func (h *ListHandler) OrderLists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.OrderListsPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode order lists payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.ListService.OrderLists(ctx, authContext.UserID, payload.ListIDs)
	if err != nil {
		if errors.Is(err, domain.ErrStaleListIDs) {
			response.Error(ctx, w, http.StatusBadRequest, "stale list ids")
		} else {
			response.Error(ctx, w, http.StatusInternalServerError, "failed to order lists")
		}

		slog.ErrorContext(ctx, "failed to order lists", slog.Any("error", err))
		return
	}

	slog.InfoContext(ctx, "lists re-ordered successfully", slog.String("user_id", authContext.UserID))
	response.Status(w, http.StatusNoContent)
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
// @Router /lists/items [post]
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

	slog.InfoContext(ctx, "created list item successfully", slog.String("list_item_id", item.ID))
	response.JSON(ctx, w, http.StatusCreated, item)
}

// DeleteListItem handles the deletion of a list item
//
// @Summary Delete list item route
// @Description Deletes an item
// @Tags List
// @Accept json
// @Produce json
// @Param id path int true "List Item ID"
// @Success 204
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to delete list item"
// @Router /lists/items/{id} [delete]
func (h *ListHandler) DeleteListItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	listItemID := r.PathValue("id")
	if listItemID == "" {
		slog.ErrorContext(ctx, "failed to read list item id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete list item")
		return
	}

	err = h.ListService.DeleteListItem(ctx, authContext.UserID, listItemID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete list item", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete list item")
		return
	}

	slog.InfoContext(ctx, "deleted list item successfully", slog.String("list_item_id", listItemID))
	response.Status(w, http.StatusNoContent)
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
// @Router /lists/items [put]
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

	slog.InfoContext(ctx, "updated list item successfully", slog.String("list_item_id", payload.ListItemID))
	response.Status(w, http.StatusNoContent)
}

// SetListItemTitle handles updating "title" of a list item
//
// @Summary Updates "title" of list item
// @Description Update an existing list item, setting the "title" field
// @Tags List
// @Accept json
// @Produce json
// @Param id path int true "List Item ID"
// @Param payload body request.SetListItemTitlePayload true "Update list item title payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to set list item title"
// @Router /lists/items/{id}/title [post]
func (h *ListHandler) SetListItemTitle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	listItemID := r.PathValue("id")
	if listItemID == "" {
		slog.ErrorContext(ctx, "failed to read list item id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	payload, err := request.DecodeJSON[request.SetListItemTitlePayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode set list item title payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusUnauthorized, "not authorized")
		return
	}

	err = h.ListService.SetListItemTitle(ctx, authContext.UserID, listItemID, payload.Title)
	if err != nil {
		slog.ErrorContext(ctx, "failed to set list item title", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to set list item title")
		return
	}

	slog.InfoContext(ctx, "update list item title successful", slog.String("list_item_id", listItemID))
	response.Status(w, http.StatusNoContent)
}

// SetListItemCompleted handles updating "is_completed" of a list item
//
// @Summary Updates "is_completed" of list item
// @Description Update an existing list item, setting the "is_completed" field
// @Tags List
// @Accept json
// @Produce json
// @Param id path int true "List Item ID"
// @Param payload body request.SetListItemCompletedPayload true "Update list item is completed payload"
// @Success 204 {object} nil
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to set list item completed"
// @Router /lists/items/{id}/complete [post]
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

	slog.InfoContext(ctx, "update list item completed successful", slog.String("list_item_id", listItemID))
	response.Status(w, http.StatusNoContent)
}

// GetListItemsForList handles return all list items of a list
//
// @Summary Returns all items from a list
// @Description Retrieve all list items of a list
// @Tags List
// @Accept json
// @Produce json
// @Param id path int true "List ID"
// @Success 200 {object} []domain.ListItem
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 401 {object} response.ErrorResponse "not authorized"
// @Error 500 {object} response.ErrorResponse "failed to read list items"
// @Router /lists/{id} [get]
func (h *ListHandler) GetListItemsForList(w http.ResponseWriter, r *http.Request) {
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

	items, err := h.ListService.GetListItemsForList(ctx, authContext.UserID, listID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to read list items", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to read list items")
		return
	}

	response.JSON(ctx, w, http.StatusOK, items)
}
