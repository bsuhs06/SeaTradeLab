package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bsuhs/shiptracker/web-server/internal/api"
	"github.com/bsuhs/shiptracker/web-server/internal/db"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		if err2 := godotenv.Load("../../ais-collector/.env"); err2 != nil {
			log.Println("No .env file found, using environment variables")
		}
	}

	logger := log.New(os.Stdout, "[AIS-WEB] ", log.LstdFlags|log.Lshortfile)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Fatal("DATABASE_URL environment variable is required")
	}
	port := getEnv("WEB_PORT", "8080")

	logger.Printf("Starting AIS Web Server on port %s", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo, err := db.NewRepo(ctx, databaseURL)
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()

	logger.Println("Connected to database")

	mux := http.NewServeMux()

	handler := api.NewHandler(repo, logger)
	handler.Register(mux)

	frontendDir := getEnv("FRONTEND_DIR", "")
	if frontendDir == "" {
		// Default: look for frontend/dist relative to working directory
		frontendDir = filepath.Join("frontend", "dist")
	}
	if info, err := os.Stat(frontendDir); err == nil && info.IsDir() {
		logger.Printf("Serving frontend from %s", frontendDir)
		fs := http.FileServer(http.Dir(frontendDir))
		mux.Handle("/", spaHandler(fs, frontendDir))
	} else {
		logger.Printf("Frontend dir %s not found, serving API only", frontendDir)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("SeaTradeLab API. Frontend not built — run: cd frontend && npm run build"))
		})
	}

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Println("Shutting down web server...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	logger.Printf("Web server listening on http://localhost:%s", port)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatalf("Server error: %v", err)
	}

	logger.Println("Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// spaHandler wraps a file server and falls back to index.html for client-side routing.
func spaHandler(h http.Handler, dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		h.ServeHTTP(w, r)
	})
}
