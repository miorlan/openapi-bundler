package infrastructure

import (
	"context"

	"github.com/miorlan/openapi-bundler/internal/domain"
)

var componentTypes = []string{"schemas", "responses", "parameters", "examples", "requestBodies", "headers", "securitySchemes", "links", "callbacks"}

type ReferenceResolver struct {
	fileLoader domain.FileLoader
	parser     domain.Parser
	visited    map[string]bool
	rootDoc    map[string]interface{}
	components map[string]map[string]interface{}
	componentRefs map[string]string
	componentCounter map[string]int
	pathsBaseDir string
	componentsBaseDir map[string]string
	fileCache map[string]interface{}
	componentHashes map[string]string
	rootBasePath string
}

func NewReferenceResolver(fileLoader domain.FileLoader, parser domain.Parser) domain.ReferenceResolver {
	components := make(map[string]map[string]interface{})
	componentCounter := make(map[string]int)
	for _, ct := range componentTypes {
		components[ct] = make(map[string]interface{})
		componentCounter[ct] = 0
	}
	return &ReferenceResolver{
		fileLoader: fileLoader,
		parser:     parser,
		visited:    make(map[string]bool),
		components: components,
		componentRefs: make(map[string]string),
		componentCounter: componentCounter,
		componentsBaseDir: make(map[string]string),
		fileCache: make(map[string]interface{}),
		componentHashes: make(map[string]string),
	}
}

func (r *ReferenceResolver) ResolveAll(ctx context.Context, data map[string]interface{}, basePath string, config domain.Config) error {
	r.visited = make(map[string]bool)
	r.rootDoc = data
	r.rootBasePath = basePath
	r.pathsBaseDir = ""
	r.componentsBaseDir = make(map[string]string)
	r.fileCache = make(map[string]interface{})
	r.componentHashes = make(map[string]string)
	for _, ct := range componentTypes {
		r.components[ct] = make(map[string]interface{})
		r.componentCounter[ct] = 0
	}
	r.componentRefs = make(map[string]string)

	var components map[string]interface{}
	if c, ok := data["components"].(map[string]interface{}); ok {
		components = c
	} else {
		components = make(map[string]interface{})
		data["components"] = components
	}

	for _, ct := range componentTypes {
		if _, ok := components[ct]; !ok {
			components[ct] = make(map[string]interface{})
		}
	}

	if err := r.expandSectionRefs(ctx, data, basePath, config); err != nil {
		return err
	}

	if err := r.replaceExternalRefs(ctx, data, basePath, config, 0); err != nil {
		return err
	}

	for _, ct := range componentTypes {
		if section, ok := components[ct].(map[string]interface{}); ok {
			for name, component := range r.components[ct] {
				if component == nil {
					continue
				}
				if existing, exists := section[name]; !exists {
					section[name] = component
				} else {
					if _, isMap := existing.(map[string]interface{}); !isMap {
						if _, isComponentMap := component.(map[string]interface{}); isComponentMap {
							section[name] = component
						}
					}
				}
			}
		}
	}

	if err := r.replaceExternalRefs(ctx, data, basePath, config, 0); err != nil {
		return err
	}

	r.cleanNilValues(data)

	for _, ct := range componentTypes {
		if section, ok := components[ct].(map[string]interface{}); ok {
			if len(section) == 0 {
				delete(components, ct)
			}
		}
	}

	if componentsMap, ok := data["components"].(map[string]interface{}); ok {
		if len(componentsMap) == 0 {
			delete(data, "components")
		}
	}

	return nil
}

func (r *ReferenceResolver) Resolve(ctx context.Context, ref string, basePath string, config domain.Config) (interface{}, error) {
	refPath := r.getRefPath(ref, basePath)
	if refPath == "" {
		return nil, &domain.ErrInvalidReference{Ref: ref}
	}
	return r.loadAndParseFile(ctx, ref, basePath, config)
}
