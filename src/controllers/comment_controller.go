package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type CommentController struct {
	svc *services.CommentService
}

func NewCommentController(svc *services.CommentService) *CommentController {
	return &CommentController{svc: svc}
}

func (ctrl *CommentController) List(c *gin.Context) {
	taskID, ok := parseID(c, "id")
	if !ok {
		return
	}
	res, err := ctrl.svc.List(taskID, currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *CommentController) Create(c *gin.Context) {
	taskID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req services.CreateCommentRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Create(taskID, currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 201, res)
}

func (ctrl *CommentController) Delete(c *gin.Context) {
	commentID, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := ctrl.svc.Delete(commentID, currentUserID(c)); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, gin.H{"deleted": true})
}
