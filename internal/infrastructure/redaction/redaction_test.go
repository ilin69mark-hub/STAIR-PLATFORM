package redaction

import "testing"

func TestSensitive(t *testing.T) {
	cases := map[string]string{ //nolint:gosec // тестовые фикстуры, не реальные секреты
		`{"password":"hunter2","email":"a@b.c"}`: `{"password":"***","email":"a@b.c"}`,
		`{"secret":  "abc", "token":"xyz"}`:      `{"secret":  "***", "token":"***"}`,
		`Authorization: Bearer abcdef`:           `Authorization: Bearer abcdef`,
		`password=Hunter2&login=admin`:           `password=***&login=admin`,
		`{"api_key":"k123","payload":[1,2]}`:     `{"api_key":"***","payload":[1,2]}`,
		`{"client_secret":"s"}`:                  `{"client_secret":"***"}`,
		`{"safe":"keep-me"}`:                     `{"safe":"keep-me"}`,
		`{"username":"name","passwd":"p"}`:       `{"username":"name","passwd":"***"}`,
	}
	for in, want := range cases {
		if got := Sensitive(in); got != want {
			t.Errorf("Sensitive(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSensitiveEmptyInput(t *testing.T) {
	if got := Sensitive(""); got != "" {
		t.Errorf("Sensitive(\"\") = %q, want empty", got)
	}
}
