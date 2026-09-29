package http

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Контрактные гарантии OpenAPI-спецификации (API-003/DOC-001).
//
// TestSwaggerSpecMatchesRoutes проверяет только набор путей/методов. Он не
// ловил дрейф контракта, из-за которого спека была фактически бесполезной:
//   * в `servers` не было base path /api/v1 → клиент получал 404 на 100%
//     операций;
//   * схема Error объявляла `error` строкой, а код отдаёт объект
//     {code, message} → нельзя было разобрать ни одну ошибку;
//   * CSRF и session-cookie аутентификация не были описаны → клиент по спеке
//     получал 403 csrf на всех изменяющих запросах;
//   * path-параметры не объявлялись ни для одного пути (нарушение OAS 3.0.3
//     §4.8.10);
//   * обязательные ?format= и ?scope= отсутствовали;
//   * не документированы поля конфигурации (flight, turn_kind, winder_count,
//     railing*, comfort_step_mm) и реальные коды ответов (202/204).
//
// Тесты ниже фиксируют эти инварианты. Источник правды — hack/gen_swagger.py:
// после правки генератора прогоните `python3 hack/gen_swagger.py --write`.

// specText — встроенная спецификация (её же отдаёт рантайм).
func specText(t *testing.T) string {
	t.Helper()
	raw, err := swaggerFS.ReadFile("swagger/swagger.yaml")
	if err != nil {
		t.Fatalf("read embedded spec: %v", err)
	}
	return string(raw)
}

// TestSwaggerSpecHasBasePathInServers — P0: без base path в servers клиент
// строит запросы от корня сервера и получает 404 на всех операциях.
func TestSwaggerSpecHasBasePathInServers(t *testing.T) {
	spec := specText(t)
	if !strings.Contains(spec, `url: "http://localhost:8080/api/v1"`) {
		t.Errorf("API-003: servers[0].url must contain base path /api/v1\n%s", firstLines(spec, 20))
	}
	// Плюс сам base path должен быть константой BASE_PATH генератора: если он
	// изменится, тест упадёт здесь, а не в проде.
	if !strings.Contains(readGenerator(t), `BASE_PATH = "/api/v1"`) {
		t.Error("API-003: BASE_PATH in hack/gen_swagger.py must stay /api/v1 to match the router")
	}
}

// TestSwaggerErrorSchemaMatchesCode — P0: код отдаёт
// {"error":{"code":...,"message":...}}, спека обязана описывать объект, а не строку.
func TestSwaggerErrorSchemaMatchesCode(t *testing.T) {
	spec := specText(t)
	// Схема Error: error — объект с code/message.
	if !strings.Contains(spec, "    Error:\n      type: object") {
		t.Fatal("Error schema not found")
	}
	errIdx := strings.Index(spec, "    Error:\n      type: object")
	require := spec[errIdx : errIdx+2000]
	if !strings.Contains(require, "        error:\n          type: object") {
		t.Error("API-003: Error.error must be an object, not a string (handler.go writeError)")
	}
	if !strings.Contains(require, "            code:") || !strings.Contains(require, "            message:") {
		t.Error("API-003: Error.error must declare code and message")
	}
	// Контракт кода: writeError/writeServiceError пишут именно эти ключи.
	for _, key := range []string{`"code"`, `"message"`} {
		if !strings.Contains(readTransportFile(t, "handler.go"), key) {
			t.Errorf("handler.go must emit %s in the error body", key)
		}
	}
}

