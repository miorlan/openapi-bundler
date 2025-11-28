package resolver

type RefContext struct {
	InContent bool
	
	InSchema bool
	
	InOneOfAnyOfAllOf bool
	
	InComponentsSchemas bool
}

func NewRefContext(inContent, inSchema, inOneOfAnyOfAllOf, inComponentsSchemas bool) RefContext {
	return RefContext{
		InContent:           inContent,
		InSchema:            inSchema,
		InOneOfAnyOfAllOf:  inOneOfAnyOfAllOf,
		InComponentsSchemas: inComponentsSchemas,
	}
}

func DefaultRefContext() RefContext {
	return RefContext{}
}

func (c RefContext) WithContent() RefContext {
	c.InContent = true
	return c
}

func (c RefContext) WithSchema() RefContext {
	c.InSchema = true
	return c
}

func (c RefContext) WithOneOfAnyOfAllOf() RefContext {
	c.InOneOfAnyOfAllOf = true
	return c
}

func (c RefContext) WithComponentsSchemas() RefContext {
	c.InComponentsSchemas = true
	return c
}

func (c RefContext) ShouldInline(usageCount int) bool {
	if c.InOneOfAnyOfAllOf {
		return false
	}
	if c.InComponentsSchemas {
		return false
	}
	if c.InContent {
		return true
	}
	return usageCount == 1
}

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

