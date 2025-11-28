package domain

import "testing"

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     FileFormat
	}{
		{
			name:     "YAML file with .yaml extension",
			filePath: "test.yaml",
			want:     FormatYAML,
		},
		{
			name:     "YAML file with .yml extension",
			filePath: "test.yml",
			want:     FormatYAML,
		},
		{
			name:     "JSON file",
			filePath: "test.json",
			want:     FormatJSON,
		},
		{
			name:     "File without extension",
			filePath: "test",
			want:     FormatYAML, // default
		},
		{
			name:     "File with path",
			filePath: "/path/to/file.yaml",
			want:     FormatYAML,
		},
		{
			name:     "File with path and JSON",
			filePath: "/path/to/file.json",
			want:     FormatJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectFormat(tt.filePath); got != tt.want {
				t.Errorf("DetectFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

