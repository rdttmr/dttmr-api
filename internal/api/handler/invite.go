package handler

import (
	"log/slog"
	"net/http"

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
// @Success 200 {object} []domain.Invite
// @Error 500 {object} response.ErrorResponse "failed to get invites"
// @Router /user/invites [get]
func (h *InviteHandler) GetInvites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	authContext, err := domain.GetAuthContext(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get auth context", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to get invites")
		return
	}

	invites, err := h.InviteService.GetInvites(ctx, authContext.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get invites", slog.Any("error", err))
		response.Error(ctx, w, http.StatusInternalServerError, "failed to get invites")
		return
	}

	response.JSON(ctx, w, http.StatusOK, invites)
}
