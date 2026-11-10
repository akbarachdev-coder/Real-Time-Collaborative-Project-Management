package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type ProjectStatusController struct {
	svc *services.ProjectStatusService
}

func NewProjectStatusController(svc *services.ProjectStatusService) *ProjectStatusController {
	return &ProjectStatusController{svc: svc}
}

func (ctrl *ProjectStatusController) List(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.List(projectID, currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *ProjectStatusController) Create(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.CreateStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Create(projectID, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 201, res)
}

func (ctrl *ProjectStatusController) Update(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	statusID, ok := parseID(c, "statusId")
	if !ok {
		return
	}
	var req services.UpdateStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Update(projectID, statusID, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *ProjectStatusController) Delete(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	statusID, ok := parseID(c, "statusId")
	if !ok {
		return
	}
	if err := ctrl.svc.Delete(projectID, statusID, currentUserID(c)); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, gin.H{"deleted": true})
}

func (ctrl *ProjectStatusController) Reorder(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.ReorderStatusesRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Reorder(projectID, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}
