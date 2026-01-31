package main

import (
	"compressor-backend/handlers"
	"compressor-backend/utils"
	"log"
	"net/http"
	"os"
	"time"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Session-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Ensure directories exist
	utils.EnsureDir(handlers.UploadDir)
	utils.EnsureDir(handlers.ProcessedDir)

	// Initialize temporary storage
	if err := handlers.InitStorage(); err != nil {
		log.Fatal("Failed to initialize storage:", err)
	}

	// Start Background Cleanup Worker (Clean every 10 mins, files older than 30 mins)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			utils.CleanupDir(handlers.UploadDir, 30*time.Minute)
			utils.CleanupDir(handlers.ProcessedDir, 30*time.Minute)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/upload", handlers.UploadHandler)
	mux.HandleFunc("/compress", handlers.CompressHandler)
	mux.HandleFunc("/extract", handlers.ExtractHandler)
	mux.HandleFunc("/download/", handlers.DownloadHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("VaultZip Server running at http://localhost:%s\n", port)

	err := http.ListenAndServe(":"+port, enableCORS(mux))
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
