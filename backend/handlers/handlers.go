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
		// Check for context cancellation (e.g., refresh or cancel)
		select {
		case <-r.Context().Done():
			fmt.Println("Upload canceled by client")
			return
		default:
		}

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
			data, err := io.ReadAll(io.LimitReader(part, 32*1024))
			if err != nil {
				fmt.Printf("Error reading path: %v\n", err)
				continue
			}
			nextRelPath = strings.TrimSpace(string(data))
			continue
		}

		if formName == "files" {
			relPath := nextRelPath
			if relPath == "" {
				relPath = part.FileName()
			}
			nextRelPath = ""

			if relPath == "" {
				continue
			}

			if strings.Contains(relPath, "..") {
				continue
			}

			targetPath := filepath.Join(sessionDir, relPath)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				fmt.Printf("Mkdir error: %v\n", err)
				continue
			}

			out, err := os.Create(targetPath)
			if err != nil {
				fmt.Printf("Create file error: %v\n", err)
				continue
			}

			// Copy with context awareness to stop early if canceled
			// We can't easily interrupt io.Copy, but we can check context before/after
			// and use a smaller buffer if we really wanted to be reactive.
			// For now, checking context before each part is a huge improvement.
			n, err := io.Copy(out, part)
			out.Close()

			if err != nil {
				// If error was due to context cancellation, remove partial file
				if r.Context().Err() != nil {
					os.Remove(targetPath)
					fmt.Println("Cleaned up partial file due to cancellation")
					return
				}
				fmt.Printf("Copy error: %v\n", err)
			} else {
				fmt.Printf("Uploaded %s: %d bytes\n", relPath, n)
				uploadedFilenames = append(uploadedFilenames, relPath)
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
	// Output file - prefix with sessionId for unified cleanup
	outputFilename := fmt.Sprintf("%s_%s", req.SessionID, utils.GenerateTimestampedFilename("archive.zip"))
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

	// Create a folder for extraction - prefix with sessionId for unified cleanup
	extractDirName := fmt.Sprintf("%s_%s", req.SessionID, utils.GenerateTimestampedFilename(filepath.Base(filename)+"_extracted"))

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

// DeleteSessionHandler removes the entire session directory
func DeleteSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sessionDir := filepath.Join(UploadDir, sessionID)
	fmt.Printf("Explicit cleanup for session: %s\n", sessionID)

	// 1. Delete uploads
	os.RemoveAll(sessionDir)

	// 2. Delete processed files/folders starting with sessionId
	processedEntries, err := os.ReadDir(ProcessedDir)
	if err == nil {
		prefix := sessionID + "_"
		for _, entry := range processedEntries {
			if strings.HasPrefix(entry.Name(), prefix) {
				path := filepath.Join(ProcessedDir, entry.Name())
				fmt.Printf("Cleanup processed item: %s\n", entry.Name())
				os.RemoveAll(path)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Session deleted"})
}

// DeleteFileHandler removes a specific file from the session
func DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	filename := r.URL.Query().Get("filename")

	if sessionID == "" || filename == "" {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(UploadDir, sessionID, filename)
	fmt.Printf("Explicit removal of file: %s in session %s\n", filename, sessionID)
	os.RemoveAll(filePath) // RemoveAll handles both files and directories

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "File deleted"})
}