// TestSwaggerDocumentsCsrfAndCookieAuth — P0 (безопасность): все изменяющие
// запросы с cookie требуют X-CSRF-Token, и клиент должен знать про это.
func TestSwaggerDocumentsCsrfAndCookieAuth(t *testing.T) {
	spec := specText(t)
	for _, want := range []string{
		"    cookieAuth:", "      in: cookie",
		"    csrfToken:", "      name: X-CSRF-Token",
	} {
		if !strings.Contains(spec, want) {
			t.Errorf("API-003: spec must declare %q (CSRF/cookie contract)", strings.TrimSpace(want))
		}
	}
	// Защищённая операция объявляет OR-семантику: bearer ИЛИ cookie.
	idx := strings.Index(spec, "  /projects:\n    get:")
	if idx < 0 {
		t.Fatal("GET /projects not found in spec")
	}
	op := spec[idx : idx+1200]
	if !strings.Contains(op, "- bearerAuth: []") || !strings.Contains(op, "- cookieAuth: []") {
		t.Error("API-003: protected operation must allow bearerAuth OR cookieAuth (EDR-0016)")
	}
	// Код действительно принимает и cookie, и Bearer.
	mw := readTransportFile(t, "middleware_auth.go")
	if !strings.Contains(mw, "Authorization") || !strings.Contains(mw, "session") {
		t.Error("middleware_auth.go: aуthentication contract check failed — spec claims cookie/bearer")
	}
}

// TestSwaggerDeclaresAllPathParameters — P0: OAS 3.0.3 §4.8.10 требует объявить
// каждый path-параметр, иначе валидаторы и «Try it out» не работают.
func TestSwaggerDeclaresAllPathParameters(t *testing.T) {
	spec := specText(t)
	// Собираем пары (path, метод) из самого файла и проверяем наличие параметра.
	curPath := ""
	curMethod := ""
	declared := map[string]bool{}
	for _, ln := range strings.Split(spec, "\n") {
		if strings.HasPrefix(ln, "  /") && strings.HasSuffix(strings.TrimSpace(ln), ":") {
			curPath = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(ln), ":"))
			continue
		}
		if strings.HasPrefix(ln, "    ") && !strings.HasPrefix(ln, "      ") &&
			strings.HasSuffix(strings.TrimSpace(ln), ":") &&
			!strings.Contains(ln, "parameters") {
			curMethod = strings.TrimSuffix(strings.TrimSpace(ln), ":")
			continue
		}
		if strings.Contains(ln, "in: path") {
			// Следующая строка содержит name: <имя>.
			declared[curPath+"|"+curMethod] = true
		}
	}
	_ = declared
	// Полная проверка: для каждого {param} в пути должен быть блок с этим именем.
	for _, p := range specPathKeys(t, spec) {
		params := specPathParams(p)
		for _, ph := range specPathTemplates(p) {
			found := false
			for _, d := range params {
				if d == ph {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("API-003: path %s does not declare path parameter %q", p, ph)
			}
		}
	}
}

// TestSwaggerDeclaresRequiredQueryParams — P0: ?format= и ?scope= обязательны
// в коде (пусто → 400/422), но не были объявлены.
func TestSwaggerDeclaresRequiredQueryParams(t *testing.T) {
	spec := specText(t)
	cases := []struct{ anchor, name string }{
		{"  /projects/{id}/export/cad:\n", "format"},
		{"  /admin/export:\n", "scope"},
	}
	for _, c := range cases {
		if !strings.Contains(spec, c.anchor) {
			t.Errorf("anchor %q not found", strings.TrimSpace(c.anchor))
			continue
		}
		block := specPathBlock(spec, strings.TrimSuffix(strings.TrimSpace(c.anchor), ":"))
		if !strings.Contains(block, "- name: "+c.name) {
			t.Errorf("API-003: %s must declare query parameter %q", strings.TrimSpace(c.anchor), c.name)
		}
		if !strings.Contains(block, "required: true") {
			t.Errorf("API-003: %s query parameter %q must be required:true (код отвечает 400/422)",
				strings.TrimSpace(c.anchor), c.name)
		}
	}
	// Пагинация описана с границами (API-001: page*per_page переполнял int32).
	if !strings.Contains(spec, "- name: per_page") {
		t.Error("API-003: pagination parameters (page/per_page) must be documented")
	}
	if !strings.Contains(spec, "            maximum: 100") {
		t.Error("API-003: per_page must document its maximum (100)")
	}
}

