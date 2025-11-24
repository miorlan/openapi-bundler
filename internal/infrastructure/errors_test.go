package infrastructure

import "testing"

func TestErrFileNotFound_Error(t *testing.T) {
	err := &ErrFileNotFound{Path: "/path/to/file.yaml"}
	want := "file not found: /path/to/file.yaml"
	if got := err.Error(); got != want {
		t.Errorf("ErrFileNotFound.Error() = %v, want %v", got, want)
	}
}

