package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type TaskDocStateController struct {
	svc *services.TaskDocStateService
}

func NewTaskDocStateController(svc *services.TaskDocStateService) *TaskDocStateController {
	return &TaskDocStateController{svc: svc}
}

func (ctrl *TaskDocStateController) Get(c *gin.Context) {
	taskID, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.Get(taskID, currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *TaskDocStateController) Put(c *gin.Context) {
	taskID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.PutTaskDocStateRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Put(taskID, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}
