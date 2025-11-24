package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

type FileWriter struct{}

func NewFileWriter() domain.FileWriter {
	return &FileWriter{}
}

// Write удаляет файл, если data == nil (используется при ошибке валидации)
func (fw *FileWriter) Write(path string, data []byte) error {
	if data == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	outputDir := filepath.Dir(path)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

