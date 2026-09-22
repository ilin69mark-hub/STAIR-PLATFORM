package http

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// nonRESTRoutes — маршруты, не входящие в REST-спецификацию (мета/инфра).
var nonRESTRoutes = map[string]bool{
	"/":                          true,
	"/ws":                        true,
	"/ws/admin":                  true,
	"/swagger":                   true,
	"/docs/openapi/swagger.yaml": true,
	"/n/swagger.yaml":            true,
}

// syncRoutes извлекает (method, path) из router-регистраций транспортного
// пакета: .Handle(HANDLER)("METHOD /api/v1/..."). Тот же регэксп, что в
// hack/gen_swagger.py — источник правды один.
func syncRoutes(t *testing.T) (map[string]map[string]bool, []string) {
	t.Helper()
	re := regexp.MustCompile(`\.Handle(Func)?\(("(?:(?:POST|GET|PUT|PATCH|DELETE|OPTIONS)\s+)?[^"]+")`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	out := map[string]map[string]bool{}
	var rpc []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src := readDisk(t, f)
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			raw := strings.Trim(m[2], `"`)
			var met, path string
			if i := strings.IndexRune(raw, ' '); i >= 0 {
				met, path = raw[:i], raw[i+1:]
			} else {
				met, path = "GET", raw
			}
			path = strings.TrimPrefix(path, "/api/v1")
			if strings.Contains(path, ":") {
				rpc = append(rpc, met+" "+path)
				continue
			}
			path = strings.ReplaceAll(path, "{key...}", "{key}")
			if nonRESTRoutes[path] {
				continue
			}
			if out[path] == nil {
				out[path] = map[string]bool{}
			}
			out[path][met] = true
		}
	}
	return out, rpc
}

// specPaths извлекает (path → методы) и RPC-маршруты из окружённого spec.
func specPaths(t *testing.T) (map[string]map[string]bool, []string) {
	t.Helper()
	spec := readEmbed(t, "swagger/swagger.yaml")
	pathRe := regexp.MustCompile(`(?m)^  (/\S+):\s*$`)
	rpcRe := regexp.MustCompile(`(?m)^  - "(POST|GET|PUT|PATCH|DELETE)\s+(\S+)"\s*$`)
	methods := map[string]map[string]bool{}
	rpcMatches := rpcRe.FindAllStringSubmatch(spec, -1)
	rpc := make([]string, 0, len(rpcMatches))
	for _, m := range rpcMatches {
		rpc = append(rpc, m[1]+" "+m[2])
	}
	for _, line := range pathRe.FindAllStringSubmatch(spec, -1) {
		p := line[1]
		if strings.HasPrefix(p, "x-") {
			continue
		}
		if methods[p] == nil {
			methods[p] = map[string]bool{}
		}
	}
	// методы: собираем по блокам "    <verb>:" под каждым путём.
	cur := ""
	for _, ln := range strings.Split(spec, "\n") {
		pm := pathRe.FindStringSubmatch(ln)
		if pm != nil && strings.HasPrefix(pm[1], "/") {
			cur = pm[1]
			continue
		}
		vm := regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`).FindStringSubmatch(ln)
		if vm != nil && methods[cur] != nil {
			methods[cur][strings.ToUpper(vm[1])] = true
		}
	}
	return methods, rpc
}

// TestSwaggerSpecMatchesRoutes — P2-13: окружённая OpenAPI-спецификация не
// должна расходиться с фактическими маршрутами роутера. При добавлении
// роута обновите spec: python3 hack/gen_swagger.py --write.
func TestSwaggerSpecMatchesRoutes(t *testing.T) {
	code, codeRPC := syncRoutes(t)
	spec, specRPC := specPaths(t)

	if len(code) < 60 {
		t.Fatalf("подозрительно мало маршрутов (%d); регэксп сломан?", len(code))
	}

	var missing, extra []string
	for p, ms := range code {
		if sp := spec[p]; sp != nil {
			for m := range ms {
				if !sp[m] {
					missing = append(missing, m+" "+p)
				}
			}
			continue
		}
		missing = append(missing, "all /"+p)
	}
	for p, ms := range spec {
		if cm := code[p]; cm != nil {
			for m := range ms {
				if !cm[m] {
					extra = append(extra, m+" "+p)
				}
			}
		} else {
			extra = append(extra, "all /"+p)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("spec расходится с роутером.\n  в коде, но нет в spec: %v\n  в spec, но нет в коде: %v\n"+
			"правка: python3 hack/gen_swagger.py --write", missing, extra)
	}

	codeRPCNorm := map[string]bool{}
	for _, r := range codeRPC {
		codeRPCNorm[r] = true
	}
	for _, r := range specRPC {
		if !codeRPCNorm[r] {
			t.Fatalf("RPC-маршрут %q в x-rpc-routes отсутствует в коде", r)
		}
	}
}

func readDisk(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name) //nolint:gosec // контролируемый тестовый путь
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

func readEmbed(t *testing.T, name string) string {
	t.Helper()
	b, err := swaggerFS.ReadFile(name)
	if err != nil {
		t.Skipf("spec %s: %v", name, err)
	}
	return string(b)
}
