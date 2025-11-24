package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// ProgressBar представляет простой прогресс-бар
type ProgressBar struct {
	total   int
	current int
	writer  io.Writer
	enabled bool
}

// NewProgressBar создает новый прогресс-бар
func NewProgressBar(total int, enabled bool) *ProgressBar {
	return &ProgressBar{
		total:   total,
		current: 0,
		writer:  os.Stderr,
		enabled: enabled,
	}
}

// Update обновляет прогресс-бар
func (pb *ProgressBar) Update(current int, message string) {
	if !pb.enabled {
		return
	}

	pb.current = current
	if pb.total > 0 {
		percent := float64(current) / float64(pb.total) * 100
		barWidth := 30
		filled := int(float64(barWidth) * percent / 100)
		bar := strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled)
		
		fmt.Fprintf(pb.writer, "\r[%s] %.1f%% %s", bar, percent, message)
		if current >= pb.total {
			fmt.Fprintf(pb.writer, "\n")
		}
	} else {
		// Если total неизвестен, показываем только счетчик
		fmt.Fprintf(pb.writer, "\rОбработано файлов: %d %s", current, message)
	}
}

// Finish завершает прогресс-бар
func (pb *ProgressBar) Finish() {
	if !pb.enabled {
		return
	}
	fmt.Fprintf(pb.writer, "\n")
}

// SimpleProgress показывает простой прогресс без бара
type SimpleProgress struct {
	writer  io.Writer
	enabled bool
}

// NewSimpleProgress создает новый простой прогресс
func NewSimpleProgress(enabled bool) *SimpleProgress {
	return &SimpleProgress{
		writer:  os.Stderr,
		enabled: enabled,
	}
}

// Update обновляет прогресс
func (sp *SimpleProgress) Update(message string) {
	if !sp.enabled {
		return
	}
	fmt.Fprintf(sp.writer, "  %s\n", message)
}

