package controllers

import (
	"log"

	"collabpm/middlewares"
	"collabpm/realtime"

	"github.com/gin-gonic/gin"
)

type WSController struct {
	hub *realtime.Hub
}

func NewWSController(hub *realtime.Hub) *WSController {
	return &WSController{hub: hub}
}

func (ctrl *WSController) Handle(c *gin.Context) {
	userID := middlewares.UserID(c)
	if err := realtime.ServeWS(ctrl.hub, c.Writer, c.Request, userID); err != nil {
		log.Printf("websocket upgrade failed: %v", err)
	}
}
