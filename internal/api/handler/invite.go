package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/robindittmar/dttmr-api/internal/api/response"
	"github.com/robindittmar/dttmr-api/internal/domain"
)

type InviteHandler struct {
	InviteService *domain.InviteService
}

func NewInviteHandler(inviteService *domain.InviteService) *InviteHandler {
	return &InviteHandler{InviteService: inviteService}
}

// CreateInvite handles the creation of an invitation
//
// @Summary Create invite route
// @Description Create an invitation, which may be used to register an account
// @Tags Invite
// @Accept json
// @Produce json
// @Success 201 {object} domain.Invite
// @Error 500 {object} response.ErrorResponse "failed to create invite"
// @Router /user/invites [post]
func (h *InviteHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	invite, err := h.InviteService.CreateInvite(ctx, authContext.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create invite", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	slog.InfoContext(ctx, "created invite successfully", slog.String("invite_id", invite.ID))
	response.JSON(ctx, w, http.StatusCreated, invite)
}

// DeleteInvite handles the deletion of an invitation
//
// @Summary Delete invitation route
// @Description Deletes an invitation. Invitations can only be deleted when they have not been used
// @Tags Invite
// @Accept json
// @Produce json
// @Param id path int true "Invite ID"
// @Success 204
// @Error 400 {object} response.ErrorResponse "failed to decode request url"
// @Error 500 {object} response.ErrorResponse "failed to delete invite"
// @Router /user/invites/{id} [delete]
func (h *InviteHandler) DeleteInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	inviteID := r.PathValue("id")
	if inviteID == "" {
		slog.ErrorContext(ctx, "failed to read invite id from path")
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete invite")
		return
	}

	err = h.InviteService.DeleteInvite(ctx, authContext.UserID, inviteID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete invite", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to delete invite")
		return
	}

	slog.InfoContext(ctx, "deleted invite successfully", slog.String("invite_id", inviteID))
	response.Status(w, http.StatusNoContent)
}

// GetInvites handles fetching the list of a users invitations
//
// @Summary Get invitations route
// @Description Gets a list of all invitations the user has created
// @Tags Invite
// @Accept json
// @Produce json
// @Param page query int false "page"
// @Param count query int false "count"
// @Success 200 {object} response.Paginated[domain.Invite]
// @Error 400 {object} response.ErrorResponse "failed to decode request body"
// @Error 400 {object} response.ErrorResponse "invalid value for page"
// @Error 400 {object} response.ErrorResponse "invalid value for count"
// @Error 500 {object} response.ErrorResponse "failed to get invites"
// @Router /user/invites [get]
func (h *InviteHandler) GetInvites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		pageStr = "1"
	}
	countStr := r.URL.Query().Get("count")
	if countStr == "" {
		countStr = "10"
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil {
		slog.ErrorContext(ctx,
			"failed to read page from query",
			slog.String("page", pageStr))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		slog.ErrorContext(ctx,
			"failed to read count from query",
			slog.String("count", countStr))
		response.Error(ctx, w, http.StatusBadRequest, "failed to decode request url")
		return
	}

	if page < 1 {
		slog.ErrorContext(ctx,
			"page parameter is invalid",
			slog.Int("page", page))
		response.Error(ctx, w, http.StatusBadRequest, "invalid value for page")
		return
	}
	if count <= 0 {
		slog.ErrorContext(ctx,
			"count parameter is invalid",
			slog.Int("count", count))
		response.Error(ctx, w, http.StatusBadRequest, "invalid value for count")
		return
	}

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to get invites")
		return
	}

	invites, err := h.InviteService.GetInvites(ctx, authContext.UserID, page, count)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get invites", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to get invites")
		return
	}

	total, err := h.InviteService.CountInvites(ctx, authContext.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to count invites", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to get invites")
		return
	}

	response.JSON(ctx, w, http.StatusOK, response.Paginated[domain.Invite]{
		Count: len(invites),
		Total: total,
		Data:  invites,
	})
}

// GetInvitesStatus handles fetching the counts of a users invitations
//
// @Summary Get invitations status
// @Description Gets active/expired/used counts for all the users invites
// @Tags Invite
// @Accept json
// @Produce json
// @Success 200 {object} domain.InviteCounts
// @Error 500 {object} response.ErrorResponse "failed to count invites"
// @Router /user/invites [get]

func (h *InviteHandler) GetInvitesStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to count invites")
		return
	}

	counts, err := h.InviteService.CountInvitesStructured(ctx, authContext.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to count invites", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to count invites")
		return
	}

	response.JSON(ctx, w, http.StatusOK, counts)
}
