.PHONY: build install clean test help

# Имя бинарного файла
BINARY_NAME=openapi-bundler

# Путь для установки (можно переопределить через GOPATH/bin или GOBIN)
INSTALL_PATH=$(shell go env GOPATH)/bin

help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Собрать бинарный файл
	@echo "🔨 Сборка $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) ./cmd
	@echo "✅ Готово: ./$(BINARY_NAME)"

install: build ## Установить в $(INSTALL_PATH) с созданием симлинка
	@echo "📦 Установка $(BINARY_NAME) в $(INSTALL_PATH)..."
	@mkdir -p $(INSTALL_PATH)
	@cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✅ Установлено: $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo "💡 Убедитесь, что $(INSTALL_PATH) в вашем PATH"

install-go: ## Установить через go install и создать симлинк
	@echo "📦 Установка через go install..."
	@go install ./cmd
	@echo "🔗 Создание симлинка openapi-bundler -> cmd..."
	@if [ -f "$(INSTALL_PATH)/cmd" ]; then \
		ln -sf $(INSTALL_PATH)/cmd $(INSTALL_PATH)/openapi-bundler; \
		echo "✅ Симлинк создан: $(INSTALL_PATH)/openapi-bundler -> $(INSTALL_PATH)/cmd"; \
	else \
		echo "❌ Ошибка: $(INSTALL_PATH)/cmd не найден"; \
		exit 1; \
	fi
	@echo "💡 Убедитесь, что $(INSTALL_PATH) в вашем PATH"

clean: ## Удалить собранные файлы
	@echo "🧹 Очистка..."
	@rm -f $(BINARY_NAME)
	@echo "✅ Готово"

test: ## Запустить тесты
	@echo "🧪 Запуск тестов..."
	@go test -v ./...

fmt: ## Форматировать код
	@echo "📝 Форматирование кода..."
	@go fmt ./...

vet: ## Проверить код с помощью go vet
	@echo "🔍 Проверка кода..."
	@go vet ./...

install-man: ## Установить man pages (требуются права sudo)
	@echo "📖 Установка man pages..."
	@mkdir -p /usr/local/share/man/man1
	@cp man/openapi-bundler.1 /usr/local/share/man/man1/
	@mandb > /dev/null 2>&1 || true
	@echo "✅ Man pages установлены. Используйте: man openapi-bundler"

.PHONY: codegen api

codegen: api1 api

api: build
	@# Используем локальный бинарный файл
	@./$(BINARY_NAME) bundle -o openapi/openapi.custom.yaml openapi/index.yaml

api1:
	echo "Запуск oapi-codegen"
	@# Используем swagger-cli через npx, чтобы не требовалась глобальная установка
	npx --yes swagger-cli bundle -o openapi/openapi.yaml openapi/index.yaml --type yaml