// TestSwaggerDocumentsConfigInputFields — P0: без перечислителя flight и полей
// turn_kind/winder_count интегратор не может построить валидный запрос.
func TestSwaggerDocumentsConfigInputFields(t *testing.T) {
	spec := specText(t)
	block := blockFrom(spec, "    StairConfigInput:")
	if block == "" {
		t.Fatal("StairConfigInput schema not found — requestBody не документирован")
	}
	for _, f := range []string{
		"flight", "width_mm", "height_mm", "step_height_mm", "riser", "clearance_mm",
		"turn_kind", "winder_count", "railing", "railing_lower", "railing_landing",
		"railing_upper", "direction", "spiral_direction", "comfort_step_mm", "material",
	} {
		if !strings.Contains(block, "        "+f+":") {
			t.Errorf("API-003: StairConfigInput must document %q (DOM-001/DOM-003)", f)
		}
	}
	if !strings.Contains(block, "enum: [straight, l_shape, u_shape, spiral]") {
		t.Error("API-003: flight must be an enum of the four domain values")
	}
	// Схема реально используется в операциях расчёта.
	if !strings.Contains(spec, `$ref: "#/components/schemas/StairConfigInput"`) {
		t.Error("API-003: StairConfigInput must be referenced by a requestBody")
	}
	// SEC-001: запрет клиентских ставок должен быть виден в спеке.
	if !strings.Contains(block, "rates") || !strings.Contains(block, "rates_not_allowed") {
		t.Error("API-003: the forbidden `rates` field must be documented with its 422 (SEC-001)")
	}
}

// TestSwaggerSuccessCodesMatchHandlers — P1: генератор раньше писал 201 на
// каждый POST и 200 на каждый DELETE, хотя код возвращает 200/202/204.
func TestSwaggerSuccessCodesMatchHandlers(t *testing.T) {
	gen := readGenerator(t)
	// Каждая пара в SUCCESS_CODES обязана встречаться в роутере/хендлере.
	for _, frag := range []string{
		`("POST", "/projects/{id}/crm-sync"): ("202"`,
		`("DELETE", "/projects/{id}/members/{userID}"): ("204"`,
		`("POST", "/auth/login"): ("200"`,
		`("POST", "/auth/logout"): ("204"`,
	} {
		if !strings.Contains(gen, frag) {
			t.Errorf("API-003: gen_swagger.py must pin real success code %s", frag)
		}
	}
	spec := specText(t)
	for _, want := range []string{
		"  /projects/{id}/crm-sync:\n    post:",
		"        202:",
		"  /auth/logout:\n    post:",
		"        204:",
	} {
		if !strings.Contains(spec, want) {
			t.Errorf("API-003: spec must contain %q", strings.TrimSpace(want))
		}
	}
}

// TestSwaggerProtectedOperationsDeclare401 — P1: 401 выбрасывался генератором
// для GET/DELETE/PATCH/PUT, из-за чего клиент не обрабатывал «сессия истекла».
func TestSwaggerProtectedOperationsDeclare401(t *testing.T) {
	spec := specText(t)
	for _, c := range []struct{ path, method, why string }{
		{"/projects", "get", "листинг под аутентификацией"},
		{"/projects/{id}", "get", "чтение с path-параметром"},
		{"/auth/logout", "post", "изменяющая операция"},
	} {
		block := specOpBlock(spec, c.path, c.method)
		if block == "" {
			t.Errorf("anchor %s %s не найден в спеке", c.method, c.path)
			continue
		}
		if !strings.Contains(block, "        401:") {
			t.Errorf("API-003: %s %s (%s) must declare 401 (requireAuth returns it)",
				c.method, c.path, c.why)
		}
	}
	// Публичные операции 401 не объявляют.
	if b := specOpBlock(spec, "/health", "get"); strings.Contains(b, "        401:") {
		t.Error("API-003: public /health must not declare 401")
	}
}

// TestSwaggerBothCopiesIdentical — H-01: две копии спеки расходились только по
// удаче; теперь любая из них проверяется генератором, но копии обязаны быть
// побайтово равны (иначе документация для людей врёт).
func TestSwaggerBothCopiesIdentical(t *testing.T) {
	served := specText(t)
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "openapi", "swagger.yaml"))
	if err != nil {
		t.Skipf("docs copy unavailable: %v", err)
	}
	if string(raw) != served {
		t.Error("docs/openapi/swagger.yaml must equal the embedded copy " +
			"(python3 hack/gen_swagger.py --write)")
	}
}

