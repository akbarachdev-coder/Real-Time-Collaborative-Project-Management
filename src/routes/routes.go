package routes

import (
	"net/http"

	"collabpm/config"
	"collabpm/controllers"
	"collabpm/middlewares"
	"collabpm/realtime"
	"collabpm/services"
	"collabpm/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(cfg *config.Config, db *gorm.DB, hub *realtime.Hub) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), CORS(cfg))

	r.GET("/health", func(c *gin.Context) {
		utils.OK(c, 200, gin.H{"status": "ok"})
	})

	authSvc := services.NewAuthService(db, cfg)
	userSvc := services.NewUserService(db)
	projectSvc := services.NewProjectService(db, hub)
	taskSvc := services.NewTaskService(db, hub)
	commentSvc := services.NewCommentService(db, hub)
	notificationSvc := services.NewNotificationService(db)

	authCtrl := controllers.NewAuthController(authSvc)
	userCtrl := controllers.NewUserController(userSvc)
	projectCtrl := controllers.NewProjectController(projectSvc)
	taskCtrl := controllers.NewTaskController(taskSvc)
	commentCtrl := controllers.NewCommentController(commentSvc)
	notificationCtrl := controllers.NewNotificationController(notificationSvc)
	wsCtrl := controllers.NewWSController(hub)

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", authCtrl.Register)
			auth.POST("/login", authCtrl.Login)
			auth.POST("/refresh", authCtrl.Refresh)
		}

		// WebSocket authenticates via ?token= instead of the Authorization header.
		r.GET("/api/ws", middlewares.AuthQuery(cfg.JWTAccessSecret), wsCtrl.Handle)

		authorized := api.Group("")
		authorized.Use(middlewares.Auth(cfg.JWTAccessSecret))
		{
			authorized.GET("/users/me", userCtrl.Me)
			authorized.PUT("/users/me", userCtrl.UpdateMe)
			authorized.GET("/users", userCtrl.Search)

			authorized.GET("/projects", projectCtrl.List)
			authorized.POST("/projects", projectCtrl.Create)
			authorized.GET("/projects/:id", projectCtrl.Get)
			authorized.PUT("/projects/:id", projectCtrl.Update)
			authorized.DELETE("/projects/:id", projectCtrl.Delete)
			authorized.GET("/projects/:id/members", projectCtrl.ListMembers)
			authorized.POST("/projects/:id/members", projectCtrl.AddMember)
			authorized.DELETE("/projects/:id/members/:userId", projectCtrl.RemoveMember)

			authorized.GET("/projects/:id/tasks", taskCtrl.ListByProject)
			authorized.POST("/projects/:id/tasks", taskCtrl.Create)
			authorized.GET("/tasks/:id", taskCtrl.Get)
			authorized.PUT("/tasks/:id", taskCtrl.Update)
			authorized.DELETE("/tasks/:id", taskCtrl.Delete)

			authorized.GET("/tasks/:id/comments", commentCtrl.List)
			authorized.POST("/tasks/:id/comments", commentCtrl.Create)
			authorized.DELETE("/comments/:id", commentCtrl.Delete)

			authorized.GET("/notifications", notificationCtrl.List)
			authorized.PATCH("/notifications/:id/read", notificationCtrl.MarkRead)
			authorized.POST("/notifications/read-all", notificationCtrl.MarkAllRead)
		}
	}

	return r
}

func CORS(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if isOriginAllowed(cfg.AllowedOrigins, origin) {
			if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			} else {
				c.Header("Access-Control-Allow-Origin", "*")
			}
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func isOriginAllowed(allowed []string, origin string) bool {
	for _, a := range allowed {
		if a == "*" || (origin != "" && a == origin) {
			return true
		}
	}
	return false
}
