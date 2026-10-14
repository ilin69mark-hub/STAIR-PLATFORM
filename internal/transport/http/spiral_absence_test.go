package http

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Гейт «обещание равно коду» для типа марша SPIRAL (S-183).
//
// Винтовой марш удалён из проекта целиком (решение владельца 2026-10-06), но
// обещание о нём успело доехать до трёх мест, где клиент его видит:
//
//   - SEO-текст витрины (meta description, og:description, JSON-LD);
//   - enum FlightType в GraphQL-схеме — клиент по ней строил запрос;
//   - финальный маркер EDR-0007, который гласил APPROVED, пока шапка того же
//     документа говорила SUPERSEDED.
//
// Каждое из них молча вернётся при правке «по месту», и ни один тест этого
// не ловил: отсутствие винтового марша проверялось только запросом к API.
// Поэтому здесь проверяется не поведение API (оно уже проверено в
// flight_availability_test.go), а отсутствие обещаний.

// repoPath поднимается к корню репозитория из каталога пакета.
func repoPath(t *testing.T, parts ...string) string {
	t.Helper()
	return filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
}

// TestStorefrontDoesNotPromiseSpiral — витрина не обещает винтовых лестниц.
func TestStorefrontDoesNotPromiseSpiral(t *testing.T) {
	path := repoPath(t, "frontend-store", "index.html")
	raw, err := os.ReadFile(path) //nolint:gosec // путь собран константами теста
	if err != nil {
		t.Fatalf("читаю %s: %v", path, err)
	}
	html := strings.ToLower(string(raw))
	for _, word := range []string{"винтов", "спираль"} {
		if strings.Contains(html, word) {
			t.Errorf("витрина обещает «%s», а такого типа марша в проекте нет "+
				"(удалён 2026-10-06). Публичный текст обещает продукт, которого "+
				"не будет на первом же звонке: %s", word, path)
		}
	}
}

// TestGraphQLFlightEnumHasNoSpiral — в enum FlightType схемы нет SPIRAL.
//
// Схема отдаётся клиенту, и по ней он строит запрос. Пока значение значилось
// в enum, клиент мог его прислать; резолвер конвертирует строку в тип без
// валидации, и отказ приходил из домена, а не из контракта.
func TestGraphQLFlightEnumHasNoSpiral(t *testing.T) {
	path := repoPath(t, "internal", "transport", "graphql", "schema.graphql")
	raw, err := os.ReadFile(path) //nolint:gosec // путь собран константами теста
	if err != nil {
		t.Fatalf("читаю %s: %v", path, err)
	}
	schema := string(raw)
	start := strings.Index(schema, "enum FlightType {")
	if start < 0 {
		t.Fatal("в схеме нет enum FlightType — проверка бессмысленна")
	}
	end := strings.Index(schema[start:], "}")
	if end < 0 {
		t.Fatal("enum FlightType не закрыт — схема разъехалась")
	}
	body := schema[start : start+end]
	if strings.Contains(body, "SPIRAL") {
		t.Errorf("SPIRAL вернулся в enum FlightType:\n%s", body)
	}
	// Все три значения, которые реально поддерживает домен, должны остаться:
	// молчаливое удаление L_SHAPED или U_SHAPED — обратная ошибка.
	for _, want := range []string{"STRAIGHT", "L_SHAPED", "U_SHAPED"} {
		if !strings.Contains(body, want) {
			t.Errorf("в enum FlightType нет %s:\n%s", want, body)
		}
	}
}

// TestEDR0007MarkersAgree — шапка и финальный маркер EDR-0007 говорят одно и
// то же.
//
// Расхождение делало статус документа неопределённым: сверка по маркерам
// видела в шапке SUPERSEDED, а в конце APPROVED, и документ считался то
// утверждённым, то отменённым — в зависимости от того, откуда смотрели.
func TestEDR0007MarkersAgree(t *testing.T) {
	path := repoPath(t, "docs", "EDR", "EDR-0007_Spiral_Stair_Flight.md")
	raw, err := os.ReadFile(path) //nolint:gosec // путь собран константами теста
	if err != nil {
		t.Fatalf("читаю %s: %v", path, err)
	}
	doc := string(raw)
	if !strings.Contains(doc, "**Status:** SUPERSEDED") {
		t.Error("шапка EDR-0007 не помечена SUPERSEDED")
	}
	// Финальный маркер — последняя непустая строка документа.
	lines := strings.Split(strings.TrimSpace(doc), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(last, "SUPERSEDED") {
		t.Errorf("финальный маркер EDR-0007 = %q, want начало с SUPERSEDED. "+
			"Расхождение с шапкой делает статус документа неопределённым", last)
	}
	if strings.HasSuffix(strings.TrimSpace(doc), "APPROVED") {
		t.Error("документ заканчивается на APPROVED — маркер не приведён к шапке")
	}
}
