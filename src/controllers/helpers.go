package controllers

import (
	"strconv"

	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

func bindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		utils.Fail(c, 400, err.Error())
		return false
	}
	return true
}

func currentUserID(c *gin.Context) uint {
	id, _ := c.Get("userID")
	userID, _ := id.(uint)
	return userID
}

func parseID(c *gin.Context, name string) (uint, bool) {
	raw := c.Param(name)
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || v == 0 {
		utils.Fail(c, 400, "invalid "+name)
		return 0, false
	}
	return uint(v), true
}
