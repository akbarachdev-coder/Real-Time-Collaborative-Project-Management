package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type ActivityController struct {
	svc *services.ActivityService
}

func NewActivityController(svc *services.ActivityService) *ActivityController {
	return &ActivityController{svc: svc}
}

func (ctrl *ActivityController) ListByWorkspace(c *gin.Context) {
	workspaceID, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.ListByWorkspace(workspaceID, currentUserID(c), 50)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *ActivityController) ListByTask(c *gin.Context) {
	taskID, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.ListByTask(taskID, currentUserID(c), 50)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}