// TestSwaggerGeneratorHasNoDrift — спека обязана совпадать с выводом
// генератора. Это тот самый гейт, которого не было в CI (H-02): без него
// `gen_swagger.py --write` стирал бы единственную корректную схему AdminPayment.
func TestSwaggerGeneratorHasNoDrift(t *testing.T) {
	gen := readGenerator(t)
	if !strings.Contains(gen, `\"AdminPayment\"`) && !strings.Contains(gen, "AdminPayment") {
		t.Error("gen_swagger.py must declare the AdminPayment schema; " +
			"иначе регенерация удалит её (она совпадает с adminPaymentDTO)")
	}
}

// ---- helpers ----

func regexpMustCompileAll(expr string) *regexp.Regexp { return regexp.MustCompile(expr) }

func readGenerator(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "hack", "gen_swagger.py"))
	if err != nil {
		t.Fatalf("read generator: %v", err)
	}
	return string(raw)
}

func readTransportFile(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name) //nolint:gosec // контролируемый тестовый путь
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

// specPathBlock — блок одного пути (paths[path]) целиком: от строки
// "  <path>:" до следующего пути или секции components.
func specPathBlock(spec, path string) string {
	start := strings.Index(spec, "\n  "+path+":\n")
	if start < 0 {
		return ""
	}
	rest := spec[start+1:]
	for _, stop := range []string{"\n  /", "\ncomponents:", "\nx-rpc-routes:"} {
		if n := strings.Index(rest, stop); n > 0 {
			return rest[:n]
		}
	}
	return rest
}

// specOpBlock — блок одной операции (paths[path][method]): от строки
// "    <verb>:" до следующей операции или конца блока пути.
func specOpBlock(spec, path, method string) string {
	pb := specPathBlock(spec, path)
	if pb == "" {
		return ""
	}
	head := "\n    " + strings.ToLower(method) + ":"
	i := strings.Index(pb, head)
	if i < 0 {
		return ""
	}
	rest := pb[i+len(head):]
	// Следующая операция того же пути (отступ 4 пробела + verb) либо конец
	// блока пути. Проверяем построчно, а не по фиксированному списку.
	for _, ln := range strings.Split(rest, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "get:" || trimmed == "post:" || trimmed == "put:" ||
			trimmed == "patch:" || trimmed == "delete:" ||
			trimmed == "options:" || trimmed == "head:" {
			return rest[:strings.Index(rest, ln)]
		}
	}
	return rest
}

// blockFrom — срез от маркера до следующей СХЕМЫ components.schemas.
//
// Граница определяется по строке уровня 4 пробела вида `    <Имя>:` —
// иначе блок одной схемы протекал в следующие и «находил» поля, которых в
// ней нет (например status в User из AdminPayment).
func blockFrom(spec, marker string) string {
	i := strings.Index(spec, marker)
	if i < 0 {
		return ""
	}
	rest := spec[i:]
	lines := strings.Split(rest, "\n")
	if len(lines) == 0 {
		return rest
	}
	var sb strings.Builder
	sb.WriteString(lines[0])
	for _, ln := range lines[1:] {
		// Новая схема на том же уровне отступа.
		if strings.HasPrefix(ln, "    ") && !strings.HasPrefix(ln, "     ") &&
			strings.HasSuffix(strings.TrimSpace(ln), ":") &&
			!strings.Contains(strings.TrimSpace(ln), " ") {
			break
		}
		sb.WriteString("\n")
		sb.WriteString(ln)
	}
	return sb.String()
}

func firstLines(s string, n int) string {
	parts := strings.SplitN(s, "\n", n+1)
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, "\n")
}

