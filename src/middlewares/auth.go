package middlewares

import (
	"strings"

	"collabpm/utils"

	"github.com/gin-gonic/gin"
)

func Auth(accessSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			utils.Fail(c, 401, "missing or invalid authorization token")
			c.Abort()
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		authorize(c, token, accessSecret)
	}
}

// AuthQuery authenticates WebSocket upgrade requests via ?token= query param,
// since browsers cannot set headers on WebSocket handshakes.
func AuthQuery(accessSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			utils.Fail(c, 401, "missing token")
			c.Abort()
			return
		}
		authorize(c, token, accessSecret)
	}
}

func authorize(c *gin.Context, token, secret string) {
	claims, err := utils.ParseToken(token, secret)
	if err != nil || claims.TokenType != utils.TokenTypeAccess {
		utils.Fail(c, 401, "invalid or expired token")
		c.Abort()
		return
	}
	c.Set("userID", claims.UserID)
	c.Next()
}

func UserID(c *gin.Context) uint {
	id, _ := c.Get("userID")
	userID, _ := id.(uint)
	return userID
}
