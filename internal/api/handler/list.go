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
// @Router /api/v1/list [post]
func (h *ListHandler) CreateList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	payload, err := request.DecodeJSON[request.CreateListPayload](r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to decode create list payload", slog.Any("error", err))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request body")
		return
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