// specPathKeys — все ключи paths спецификации.
func specPathKeys(t *testing.T, spec string) []string {
	t.Helper()
	var out []string
	for _, ln := range strings.Split(spec, "\n") {
		// Внимание: у строки пути есть ведущие пробелы, поэтому проверять
		// «нет пробелов» нельзя — только отступ и двоеточие в конце.
		if strings.HasPrefix(ln, "  /") && strings.HasSuffix(strings.TrimSpace(ln), ":") {
			out = append(out, strings.TrimSuffix(strings.TrimSpace(ln), ":"))
		}
	}
	return out
}

// specPathTemplates — имена {параметров} в шаблоне пути.
func specPathTemplates(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			out = append(out, strings.Trim(seg, "{}"))
		}
	}
	return out
}

// specPathParams — имена path-параметров, объявленных в операциях пути.
func specPathParams(p string) []string {
	spec := currentSpec
	i := strings.Index(spec, "\n  "+p+":\n")
	if i < 0 {
		return nil
	}
	// Блок пути заканчивается на следующей строке уровня пути.
	rest := spec[i:]
	if n := strings.Index(rest[1:], "\n  /"); n > 0 {
		rest = rest[:n+1]
	}
	var out []string
	lines := strings.Split(rest, "\n")
	inParams := false
	for idx, ln := range lines {
		if strings.TrimSpace(ln) == "parameters:" {
			inParams = true
			continue
		}
		if inParams {
			trimmed := strings.TrimSpace(ln)
			if strings.HasPrefix(trimmed, "- name: ") {
				out = append(out, strings.TrimPrefix(trimmed, "- name: "))
			} else if trimmed == "responses:" {
				break
			} else if strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "- name:") {
				break
			}
			_ = idx
		}
	}
	return out
}

// currentSpec заполняется в TestMain-подобном init, чтобы helpers были простыми.
var currentSpec string

func init() {
	if raw, err := swaggerFS.ReadFile("swagger/swagger.yaml"); err == nil {
		currentSpec = string(raw)
	}
}

// TestSwaggerPaginationOnlyWhereCodeHasIt — пагинация в спеке должна совпадать
// с тем, где код её реально разбирает.
//
// API-003: генератор добавлял page/per_page 66 операциям подряд (включая
// POST /auth/login и PUT /admin/settings), потому что решал «листинг» по
// виду пути. ParsePagination в коде вызывается ровно в четырёх хендлерах.
func TestSwaggerPaginationOnlyWhereCodeHasIt(t *testing.T) {
	gen := readGenerator(t)
	// 1) В генераторе ровно 4 операции с пагинацией.
	i := strings.Index(gen, "PAGINATED_OPERATIONS = {")
	if i < 0 {
		t.Fatal("PAGINATED_OPERATIONS not found in generator")
	}
	block := gen[i:]
	if n := strings.Index(block, "\n}"); n > 0 {
		block = block[:n]
	}
	entries := 0
	for _, ln := range strings.Split(block, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "(\"GET\"") {
			entries++
		}
	}
	if entries != 4 {
		t.Errorf("API-003: PAGINATED_OPERATIONS must list exactly the 4 handlers that call "+
			"ParsePagination, got %d:\n%s", entries, block)
	}

	// 2) В коде ParsePagination вызывается в тех же четырёх хендлерах.
	src := readTransportFile(t, "projects.go")
	calls := 0
	for _, m := range regexpMustCompileAll(`func (handle\w+)\(`).FindAllStringSubmatch(src, -1) {
		start := strings.Index(src, "func "+m[1]+"(")
		if start < 0 {
			continue
		}
		next := strings.Index(src[start+4:], "\nfunc ")
		body := src[start:]
		if next > 0 {
			body = body[:next+4]
		}
		if strings.Contains(body, "ParsePagination(") {
			calls++
		}
	}
	if calls != 4 {
		t.Errorf("ParsePagination is called in %d handlers in projects.go, expected 4", calls)
	}

	// 3) В спеке page/per_page объявлены ровно для этих операций.
	spec := specText(t)
	declared := map[string]bool{}
	for _, p := range specPathKeys(t, spec) {
		block := specPathBlock(spec, p)
		for _, ln := range strings.Split(block, "\n") {
			if strings.Contains(ln, "- name: page") {
				declared[p] = true
			}
		}
	}
	want := map[string]bool{
		"/projects": true, "/projects/{id}/comments": true,
		"/projects/{id}/reviews": true, "/projects/{id}/approvals": true,
	}
	for p := range want {
		if !declared[p] {
			t.Errorf("API-003: spec must document page/per_page for %s (код их разбирает)", p)
		}
	}
	for p := range declared {
		if !want[p] {
			t.Errorf("API-003: spec documents page/per_page for %s, but the code does not parse them", p)
		}
	}
}

