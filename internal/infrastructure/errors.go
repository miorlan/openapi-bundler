package infrastructure

import "fmt"

// ErrFileNotFound - техническая ошибка инфраструктуры (файловая система, HTTP)
type ErrFileNotFound struct {
	Path string
}

func (e *ErrFileNotFound) Error() string {
	return fmt.Sprintf("file not found: %s", e.Path)
}

