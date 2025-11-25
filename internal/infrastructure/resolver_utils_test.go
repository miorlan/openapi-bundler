package infrastructure

import (
	"testing"
)

func TestNormalizeComponentName(t *testing.T) {
	resolver := &ReferenceResolver{
		componentCounter: make(map[string]int),
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal name",
			input:    "User",
			expected: "User",
		},
		{
			name:     "name with dashes",
			input:    "200-OK-OTP",
			expected: "C200_OK_OTP",
		},
		{
			name:     "name with path prefix",
			input:    ".._.._schemas_200-OK-OTP",
			expected: "C200_OK_OTP",
		},
		{
			name:     "name with component type prefix",
			input:    "schemas_User",
			expected: "User",
		},
		{
			name:     "name with multiple prefixes",
			input:    ".._.._schemas_ChangePasswordRequest",
			expected: "ChangePasswordRequest",
		},
		{
			name:     "name starting with number",
			input:    "200Response",
			expected: "C200Response",
		},
		{
			name:     "name with special characters",
			input:    "user-name@domain.com",
			expected: "user_name_domain_com",
		},
		{
			name:     "empty after normalization",
			input:    "---",
			expected: "Component1",
		},
		{
			name:     "real component names should be preserved",
			input:    "RequestGuests",
			expected: "RequestGuests",
		},
		{
			name:     "real component names with mixed case",
			input:    "ExistingGuest",
			expected: "ExistingGuest",
		},
		{
			name:     "real component names with underscores",
			input:    "AnonimGuest",
			expected: "AnonimGuest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.normalizeComponentName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeComponentName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

