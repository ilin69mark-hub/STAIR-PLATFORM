package email

import (
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	valid := []string{
		"user@example.com",
		"user.name+tag@example.co",
		"USER@Example.COM",
		"a@b.co",
	}
	for _, s := range valid {
		if !Valid(s) {
			t.Errorf("Valid(%q) = false, want true", s)
		}
	}

	// Известная мягкость: "user@example..com" regex принимает
	// (поведение унаследовано от бывшего infrastructure/validation).
	invalid := []string{
		"",
		"no-at-sign",
		"noserver@example",
		"a@b.c",
		"@example.com",
		"user@",
		"user@@example.com",
		"user@.example.com",
		"user@example.com.",
		" ",
	}
	for _, s := range invalid {
		if Valid(s) {
			t.Errorf("Valid(%q) = true, want false", s)
		}
	}
}

func TestValidLengthBounds(t *testing.T) {
	longLocal := strings.Repeat("a", 65) + "@example.com"
	if Valid(longLocal) {
		t.Errorf("Valid(local > 64) = true, want false")
	}
	longDomain := "u@" + strings.Repeat("a", 253) + ".com"
	if Valid(longDomain) {
		t.Errorf("Valid(domain > 253) = true, want false")
	}
}

func TestNormalize(t *testing.T) {
	if got, want := Normalize("  User@Example.COM "), "user@example.com"; got != want {
		t.Errorf("Normalize() = %q, want %q", got, want)
	}
}
