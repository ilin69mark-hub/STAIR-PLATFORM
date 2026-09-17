package validation

import "testing"

func TestValidateEmail(t *testing.T) {
	cases := []struct {
		name  string
		email string
		want  bool
	}{
		{"empty", "", false},
		{"spaces only", "   ", false},
		{"simple valid", "user@example.com", true},
		{"uppercase normalized", "User@Example.COM", true},
		{"plus tag", "user+tag@example.com", true},
		{"dot and hyphen", "first.last-name@example-domain.com", true},
		{"underscore", "user_name@example.com", true},
		{"with trim", "  user@example.com  ", true},
		{"missing @", "userexample.com", false},
		{"multiple @", "a@b@c.com", false},
		{"empty local", "@example.com", false},
		{"empty domain", "user@", false},
		{"no dot in domain", "user@localhost", false},
		{"domain starts with dot", "user@.example.com", false},
		{"domain ends with dot", "user@example.com.", false},
		{"too long", string(make([]byte, 255)), false},
		{"long valid borderline", func() string {
			local := string(make([]byte, 64))
			for i := range local {
				local = local[:i] + "a" + local[i+1:]
			}
			return local + "@example.com"
		}(), true},
		{"local too long", string(make([]byte, 65)) + "@example.com", false},
		{"invalid chars", "us er@example.com", false},
		{"single char TLD invalid", "user@example.c", false},
		{"numeric domain", "user@123.example.com", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ValidateEmail(c.email)
			if got != c.want {
				t.Fatalf("ValidateEmail(%q)=%v want %v", c.email, got, c.want)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  User@Example.COM  ", "user@example.com"},
		{"USER@EXAMPLE.COM", "user@example.com"},
		{"", ""},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := NormalizeEmail(c.in); got != c.want {
			t.Fatalf("NormalizeEmail(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestValidateEmailLongDomain(t *testing.T) {
	// domain >253 chars should fail
	domain := ""
	for len(domain) <= 254 {
		domain += "a."
	}
	domain += "com"
	email := "user@" + domain
	if ValidateEmail(email) {
		t.Fatal("expected invalid for domain >253")
	}
}
