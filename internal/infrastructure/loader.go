package infrastructure

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

type FileLoader struct {
	client *http.Client
}

func NewFileLoader() domain.FileLoader {
	return NewFileLoaderWithTimeout(30 * time.Second)
}

func NewFileLoaderWithTimeout(timeout time.Duration) domain.FileLoader {
	return &FileLoader{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (fl *FileLoader) Load(ctx context.Context, path string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		cleanPath := filepath.Clean(path)
		if !filepath.IsAbs(cleanPath) {
			absPath, err := filepath.Abs(cleanPath)
			if err != nil {
				return nil, fmt.Errorf("invalid path: %w", err)
			}
			cleanPath = absPath
		}
		path = cleanPath
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return fl.loadHTTP(ctx, path)
	}
	return os.ReadFile(path)
}

func (fl *FileLoader) loadHTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := fl.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch HTTP resource: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTTP response: %w", err)
	}

	return data, nil
}
