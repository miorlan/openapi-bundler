# Инструкция по загрузке на GitHub

## Шаг 1: Создайте репозиторий на GitHub

1. Перейдите на https://github.com/new
2. Заполните:
   - **Repository name**: `openapi-bundler`
   - **Description**: `CLI utility and library for bundling OpenAPI specifications`
   - **Visibility**: Public (или Private, если хотите)
   - **НЕ** добавляйте README, .gitignore или license (они уже есть)
3. Нажмите "Create repository"

## Шаг 2: Добавьте remote и загрузите код

После создания репозитория выполните:

```bash
# Добавьте remote (замените YOUR_USERNAME на ваш GitHub username)
git remote add origin https://github.com/YOUR_USERNAME/openapi-bundler.git

# Или если используете SSH:
# git remote add origin git@github.com:YOUR_USERNAME/openapi-bundler.git

# Загрузите код
git push -u origin main
```

## Шаг 3: Создайте первый релиз (опционально)

```bash
# Создайте тег для версии
git tag -a v0.1.0 -m "Release version 0.1.0"

# Загрузите тег
git push origin v0.1.0
```

Затем на GitHub:
1. Перейдите в "Releases" → "Create a new release"
2. Выберите тег `v0.1.0`
3. Заполните описание (можно скопировать из CHANGELOG.md)
4. Нажмите "Publish release"

## Шаг 4: Проверьте, что все работает

После загрузки проверьте:
- ✅ README отображается корректно
- ✅ CI/CD запускается (Actions)
- ✅ Go модуль доступен: `go get github.com/YOUR_USERNAME/openapi-bundler`
- ✅ CLI можно установить: `go install github.com/YOUR_USERNAME/openapi-bundler/cmd/openapi-bundler@latest`

## Важно

После загрузки на GitHub обновите в коде:
- `go.mod`: модуль уже правильный (`github.com/miorlan/openapi-bundler`)
- `README.md`: ссылки уже правильные
- Если ваш username отличается от `miorlan`, обновите:
  - `go.mod` (module path)
  - Все импорты в коде
  - README.md (ссылки на установку)

