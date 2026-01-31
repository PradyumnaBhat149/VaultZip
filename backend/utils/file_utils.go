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
