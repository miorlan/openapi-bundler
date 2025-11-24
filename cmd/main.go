package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/miorlan/openapi-bundler/internal/usecase"
)

//go:embed version.txt
var version string

func init() {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "0.1.0" // fallback
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Обработка команд версии и помощи
	switch command {
	case "version", "-version", "-v", "--version", "--v":
		fmt.Printf("openapi-bundler version %s\n", strings.TrimSpace(version))
		os.Exit(0)

	case "help", "-help", "-h", "--help", "--h":
		printUsage()
		os.Exit(0)
	}

	// Обработка команды bundle
	if command == "bundle" {
		var (
			inputPath  string
			outputPath string
			validate   bool
			verbose    bool
			fileType   string // для совместимости со swagger-cli (--type)
		)

		bundleCmd := flag.NewFlagSet("bundle", flag.ExitOnError)
		bundleCmd.StringVar(&inputPath, "i", "", "Путь к входному OpenAPI файлу")
		bundleCmd.StringVar(&inputPath, "input", "", "Путь к входному OpenAPI файлу")
		bundleCmd.StringVar(&outputPath, "o", "", "Путь к выходному файлу")
		bundleCmd.StringVar(&outputPath, "output", "", "Путь к выходному файлу")
		bundleCmd.StringVar(&fileType, "type", "", "Тип файла (yaml/json) - для совместимости со swagger-cli, определяется автоматически")
		bundleCmd.BoolVar(&validate, "validate", false, "Валидировать OpenAPI спецификацию после объединения")
		bundleCmd.BoolVar(&verbose, "verbose", false, "Подробный вывод")
		bundleCmd.BoolVar(&verbose, "v", false, "Подробный вывод (краткая форма)")

		if err := bundleCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Ошибка парсинга флагов: %v\n", err)
			os.Exit(1)
		}

		// Поддержка swagger-cli формата: позиционный аргумент для input
		// swagger-cli bundle -o output.yaml input.yaml --type yaml
		if inputPath == "" && len(bundleCmd.Args()) > 0 {
			inputPath = bundleCmd.Args()[0]
		}

		if inputPath == "" || outputPath == "" {
			fmt.Fprintf(os.Stderr, "❌ Ошибка: необходимо указать входной и выходной файлы\n")
			fmt.Fprintf(os.Stderr, "Использование:\n")
			fmt.Fprintf(os.Stderr, "  openapi-bundler bundle -i <input> -o <output>\n")
			fmt.Fprintf(os.Stderr, "  openapi-bundler bundle -o <output> <input>  (совместимо со swagger-cli)\n")
			os.Exit(1)
		}

		// Проверяем, что входной и выходной файлы не одинаковые
		if inputPath == outputPath {
			fmt.Fprintf(os.Stderr, "❌ Ошибка: входной и выходной файлы не могут быть одинаковыми\n")
			os.Exit(1)
		}

		// Определяем, нужен ли прогресс-бар (для файлов > 100KB или verbose режим)
		showProgress := verbose
		if !showProgress {
			if info, err := os.Stat(inputPath); err == nil && info.Size() > 100*1024 {
				showProgress = true
			}
		}

		if verbose {
			fmt.Fprintf(os.Stderr, "📦 Загрузка входного файла: %s\n", inputPath)
		}

		bundler := newBundler()
		ctx := context.Background()
		config := usecase.Config{Validate: validate}
		
		if showProgress && !verbose {
			progress := NewSimpleProgress(true)
			progress.Update("📦 Загрузка входного файла...")
		}
		
		if err := bundler.Execute(ctx, inputPath, outputPath, config); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "❌ Ошибка при объединении: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "❌ Ошибка: %v\n", err)
			}
			os.Exit(1)
		}

		if showProgress && !verbose {
			progress := NewSimpleProgress(true)
			progress.Update("🔄 Объединение ссылок...")
			progress.Update("✅ Объединение завершено")
			if validate {
				progress.Update("🔍 Валидация OpenAPI спецификации...")
				progress.Update("✅ Валидация пройдена")
			}
			progress.Update(fmt.Sprintf("💾 Результат сохранен: %s", outputPath))
		} else if verbose {
			fmt.Fprintf(os.Stderr, "✅ Объединение завершено\n")
			if validate {
				fmt.Fprintf(os.Stderr, "🔍 Валидация OpenAPI спецификации...\n")
				fmt.Fprintf(os.Stderr, "✅ Валидация пройдена\n")
			}
			fmt.Fprintf(os.Stderr, "💾 Сохранение результата: %s\n", outputPath)
		}

		validateMsg := ""
		if validate {
			validateMsg = " и валидирована"
		}
		fmt.Printf("✅ OpenAPI спецификация успешно объединена%s: %s\n", validateMsg, outputPath)
		return
	}

	// Неизвестная команда
	fmt.Fprintf(os.Stderr, "❌ Неизвестная команда: %s\n\n", command)
	printUsage()
	os.Exit(1)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `openapi-bundler - утилита для объединения разбитых OpenAPI спецификаций

Использование:
  openapi-bundler <команда> [флаги]

Команды:
  bundle    Объединить разбитую OpenAPI спецификацию в один файл
            Используйте 'openapi-bundler bundle --help' для справки по флагам
  version   Показать версию
  help      Показать эту справку

Примеры:
  openapi-bundler bundle -i input.yaml -o output.yaml
  openapi-bundler bundle -o output.yaml input.yaml  # формат swagger-cli
  openapi-bundler version

Подробная документация: https://github.com/miorlan/openapi-bundler

`)
}

