package resolver

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

func (r *ReferenceResolver) replaceExternalRefs(ctx context.Context, node interface{}, baseDir string, config domain.Config, depth int) error {
	return r.replaceExternalRefsWithContext(ctx, node, baseDir, config, depth, false, false)
}

func (r *ReferenceResolver) resolveAndReplaceExternalRefWithContext(ctx context.Context, ref string, baseDir string, config domain.Config, depth int, skipExtraction bool) (string, error) {
	return r.resolveAndReplaceExternalRefWithType(ctx, ref, baseDir, config, depth, "", skipExtraction)
}

func (r *ReferenceResolver) loadAndParseRefFile(ctx context.Context, refPath string, config domain.Config) (interface{}, error) {
	if !strings.HasPrefix(refPath, "http://") && !strings.HasPrefix(refPath, "https://") {
		refPath = filepath.Clean(refPath)
	}

	if cached, ok := r.fileCache[refPath]; ok {
		return cached, nil
	}

	data, err := r.fileLoader.Load(ctx, refPath)
	if err != nil {
		return nil, err
	}

	format := domain.DetectFormat(refPath)
	var parsed interface{}
	if err := r.parser.Unmarshal(data, &parsed, format); err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", refPath, err)
	}

	r.fileCache[refPath] = parsed
	return parsed, nil
}
