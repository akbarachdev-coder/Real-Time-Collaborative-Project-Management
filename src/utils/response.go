package utils

import (
	"errors"
	"log"

	"github.com/gin-gonic/gin"
)

type ErrorBody struct {
	Error string `json:"error"`
}

func OK(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

func Fail(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorBody{Error: message})
}

func RespondError(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		Fail(c, appErr.Status, appErr.Message)
		return
	}
	log.Printf("internal error: %v", err)
	Fail(c, 500, "internal server error")
}
