package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type WorkspaceController struct {
	svc *services.WorkspaceService
}

func NewWorkspaceController(svc *services.WorkspaceService) *WorkspaceController {
	return &WorkspaceController{svc: svc}
}

func (ctrl *WorkspaceController) List(c *gin.Context) {
	res, err := ctrl.svc.List(currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *WorkspaceController) Create(c *gin.Context) {
	var req services.CreateWorkspaceRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Create(currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 201, res)
}

func (ctrl *WorkspaceController) Get(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.Get(id, currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *WorkspaceController) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.UpdateWorkspaceRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Update(id, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *WorkspaceController) Delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := ctrl.svc.Delete(id, currentUserID(c)); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, gin.H{"deleted": true})
}

func (ctrl *WorkspaceController) ListMembers(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.ListMembers(id, currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *WorkspaceController) AddMember(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.AddWorkspaceMemberRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.AddMember(id, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 201, res)
}

func (ctrl *WorkspaceController) UpdateMemberRole(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := parseID(c, "userId")
	if !ok {
		return
	}
	var req services.UpdateWorkspaceMemberRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.UpdateMemberRole(id, currentUserID(c), userID, req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *WorkspaceController) RemoveMember(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := parseID(c, "userId")
	if !ok {
		return
	}
	if err := ctrl.svc.RemoveMember(id, currentUserID(c), userID); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, gin.H{"removed": true})
}
