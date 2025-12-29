package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/stut/s3dir/internal/config"
	"github.com/stut/s3dir/pkg/auth"
	"github.com/stut/s3dir/pkg/s3"
	"github.com/stut/s3dir/pkg/storage"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Print startup information
	fmt.Printf("S3Dir - S3-Compatible Directory Server\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Data Directory: %s\n", cfg.DataDir)
	fmt.Printf("Listen Address: %s\n", cfg.Address())
	fmt.Printf("Authentication: %v\n", cfg.EnableAuth)
	fmt.Printf("Read-Only Mode: %v\n", cfg.ReadOnly)
	fmt.Printf("Verbose Logging: %v\n", cfg.Verbose)
	fmt.Printf("========================================\n\n")

	// Initialize storage
	store, err := storage.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize S3 handler
	handler := s3.NewHandler(store, cfg.ReadOnly, cfg.Verbose)

	// Initialize authenticator
	authenticator := auth.New(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.EnableAuth)

	// Setup HTTP server with middleware
	var httpHandler http.Handler = handler
	if cfg.EnableAuth {
		httpHandler = authenticator.Middleware(httpHandler)
	}

	// Add CORS headers for browser compatibility
	httpHandler = corsMiddleware(httpHandler)

	// Create server
	server := &http.Server{
		Addr:    cfg.Address(),
		Handler: httpHandler,
	}

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		fmt.Printf("Server starting on %s\n", cfg.Address())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-shutdown
	fmt.Println("\nShutting down server...")

	// Graceful shutdown
	if err := server.Close(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	fmt.Println("Server stopped")
}

// corsMiddleware adds CORS headers to responses
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Content-Length, X-Amz-Date, X-Amz-Content-Sha256")
		w.Header().Set("Access-Control-Expose-Headers", "ETag, Last-Modified")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
