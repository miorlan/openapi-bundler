package infrastructure

import (
	"context"
	"fmt"

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
	refToComponentName map[string]string // Кэш: исходный $ref -> имя компонента
}

func NewReferenceResolver(fileLoader domain.FileLoader, parser domain.Parser) domain.ReferenceResolver {
	components := make(map[string]map[string]interface{}, len(componentTypes))
	componentCounter := make(map[string]int, len(componentTypes))
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
		refToComponentName: make(map[string]string),
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
	r.refToComponentName = make(map[string]string)
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

	// Предзагружаем все внешние файлы параллельно (только если нет ограничения глубины)
	// При ограничении глубины предзагрузка может нарушить логику проверки глубины
	if config.MaxDepth == 0 {
		if err := r.preloadExternalFiles(ctx, data, basePath, config); err != nil {
			return fmt.Errorf("failed to preload external files: %w", err)
		}
	}

	// ОДИН проход replaceExternalRefs - он обработает все ссылки и соберёт компоненты
	if err := r.replaceExternalRefs(ctx, data, basePath, config, 0); err != nil {
		return err
	}

	// Объединяем собранные компоненты в финальную секцию с проверкой уникальности
	for _, ct := range componentTypes {
		if section, ok := components[ct].(map[string]interface{}); ok {
			for name, component := range r.components[ct] {
				if component == nil {
					continue
				}
				
				// Нормализуем имя перед добавлением
				normalizedName := r.normalizeComponentName(name)
				
				// Проверяем дедупликацию по хешу
				componentHash := r.hashComponent(component)
				
				// ВАЖНО: Проверяем, не существует ли компонент с таким же содержимым в section под другим именем
				// Это нужно, чтобы заменить компоненты, которые являются только $ref
				for existingName, existingComponent := range section {
					if existingMap, ok := existingComponent.(map[string]interface{}); ok {
						if refVal, hasRef := existingMap["$ref"]; hasRef {
							if len(existingMap) == 1 {
								// Существующий компонент - это только $ref
								if refStr, ok := refVal.(string); ok {
									expectedRef := "#/components/" + ct + "/" + existingName
									if refStr == expectedRef {
										// Это самоссылка, проверяем, не тот ли это же компонент по содержимому
										if r.componentsEqual(component, existingComponent) || r.hashComponent(component) == r.hashComponent(existingComponent) {
											// Заменяем на реальное содержимое
											section[existingName] = component
											r.componentHashes[componentHash] = existingName
											// Удаляем компонент из r.components, если он был добавлен с другим именем
											if name != existingName {
												delete(r.components[ct], name)
											}
											goto nextComponent
										}
									}
								}
							}
						}
					}
				}
				
				if existingName, exists := r.componentHashes[componentHash]; exists {
					if existingName != normalizedName {
						// Компонент с таким же содержимым уже существует под другим именем
						// Используем существующее имя вместо создания дубликата
						continue
					}
				}
				
				// Проверяем уникальность имени перед добавлением
				if existing, exists := section[normalizedName]; !exists {
					// Имя уникально, добавляем компонент
					section[normalizedName] = component
					r.componentHashes[componentHash] = normalizedName
				} else {
					// Имя уже существует
					// СНАЧАЛА проверяем, не является ли существующий компонент только $ref
					if existingMap, ok := existing.(map[string]interface{}); ok {
						if refVal, hasRef := existingMap["$ref"]; hasRef {
							if len(existingMap) == 1 {
								// Существующий компонент - это только $ref
								// Проверяем, не ссылается ли он сам на себя
								if refStr, ok := refVal.(string); ok {
									expectedRef := "#/components/" + ct + "/" + normalizedName
									if refStr == expectedRef {
										// Это самоссылка (Error: { $ref: '#/components/schemas/Error' })
										// Заменяем на реальное содержимое
										section[normalizedName] = component
										r.componentHashes[componentHash] = normalizedName
										continue
									}
								}
								// Это ссылка на другой компонент, заменяем на реальное содержимое
								section[normalizedName] = component
								r.componentHashes[componentHash] = normalizedName
								continue
							}
						}
					}
					// Проверяем, не тот ли это же компонент
					if r.componentsEqual(existing, component) {
						// Это тот же компонент, пропускаем
						continue
					}
					// Разные компоненты с одинаковым именем - это конфликт
					// Используем уникальное имя
					uniqueName := r.ensureUniqueComponentName(normalizedName, section, ct)
					section[uniqueName] = component
					r.componentHashes[componentHash] = uniqueName
				}
			nextComponent:
			}
		}
	}

	// "Поднимаем" $ref в components после разрешения всех ссылок
	if err := r.liftComponentRefs(ctx, data, config); err != nil {
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
