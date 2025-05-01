package fileserver

import (
	"context"
	"fmt"
	"log"
	"naevis/fileserver/routes"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"go.mongodb.org/mongo-driver/mongo"
)

var DB *mongo.Client

// Security headers middleware
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate, private")
		next.ServeHTTP(w, r)
	})
}

func FileServer() {
	baseRouter := httprouter.New()
	baseRouter.GET("/health", Index)

	routes.AddStaticRoutes(baseRouter)

	// Use http.NewServeMux() to allow future extensibility
	mux := http.NewServeMux()
	mux.Handle("/", baseRouter)

	// Apply CORS first, then security headers
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}).Handler(securityHeaders(mux))

	server := &http.Server{
		Addr:              ":6767",
		Handler:           handler,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	// Register cleanup tasks on shutdown
	server.RegisterOnShutdown(func() {
		log.Println("🛑 Cleaning up resources before shutdown...")
		// Add cleanup tasks like closing DB connection
	})

	// Start server in a goroutine
	go func() {
		log.Println("✅ Server started on port 6767")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	// Graceful shutdown handling
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownChan
	log.Printf("⚠️  Received signal: %v. Shutting down...", sig)

	// Gracefully shut down with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("❌ Server shutdown failed: %v", err)
	}

	log.Println("✅ Server stopped gracefully")
}

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Fprint(w, "200")
}
