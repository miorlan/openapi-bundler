# OpenAPI Bundler

CLI утилита для объединения разбитых OpenAPI спецификаций в один файл.

## Установка

```bash
go install github.com/miorlan/openapi-bundler/cmd/openapi-bundler@latest
```

## Использование

```bash
# Базовое использование
openapi-bundler bundle -i api/openapi/index.yaml -o api/openapi/openapi.yaml

# Формат swagger-cli
openapi-bundler bundle -o api/openapi/openapi.yaml api/openapi/index.yaml

# Конвертация YAML -> JSON
openapi-bundler bundle -i api/openapi/index.yaml -o api/openapi/openapi.json

# Показать версию
openapi-bundler version
```

## Поддерживаемые форматы ссылок

- `./file.yaml`, `../file.yaml` — относительные пути
- `file.yaml#/components/schemas/User` — ссылки с фрагментами
- `https://example.com/schema.yaml` — HTTP/HTTPS ссылки
- `#/components/schemas/User` — внутренние ссылки

## Миграция с swagger-cli

```bash
# Было
swagger-cli bundle -o output.yaml input.yaml

# Стало
openapi-bundler bundle -o output.yaml input.yaml
```

## Makefile

```makefile
api:
	openapi-bundler bundle -o api/openapi/openapi.yaml api/openapi/index.yaml
	oapi-codegen --config=api/openapi/config.yaml api/openapi/openapi.yaml
```
