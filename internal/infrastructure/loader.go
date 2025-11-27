package infrastructure

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

type FileLoader struct {
	client *http.Client
	sem    chan struct{}
}

const defaultMaxConcurrent = 10

func NewFileLoader() domain.FileLoader {
	return NewFileLoaderWithTimeout(30 * time.Second)
}

func NewFileLoaderWithTimeout(timeout time.Duration) domain.FileLoader {
	return NewFileLoaderWithTimeoutAndConcurrency(timeout, defaultMaxConcurrent)
}

func NewFileLoaderWithTimeoutAndConcurrency(timeout time.Duration, maxConcurrent int) domain.FileLoader {
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}
	return &FileLoader{
		client: &http.Client{
			Timeout: timeout,
		},
		sem: make(chan struct{}, maxConcurrent),
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

func (fl *FileLoader) LoadMany(ctx context.Context, paths []string) (map[string][]byte, error) {
	if len(paths) == 0 {
		return make(map[string][]byte), nil
	}

	results := make(map[string][]byte, len(paths))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(paths))

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			if fl.sem != nil {
				select {
				case fl.sem <- struct{}{}:
					defer func() { <-fl.sem }()
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				}
			}

			data, err := fl.Load(ctx, p)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("failed to load %s: %w", p, err):
				case <-ctx.Done():
				}
				return
			}

			mu.Lock()
			results[p] = data
			mu.Unlock()
		}(path)
	}

	wg.Wait()
	close(errCh)

	if len(errCh) > 0 {
		return nil, <-errCh
	}

	return results, nil
}
