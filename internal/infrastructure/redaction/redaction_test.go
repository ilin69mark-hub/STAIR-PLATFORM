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
		// S-143 (S-141 №2): bearer session/csrf-токены тоже маскируются.
		`{"session":"SECRET","csrf":"TKN"}`: `{"session":"***","csrf":"***"}`,
		`session=SECRET&csrf=TKN`:           `session=***&csrf=***`,
		`{"session_admin":"SECRET"}`:        `{"session_admin":"***"}`,
		`csrf_admin=TKN`:                    `csrf_admin=***`,
		`stair_session=SECRET`:              `stair_session=***`,
		`{"stair-session":"SECRET"}`:        `{"stair-session":"***"}`,
		// обратная совместимость: близкие, но не токенные ключи не трогаем.
		`{"session_id":"keep","session_name":"n"}`: `{"session_id":"keep","session_name":"n"}`,
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
