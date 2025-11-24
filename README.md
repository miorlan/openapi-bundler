# OpenAPI Bundler

[![Go Reference](https://pkg.go.dev/badge/github.com/miorlan/openapi-bundler.svg)](https://pkg.go.dev/github.com/miorlan/openapi-bundler)
[![CI](https://github.com/miorlan/openapi-bundler/workflows/CI/badge.svg)](https://github.com/miorlan/openapi-bundler/actions)
[![Coverage](https://codecov.io/gh/miorlan/openapi-bundler/branch/main/graph/badge.svg)](https://codecov.io/gh/miorlan/openapi-bundler)

CLI утилита и библиотека для объединения (bundling) разбитых OpenAPI спецификаций в один файл.

## Описание

`openapi-bundler` — это CLI утилита и библиотека, написанная на Go, которая объединяет разбитую на множество файлов OpenAPI спецификацию в один файл, разрешая все `$ref` ссылки.


## Установка

### CLI утилита

**Способ 1: Установка через go install (рекомендуется)**

```bash
go install github.com/miorlan/openapi-bundler/cmd@latest
```

После установки бинарник будет в `$(go env GOPATH)/bin/cmd`. Рекомендуется создать симлинк или алиас:

```bash
# Создайте симлинк с удобным именем
ln -s $(go env GOPATH)/bin/cmd $(go env GOPATH)/bin/openapi-bundler

# Или добавьте алиас в ~/.bashrc или ~/.zshrc
alias openapi-bundler='$(go env GOPATH)/bin/cmd'
```

**Способ 2: Сборка из исходников (рекомендуется для удобства)**

```bash
# Клонируйте репозиторий
git clone https://github.com/miorlan/openapi-bundler.git
cd openapi-bundler

# Соберите бинарник
make build

# Или напрямую
go build -o openapi-bundler ./cmd

# Используйте
./openapi-bundler version
```

### Установка man pages (опционально)

```bash
sudo make install-man
```

После установки вы сможете использовать:
```bash
man openapi-bundler
```

### Библиотека

```bash
go get github.com/miorlan/openapi-bundler
```

## Использование

### Как CLI утилита

```bash
# Базовое использование (YAML)
openapi-bundler bundle -i api/openapi/index.yaml -o api/openapi/openapi.gen.yaml

# Работа с JSON
openapi-bundler bundle -i api/openapi/index.json -o api/openapi/openapi.gen.json

# Конвертация форматов (YAML -> JSON)
openapi-bundler bundle -i api/openapi/index.yaml -o api/openapi/openapi.gen.json

# С валидацией
openapi-bundler bundle -i api/openapi/index.yaml -o api/openapi/openapi.gen.yaml --validate

# Работа с HTTP ссылками
openapi-bundler bundle -i https://example.com/api/openapi.yaml -o local-openapi.yaml

# С полными именами флагов
openapi-bundler bundle --input input.yaml --output output.yaml

# С подробным выводом (verbose)
openapi-bundler bundle -i input.yaml -o output.yaml --verbose

# Показать версию
openapi-bundler version

# Показать справку
openapi-bundler help

# Просмотр man страницы (после установки)
man openapi-bundler
```

### Как библиотека

```go
package main

import (
	"context"
	"log"

	bundler "github.com/miorlan/openapi-bundler"
)

func main() {
	ctx := context.Background()

	// Простое использование
	err := bundler.Bundle(ctx, "input.yaml", "output.yaml")
	if err != nil {
		log.Fatal(err)
	}
}
```

Подробные примеры использования библиотеки см. в [EXAMPLES.md](EXAMPLES.md).

## Документация

- [Примеры использования](EXAMPLES.md)
- [Changelog](CHANGELOG.md)
- [API Documentation](https://pkg.go.dev/github.com/miorlan/openapi-bundler)

## Поддерживаемые форматы ссылок

### Локальные файлы
- `./file.yaml` - относительные ссылки с префиксом
- `../file.yaml` - относительные ссылки на уровень выше
- `file.yaml` - относительные ссылки без префикса (в той же директории)
- `/absolute/path.yaml` - абсолютные пути

### Внешние ссылки
- `https://example.com/schema.yaml` - HTTP/HTTPS ссылки
- `http://example.com/schema.json` - HTTP ссылки на JSON файлы

### Внутренние ссылки
- `#/components/schemas/User` - внутренние ссылки (оставляются как есть)

### Форматы файлов
- Автоматическое определение формата по расширению (`.yaml`, `.yml`, `.json`)
- Поддержка смешанных форматов (YAML файлы могут ссылаться на JSON и наоборот)
- Автоматическая конвертация формата при сохранении

## Как это работает

1. Загружает корневой OpenAPI файл
2. Рекурсивно находит все `$ref` ссылки
3. Загружает содержимое по ссылкам
4. Заменяет `$ref` на фактическое содержимое
5. Сохраняет результат в один файл

## Миграция с swagger-cli

`openapi-bundler` является прямой заменой `swagger-cli` для объединения OpenAPI спецификаций.


### Замена команды

```bash
# Было (swagger-cli)
swagger-cli bundle -o api/openapi/openapi.yaml api/openapi/index.yaml --type yaml

# Стало (openapi-bundler)
openapi-bundler bundle -i api/openapi/index.yaml -o api/openapi/openapi.yaml
```

## В Makefile

**Пример замены `swagger-cli` на `openapi-bundler`:**

```makefile
api:
	@if ! command -v openapi-bundler >/dev/null 2>&1; then \
		echo "openapi-bundler не найден. Установи: go install github.com/miorlan/openapi-bundler/cmd@latest"; \
		exit 1; \
	fi
	openapi-bundler bundle -i api/openapi/index.yaml -o api/openapi/openapi.yaml
	go tool oapi-codegen --config=api/openapi/config.yaml api/openapi/openapi.yaml
```


## Разработка

### Требования

- Go 1.21 или выше

### Сборка

```bash
make build
```

### Тесты

```bash
make test
```

### Линтинг

```bash
make vet
make fmt
```


## Вклад

Вклад приветствуется! Пожалуйста, откройте issue или pull request.