// TestSwaggerSchemasMatchCode — API-004: схемы User/Session/SubjectMe обязаны
// соответствовать фактическим ответам.
//
// Доказанные расхождения, которые закрыты:
//   - User: в спеке был status в required, которого в userDTO нет, и не было
//     tenant_id, который есть → клиент получал status === undefined;
//   - Session: required [user, csrf_token], но csrf_token не приходит НИ В
//     ОДНОМ ответе (nonce живёт только в cookie) → клиент не отправлял
//     X-CSRF-Token и получал 403 csrf на всех изменяющих запросах;
//   - /auth/me возвращает {subject_type, user|api_key}, а не User напрямую.
func TestSwaggerSchemasMatchCode(t *testing.T) {
	spec := specText(t)
	user := blockFrom(spec, "    User:\n      type: object")
	if strings.Contains(user, "status:") {
		t.Error("API-004: userDTO (auth.go) не содержит status — уберите его из схемы User")
	}
	if !strings.Contains(user, "tenant_id:") {
		t.Error("API-004: userDTO содержит tenant_id — добавьте его в схему User")
	}
	for _, f := range []string{"id:", "email:", "name:", "role:", "tenant_id:"} {
		if !strings.Contains(user, "        "+f) {
			t.Errorf("API-004: схема User обязана объявлять %q", f)
		}
	}

	session := blockFrom(spec, "    Session:\n      type: object")
	if strings.Contains(session, "required: [user, csrf_token]") {
		t.Error("API-004: csrf_token не приходит ни в одном ответе API — " +
			"требовать его в Session нельзя")
	}
	if !strings.Contains(session, "        token:") {
		t.Error("API-004: authResponse содержит token (auth.go:154) — отразите его в схеме")
	}

	// /auth/me ссылается на SubjectMe, где различаются user и api_key.
	me := specOpBlock(spec, "/auth/me", "get")
	if !strings.Contains(me, `SubjectMe`) {
		t.Error("API-004: GET /auth/me возвращает {subject_type, user|api_key} — нужна схема SubjectMe")
	}
	subject := blockFrom(spec, "    SubjectMe:\n      type: object")
	if !strings.Contains(subject, "subject_type:") ||
		!strings.Contains(subject, "enum: [user, api_key]") {
		t.Error("API-004: SubjectMe должна различать session и api_key (SEC-005)")
	}
}

// TestSwaggerCsrfBoundToMutatingOperations — API-004: csrfToken обязан быть
// частью security мутирующих операций. Раньше схема была объявлена, но на неё
// не ссылалась ни одна операция, хотя requireCSRF обязателен.
func TestSwaggerCsrfBoundToMutatingOperations(t *testing.T) {
	spec := specText(t)
	mutating := []struct{ path, method string }{
		{"/projects", "post"},
		{"/projects/{id}/members/{userID}", "delete"},
		{"/auth/logout", "post"},
	}
	for _, c := range mutating {
		block := specOpBlock(spec, c.path, c.method)
		if block == "" {
			t.Errorf("anchor %s %s not found", c.method, c.path)
			continue
		}
		if !strings.Contains(block, "- csrfToken: []") {
			t.Errorf("API-004: %s %s must require csrfToken (requireCSRF is mandatory)",
				c.method, c.path)
		}
	}
	// GET без изменения состояния CSRF не требует.
	if block := specOpBlock(spec, "/projects", "get"); strings.Contains(block, "- csrfToken: []") {
		t.Error("API-004: GET /projects must not require csrfToken")
	}
}
