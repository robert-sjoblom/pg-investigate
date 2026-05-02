package output

import (
	"os"
	"path/filepath"
	"time"
)

// Output writers for investigation results

func BuildOutputPath(outputDir, investigationName, vmName string) (string, error) {
	date := time.Now().Format("2006-01-02")
	path := filepath.Join(outputDir, investigationName, date, vmName)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return "", err
	}

	return path, nil
}
