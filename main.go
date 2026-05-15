package main

import (
	"api-gateway/handlers"
	"api-gateway/logger"
	"api-gateway/middleware"
	"api-gateway/server"
	"api-gateway/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger.Init()

	logger.Info("Starting server...")
	port := utils.GetEnvInt("PORT", 8080)
	s := server.NewServer(port)

	s.Use(middleware.RequestID())

	logger.Info("Middlewares is ready")

	s.Router.GET("/healthz", handlers.HealthHandler)
	ready := handlers.NewReadyHandler()
	s.Router.GET("/readyz", ready.ServeHTTP)

	logger.Info("Router is ready")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := s.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server run error: %v", err)
		}
	}()

	logger.Info("Server is running...")

	<-stop

	fmt.Println("Server is shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}

	fmt.Println("Server stopped")
}
