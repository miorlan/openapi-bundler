package domain

import "fmt"

// ErrCircularReference возникает при обнаружении циклической ссылки
// Это бизнес-ошибка: циклические ссылки нарушают бизнес-правила OpenAPI
type ErrCircularReference struct {
	Path string
}

func (e *ErrCircularReference) Error() string {
	return fmt.Sprintf("circular reference detected: %s", e.Path)
}

// ErrInvalidReference возникает при некорректной ссылке
// Это бизнес-ошибка: ссылка не соответствует бизнес-правилам формата
type ErrInvalidReference struct {
	Ref string
}

func (e *ErrInvalidReference) Error() string {
	return fmt.Sprintf("invalid reference: %s", e.Ref)
}

