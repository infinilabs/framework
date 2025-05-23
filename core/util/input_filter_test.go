package util

import "testing"

func TestCleanUserQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simplequery", "simplequery"},
		{"hello+world", "hello\\+world"},
		{"name:John", "name\\:John"},
		{"email@domain.com", "email@domain.com"},
		{"user[admin]", "user\\[admin\\]"},
		{"price>100", "price\\>100"},
		{"(nested) query", "\\(nested\\) query"},
		{"quote\"test\"", "quote\\\"test\\\""},
		{"slash/test", "slash\\/test"},
		{"complex && || ! query", "complex \\&& \\|| \\! query"},
		{"100%", "100%"},
		{"escaped\\backslash", "escaped\\\\backslash"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := CleanUserQuery(tt.input)
			if result != tt.expected {
				t.Errorf("CleanUserQuery(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
