package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EnsureDir creates a directory if it does not exist
func EnsureDir(dirName string) error {
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		return err
	}
	return nil
}

// GenerateTimestampedFilename avoids collisions
func GenerateTimestampedFilename(originalName string) string {
	ext := filepath.Ext(originalName)
	name := originalName[:len(originalName)-len(ext)]
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_%d%s", name, timestamp, ext)
}

// CleanupDir removes files and directories within a root that are older than maxAge
func CleanupDir(root string, maxAge time.Duration) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	now := time.Now()
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > maxAge {
			path := filepath.Join(root, entry.Name())
			fmt.Printf("Cleaning up: %s (Age: %v)\n", path, now.Sub(info.ModTime()))
			os.RemoveAll(path)
		}
	}
	return nil
}
