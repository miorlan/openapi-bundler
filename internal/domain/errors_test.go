package domain

import "testing"

func TestErrCircularReference_Error(t *testing.T) {
	err := &ErrCircularReference{Path: "/path/to/file.yaml"}
	want := "circular reference detected: /path/to/file.yaml"
	if got := err.Error(); got != want {
		t.Errorf("ErrCircularReference.Error() = %v, want %v", got, want)
	}
}

func TestErrInvalidReference_Error(t *testing.T) {
	err := &ErrInvalidReference{Ref: "./invalid.yaml"}
	want := "invalid reference: ./invalid.yaml"
	if got := err.Error(); got != want {
		t.Errorf("ErrInvalidReference.Error() = %v, want %v", got, want)
	}
}

