package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeka/zip"
)

type FileEntry struct {
	SourcePath string
	ZipPath    string
}

// CompressFiles creates a ZIP archive containing the specified files
func CompressFiles(entries []FileEntry, destPath string) error {
	zipFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	for _, entry := range entries {
		// 1. Open the file to be compressed
		f, err := os.Open(entry.SourcePath)
		if err != nil {
			return err
		}

		// 2. Get file info
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return err
		}

		// 3. Create a header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			f.Close()
			return err
		}

		// Use the specified ZipPath (relative structure)
		header.Name = entry.ZipPath

		// Use Deflate method for compression
		header.Method = zip.Deflate

		// 4. Create writer for the entry
		writer, err := archive.CreateHeader(header)
		if err != nil {
			f.Close()
			return err
		}

		if err != nil {
			f.Close()
			return err
		}

		// 5. Copy content
		_, err = io.Copy(writer, f)
		f.Close() // Close immediately after copy
		if err != nil {
			return err
		}
	}
	return nil
}

// ExtractFile extracts a ZIP archive to the destination folder
func ExtractFile(zipPath, destFolder, password string) ([]string, error) {
	var extractedFiles []string

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		// Set password if provided
		if password != "" {
			f.SetPassword(password)
		}

		// If encrypted and no password, return specific error
		if f.IsEncrypted() && password == "" {
			return nil, fmt.Errorf("password required")
		}

		// Prevent Zip Slip
		fpath := filepath.Join(destFolder, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(destFolder)+string(os.PathSeparator)) {
			return nil, fmt.Errorf("illegal file path: %s", fpath)
		}

		extractedFiles = append(extractedFiles, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return nil, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			// Check for wrong password error usually returned here
			errStr := err.Error()
			if strings.Contains(errStr, "password") || strings.Contains(errStr, "checksum") || strings.Contains(errStr, "corrupt input") {
				return nil, fmt.Errorf("invalid password")
			}
			return nil, err
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			errStr := err.Error()
			// Improve error detection for invalid password during decryption (io.Copy phase)
			if strings.Contains(errStr, "password") || strings.Contains(errStr, "checksum") || strings.Contains(errStr, "corrupt input") {
				return nil, fmt.Errorf("invalid password")
			}
			return nil, err
		}
	}
	return extractedFiles, nil
}
