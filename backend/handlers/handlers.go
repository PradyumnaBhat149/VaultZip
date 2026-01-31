package handlers

import (
	"compressor-backend/services"
	"compressor-backend/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	UploadDir    = ""
	ProcessedDir = ""
)

func InitStorage() error {
	baseTemp := filepath.Join(os.TempDir(), "VaultZip")
	UploadDir = filepath.Join(baseTemp, "uploads")
	ProcessedDir = filepath.Join(baseTemp, "processed")

	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(ProcessedDir, 0755); err != nil {
		return err
	}

	fmt.Printf("Storage initialized at: %s\n", baseTemp)
	return nil
}

type ProcessRequest struct {
	SessionID string   `json:"sessionId"`
	Filenames []string `json:"filenames"` // These are relative paths now
	Password  string   `json:"password"`  // Optional password
}

type ProcessResponse struct {
	Message     string `json:"message"`
	DownloadURL string `json:"downloadUrl"`
}

// UploadHandler handles file uploads
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		http.Error(w, "Missing Session ID", http.StatusBadRequest)
		return
	}

	// Use MultipartReader for streaming uploads (no size limit checks here, relying on disk space)
	reader, err := r.MultipartReader()
	if err != nil {
		fmt.Printf("MultipartReader error: %v\n", err)
		http.Error(w, "Invalid multipart request", http.StatusBadRequest)
		return
	}

	// Uploads go to ./uploads/{sessionID}
	sessionDir := filepath.Join(UploadDir, sessionID)
	if err := utils.EnsureDir(sessionDir); err != nil {
		http.Error(w, "Server error: cannot create session dir", http.StatusInternalServerError)
		return
	}

	uploadedFilenames := make([]string, 0)

	// State to track the path for the upcoming file
	var nextRelPath string

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Error reading part: %v\n", err)
			continue
		}

		formName := part.FormName()

		if formName == "paths" {
			// Read the path string completely
			// Limit to 32KB to prevent abuse
			data, err := io.ReadAll(io.LimitReader(part, 32*1024))
			if err != nil {
				fmt.Printf("Error reading path: %v\n", err)
				continue
			}
			nextRelPath = strings.TrimSpace(string(data))
			continue
		}

		if formName == "files" {
			// Determine relative path
			// 1. Use nextRelPath if available
			// 2. Fallback to part.FileName()
			relPath := nextRelPath
			if relPath == "" {
				relPath = part.FileName()
			}

			// Reset for next file
			nextRelPath = ""

			if relPath == "" {
				continue
			}

			// Prevent traversal
			if strings.Contains(relPath, "..") {
				continue
			}

			targetPath := filepath.Join(sessionDir, relPath)

			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				fmt.Printf("Mkdir error: %v\n", err)
				continue
			}

			out, err := os.Create(targetPath)
			if err != nil {
				fmt.Printf("Create file error: %v\n", err)
				continue
			}

			// Copy stream directly to file
			n, err := io.Copy(out, part)
			out.Close()

			fmt.Printf("Uploaded %s: %d bytes\n", relPath, n)

			if err == nil {
				uploadedFilenames = append(uploadedFilenames, relPath)
			} else {
				fmt.Printf("Copy error: %v\n", err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Upload successful",
		"filenames": uploadedFilenames,
	})
}

// CompressHandler handles compression requests
func CompressHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" || len(req.Filenames) == 0 {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	sessionDir := filepath.Join(UploadDir, req.SessionID)

	var entries []services.FileEntry

	for _, relPath := range req.Filenames {
		// Securely join
		if strings.Contains(relPath, "..") {
			continue
		}

		fullPath := filepath.Join(sessionDir, relPath)

		// Check if it exists
		if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
			entries = append(entries, services.FileEntry{
				SourcePath: fullPath,
				ZipPath:    relPath, // Preserve structure in Zip
			})
		}
	}

	if len(entries) == 0 {
		http.Error(w, "Files not found", http.StatusNotFound)
		return
	}

	if err := utils.EnsureDir(ProcessedDir); err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Output file - name it generally if mixed, or by file if single
	// If 1 ROOT file/folder, name it accordingly.
	// Simplification: archive_timestamp.zip
	outputFilename := utils.GenerateTimestampedFilename("archive.zip")
	outputPath := filepath.Join(ProcessedDir, outputFilename)

	err := services.CompressFiles(entries, outputPath)
	if err != nil {
		fmt.Printf("Compression error: %v\n", err)
		http.Error(w, "Compression failed", http.StatusInternalServerError)
		return
	}

	response := ProcessResponse{
		Message:     "Compression successful",
		DownloadURL: fmt.Sprintf("/download/%s", outputFilename),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ExtractHandler handles extraction requests
func ExtractHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" || len(req.Filenames) == 0 {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	filename := req.Filenames[0]
	sessionDir := filepath.Join(UploadDir, req.SessionID)
	sourcePath := filepath.Join(sessionDir, filename)

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Create a folder for extraction
	extractDirName := filepath.Base(filename) + "_extracted"
	// Ensure unique extraction per session? Or global?
	// Global ProcessedDir is fine for now, we use unique timestamps for zips.
	// But extracted folders might collide.
	// Use timestamp.
	extractDirName = utils.GenerateTimestampedFilename(extractDirName)

	extractPath := filepath.Join(ProcessedDir, extractDirName)

	if err := utils.EnsureDir(extractPath); err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	files, err := services.ExtractFile(sourcePath, extractPath, req.Password)
	if err != nil {
		fmt.Printf("Extraction error: %v\n", err)
		// Return specific error message for frontend handling
		if err.Error() == "password required" || err.Error() == "invalid password" {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		http.Error(w, "Extraction failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	downloadLinks := make([]string, 0)
	for _, f := range files {
		// Provide link to flattened view or direct?
		// NOTE: if file f is "folder/a.txt", we need to serve it.
		// DownloadHandler in main.go needs to handle slashes correctly or we use `http.FileServer` but we'd need to strip prefix.
		// Current DownloadHandler: `filename := r.URL.Path[len("/download/"):]`.
		// If path is `ext/folder/a.txt`, `Base` in DownloadHandler strips folder!?
		// We need to fix DownloadHandler logic in main.go/handlers.go too?
		// Let's check DownloadHandler currently in file.

		downloadLinks = append(downloadLinks, fmt.Sprintf("/download/%s/%s", extractDirName, f))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Extraction successful",
		"files":   downloadLinks,
	})
}

// DownloadHandler serves files from processed directory
func DownloadHandler(w http.ResponseWriter, r *http.Request) {
	// URL path usually /download/subdir/filename
	// We need to strip /download/
	filename := r.URL.Path[len("/download/"):]

	// Prevent path traversal
	if strings.Contains(filename, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Local path
	fileLoc := filepath.Join(ProcessedDir, filename)

	if _, err := os.Stat(fileLoc); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filename))
	http.ServeFile(w, r, fileLoc)
}
