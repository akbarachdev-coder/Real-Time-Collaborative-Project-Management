package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type AuthController struct {
	svc *services.AuthService
}

func NewAuthController(svc *services.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

func (ctrl *AuthController) Register(c *gin.Context) {
	var req services.RegisterRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Register(req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 201, res)
}

func (ctrl *AuthController) Login(c *gin.Context) {
	var req services.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Login(req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *AuthController) Refresh(c *gin.Context) {
	var req services.RefreshRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.Refresh(req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}
