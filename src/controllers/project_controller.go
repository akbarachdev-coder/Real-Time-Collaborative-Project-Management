package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type ProjectController struct {
	svc *services.ProjectService
}

func NewProjectController(svc *services.ProjectService) *ProjectController {
	return &ProjectController{svc: svc}
}

func (ctrl *ProjectController) List(c *gin.Context) {
	res, err := ctrl.svc.List(currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *ProjectController) Create(c *gin.Context) {
	var req services.CreateProjectRequest
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

func (ctrl *ProjectController) Get(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.Get(projectID, currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *ProjectController) Update(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.UpdateProjectRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Update(projectID, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *ProjectController) Delete(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := ctrl.svc.Delete(projectID, currentUserID(c)); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, gin.H{"deleted": true})
}
