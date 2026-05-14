package main

import (
	"context"
	"errors"
	"fmt"
	"httpserver/handlers"
	"httpserver/logger"
	"httpserver/middleware"
	"httpserver/server"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger.Init()

	s := server.NewServer(8080)

	s.Use(middleware.RequestID())

	s.Router.GET("/healthz", handlers.HealthHandler)
	ready := handlers.NewReadyHandler()
	s.Router.GET("/readyz", ready.ServeHTTP)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := s.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server run error: %v", err)
		}
	}()

	fmt.Println("Server is running...")

	<-stop

	fmt.Println("Server is shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}

	fmt.Println("Server stopped")
}
