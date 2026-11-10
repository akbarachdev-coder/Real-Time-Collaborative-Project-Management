package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type TaskController struct {
	svc *services.TaskService
}

func NewTaskController(svc *services.TaskService) *TaskController {
	return &TaskController{svc: svc}
}

func (ctrl *TaskController) ListByProject(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.ListByProject(projectID, currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *TaskController) Create(c *gin.Context) {
	projectID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.CreateTaskRequest
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

func (ctrl *TaskController) Get(c *gin.Context) {
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

func (ctrl *TaskController) Update(c *gin.Context) {
	taskID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.UpdateTaskRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Update(taskID, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *TaskController) Delete(c *gin.Context) {
	taskID, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := ctrl.svc.Delete(taskID, currentUserID(c)); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, gin.H{"deleted": true})
}
