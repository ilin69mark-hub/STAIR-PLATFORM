package envguard

import "testing"

func TestValidate(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"production", "production", false},
		{" PROD ", "prod", false},
		{"Development", "development", false},
		{"staging", "staging", false},
		{"test", "test", false},
		{"", "", true},
		{"   ", "", true},
		{"prodction", "", true},   // опечатка
		{"productionn", "", true}, // опечатка
		{"prd", "", true},
		{"localhost", "", true},
	}
	for _, c := range cases {
		got, err := Validate(c.in)
		if c.wantErr {
			if err == nil {
				t.Fatalf("Validate(%q): want error, got %q", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Fatalf("Validate(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestIsProduction(t *testing.T) {
	for _, v := range []string{"production", "PROD", " prod "} {
		if !IsProduction(v) {
			t.Fatalf("IsProduction(%q) = false", v)
		}
	}
	for _, v := range []string{"", "development", "staging", "prd", "localhost"} {
		if IsProduction(v) {
			t.Fatalf("IsProduction(%q) = true", v)
		}
	}
}
