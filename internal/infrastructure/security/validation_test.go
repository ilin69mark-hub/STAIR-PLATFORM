package security

import (
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"user@example.com", true},
		{"test.email@example.com", true},
		{"user+tag@example.com", true},
		{"user@example.co.uk", true},
		{"", false},
		{"user", false},
		{"user@", false},
		{"@example.com", false},
		{"user@.com", false},
		{"user@example", false},
		{"user@example..com", false},
	}

	for _, tt := range tests {
		result := ValidateEmail(tt.email)
		if result != tt.expected {
			t.Errorf("ValidateEmail(%q) = %v, want %v", tt.email, result, tt.expected)
		}
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello world", "hello world"},
		{"  hello  ", "hello"},
		{"hello\x00world", "helloworld"},
		{"hello\x01world", "helloworld"},
		{"", ""},
		{"  ", ""},
	}

	for _, tt := range tests {
		result := SanitizeString(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		username string
		expected bool
	}{
		{"user", true},
		{"user123", true},
		{"user_name", true},
		{"User_Name_123", true},
		{"us", false},
		{"a", false},
		{"a]verylongusername12345678901234567890", false},
		{"user name", false},
		{"user@name", false},
	}

	for _, tt := range tests {
		result := ValidateUsername(tt.username)
		if result != tt.expected {
			t.Errorf("ValidateUsername(%q) = %v, want %v", tt.username, result, tt.expected)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		expected bool
	}{
		{"password1", true},
		{"Password123", true},
		{"pass1", false},
		{"password", false},
		{"12345678", false},
		{"", false},
	}

	for _, tt := range tests {
		result := ValidatePassword(tt.password)
		if result != tt.expected {
			t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, result, tt.expected)
		}
	}
}

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"My Project", true},
		{"Project 123", true},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		result := ValidateProjectName(tt.name)
		if result != tt.expected {
			t.Errorf("ValidateProjectName(%q) = %v, want %v", tt.name, result, tt.expected)
		}
	}
}

func TestContainsSQLInjection(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"normal input", false},
		{"' OR '1'='1", true},
		{"'; DROP TABLE users", true},
		{"UNION SELECT * FROM users", true},
		{"admin'--", true},
		{"1' OR '1'='1", true},
	}

	for _, tt := range tests {
		result := ContainsSQLInjection(tt.input)
		if result != tt.expected {
			t.Errorf("ContainsSQLInjection(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestValidateSortOrder(t *testing.T) {
	tests := []struct {
		order    string
		expected bool
	}{
		{"asc", true},
		{"desc", true},
		{"", true},
		{"ASC", false},
		{"random", false},
	}

	for _, tt := range tests {
		result := ValidateSortOrder(tt.order)
		if result != tt.expected {
			t.Errorf("ValidateSortOrder(%q) = %v, want %v", tt.order, result, tt.expected)
		}
	}
}

func TestValidatePagination(t *testing.T) {
	tests := []struct {
		limit    int
		offset   int
		expected bool
	}{
		{10, 0, true},
		{100, 0, true},
		{1000, 0, true},
		{0, 0, true},
		{-1, 0, false},
		{1001, 0, false},
		{10, -1, false},
	}

	for _, tt := range tests {
		result := ValidatePagination(tt.limit, tt.offset)
		if result != tt.expected {
			t.Errorf("ValidatePagination(%d, %d) = %v, want %v", tt.limit, tt.offset, result, tt.expected)
		}
	}
}
