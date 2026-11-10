package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"collabpm/config"
	"collabpm/realtime"
	"collabpm/routes"
	"collabpm/services"
)

func main() {
	cfg := config.Load()

	db, err := config.ConnectMySQL(cfg)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	hub := realtime.NewHub(cfg.AllowedOrigins, func(projectID, userID uint) (bool, error) {
		return services.IsProjectMember(db, projectID, userID)
	})

	router := routes.New(cfg, db, hub)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
