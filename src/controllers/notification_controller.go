package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type NotificationController struct {
	svc *services.NotificationService
}

func NewNotificationController(svc *services.NotificationService) *NotificationController {
	return &NotificationController{svc: svc}
}

func (ctrl *NotificationController) List(c *gin.Context) {
	unreadOnly := c.Query("unread") == "true"
	res, err := ctrl.svc.List(currentUserID(c), unreadOnly)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *NotificationController) MarkRead(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := ctrl.svc.MarkRead(currentUserID(c), id); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, gin.H{"read": true})
}

func (ctrl *NotificationController) MarkAllRead(c *gin.Context) {
	if err := ctrl.svc.MarkAllRead(currentUserID(c)); err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, gin.H{"read": true})
}
