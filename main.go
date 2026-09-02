package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/shrimp-music/backend/api"
	"github.com/shrimp-music/backend/cache"
	"github.com/shrimp-music/backend/db"
	"github.com/shrimp-music/backend/limiter"
	"github.com/shrimp-music/backend/ytdlp"
)

func main() {
	fmt.Println("🦐 Shrimp Music Resolver Starting...")

	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables.")
	}

	// Initialize Database
	db.InitDB()

	// Initialize Cache
	c := cache.New()

	// Initialize Semaphore with 8 concurrent workers max
	l := limiter.New(8)

	// Start Background Updater
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ytdlp.AutoUpdater(ctx)

	// Setup API Routes
	appAPI := api.NewAPI(c, l)
	mux := http.NewServeMux()
	appAPI.RegisterRoutes(mux)

	// Configure Server
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		fmt.Printf("Server is running on port %s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	fmt.Println("Server gracefully stopped.")
}
