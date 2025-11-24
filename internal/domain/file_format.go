package domain

// FileFormat представляет формат файла
type FileFormat string

const (
	FormatYAML FileFormat = "yaml"
	FormatJSON FileFormat = "json"
)

// DetectFormat определяет формат файла по пути
func DetectFormat(filePath string) FileFormat {
	if len(filePath) >= 5 && filePath[len(filePath)-5:] == ".json" {
		return FormatJSON
	}
	if len(filePath) >= 5 && (filePath[len(filePath)-5:] == ".yaml" || filePath[len(filePath)-4:] == ".yml") {
		return FormatYAML
	}
	return FormatYAML // По умолчанию
}

