package resolver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
	"github.com/miorlan/openapi-bundler/internal/infrastructure/errors"
)

func (r *ReferenceResolver) loadAndParseFile(ctx context.Context, ref string, baseDir string, config domain.Config) (interface{}, error) {
	refPath := r.getRefPath(ref, baseDir)
	if refPath == "" {
		return nil, &domain.ErrInvalidReference{Ref: ref}
	}

	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	if cached, ok := r.fileCache[refPath]; ok {
		return cached, nil
	}

	data, err := r.fileLoader.Load(ctx, refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &errors.ErrFileNotFound{Path: refPath}
		}
		return nil, fmt.Errorf("failed to load file: %w", err)
	}

	if config.MaxFileSize > 0 && int64(len(data)) > config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", len(data), config.MaxFileSize)
	}

	format := domain.DetectFormat(refPath)
	var content interface{}
	if err := r.parser.Unmarshal(data, &content, format); err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	r.fileCache[refPath] = content
	return content, nil
}

func (r *ReferenceResolver) extractSection(content interface{}, path ...string) map[string]interface{} {
	current := content
	for i, key := range path {
		if m, ok := current.(map[string]interface{}); ok {
			if val, exists := m[key]; exists {
				current = val
			} else {
				if i == 0 && len(path) > 1 {
					return nil
				}
				if i == 0 && len(path) == 1 {
					return m
				}
				return nil
			}
		} else {
			return nil
		}
	}
	if sectionMap, ok := current.(map[string]interface{}); ok {
		return sectionMap
	}
	if m, ok := content.(map[string]interface{}); ok {
		return m
	}
	return nil
}

