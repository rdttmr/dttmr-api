package handler

import "git.dittmar.dev/robin/dttmr-api/internal/domain"

type GroupHandler struct {
	GroupService *domain.GroupService
}

func NewGroupHandler(groupService *domain.GroupService) *GroupHandler {
	return &GroupHandler{GroupService: groupService}
}
