package controllers

import (
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	svc *services.UserService
}

func NewUserController(svc *services.UserService) *UserController {
	return &UserController{svc: svc}
}

func (ctrl *UserController) Me(c *gin.Context) {
	res, err := ctrl.svc.GetMe(currentUserID(c))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *UserController) UpdateMe(c *gin.Context) {
	var req services.UpdateProfileRequest
	if !bindJSON(c, &req) {
		return
	}
	res, err := ctrl.svc.UpdateMe(currentUserID(c), req)
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}

func (ctrl *UserController) Search(c *gin.Context) {
	res, err := ctrl.svc.Search(c.Query("q"))
	if err != nil {
		utils.RespondError(c, err)
		return
	}
	utils.OK(c, 200, res)
}
