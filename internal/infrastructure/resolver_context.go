package infrastructure

// RefContext содержит контекст обработки ссылки
// Заменяет множественные булевые флаги в функциях
type RefContext struct {
	// InContent указывает, что мы обрабатываем схему внутри content (responses/requestBody)
	// В этом случае схемы инлайнятся, а не извлекаются в components
	InContent bool
	
	// InSchema указывает, что мы обрабатываем схему (используется для вложенных схем)
	InSchema bool
	
	// InOneOfAnyOfAllOf указывает, что мы обрабатываем ссылку внутри oneOf/anyOf/allOf
	// В этом случае ссылки всегда остаются как $ref, не инлайнятся
	InOneOfAnyOfAllOf bool
	
	// InComponentsSchemas указывает, что мы обрабатываем ссылку внутри components.schemas
	// В этом случае ссылки не инлайнятся, остаются как $ref
	InComponentsSchemas bool
}

// NewRefContext создает новый контекст с указанными флагами
func NewRefContext(inContent, inSchema, inOneOfAnyOfAllOf, inComponentsSchemas bool) RefContext {
	return RefContext{
		InContent:           inContent,
		InSchema:            inSchema,
		InOneOfAnyOfAllOf:  inOneOfAnyOfAllOf,
		InComponentsSchemas: inComponentsSchemas,
	}
}

// DefaultRefContext создает контекст по умолчанию (все флаги false)
func DefaultRefContext() RefContext {
	return RefContext{}
}

// WithContent возвращает новый контекст с установленным флагом InContent
func (c RefContext) WithContent() RefContext {
	c.InContent = true
	return c
}

// WithSchema возвращает новый контекст с установленным флагом InSchema
func (c RefContext) WithSchema() RefContext {
	c.InSchema = true
	return c
}

// WithOneOfAnyOfAllOf возвращает новый контекст с установленным флагом InOneOfAnyOfAllOf
func (c RefContext) WithOneOfAnyOfAllOf() RefContext {
	c.InOneOfAnyOfAllOf = true
	return c
}

// WithComponentsSchemas возвращает новый контекст с установленным флагом InComponentsSchemas
func (c RefContext) WithComponentsSchemas() RefContext {
	c.InComponentsSchemas = true
	return c
}

// ShouldInline определяет, должна ли ссылка быть инлайнирована
// Ссылка НЕ инлайнится, если:
// - она в oneOf/anyOf/allOf
// - она в components.schemas
// - она в content и используется несколько раз (извлекается в components, но инлайнится в content)
func (c RefContext) ShouldInline(usageCount int) bool {
	if c.InOneOfAnyOfAllOf {
		return false
	}
	if c.InComponentsSchemas {
		return false
	}
	// В content всегда инлайним (но если используется несколько раз, сначала извлекаем в components)
	if c.InContent {
		return true
	}
	// Вне content инлайним только одноразовые ссылки
	return usageCount == 1
}

// ShouldExtractToComponents определяет, должна ли ссылка быть извлечена в components
// Ссылка извлекается, если:
// - она используется несколько раз
// - она в oneOf/anyOf/allOf (даже если используется один раз)
// - она в components.schemas (даже если используется один раз)
func (c RefContext) ShouldExtractToComponents(usageCount int) bool {
	if usageCount > 1 {
		return true
	}
	if c.InOneOfAnyOfAllOf {
		return true
	}
	if c.InComponentsSchemas {
		return true
	}
	return false
}

