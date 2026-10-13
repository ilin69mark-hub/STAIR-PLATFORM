package cad

import (
	"bytes"
	"testing"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// ВЕДОМОСТЬ НАКЛЕЕК: гейты содержимого.
//
// Наклейка — единственный способ связать деталь с местом на участке. Если в
// ведомости нет номера, нет места или не хватает строк, монтажник клеит на
// глаз — и вся работа по нумерации теряет смысл.

// labelsPackage — пакет из двух деталей: одна с местом, одна без.
func labelsPackage() *dommfg.ManufacturingPackage {
	return &dommfg.ManufacturingPackage{
		Parts: []dommfg.Part{
			{Number: "STR-01", Kind: "stringer", Material: "STEEL-S235",
				Thickness: 8, Length: 2428, Width: 300, Where: "косоур А, кусок 1 из 2"},
			{Number: "PST-01", Kind: "post", Material: "STAINLESS-304",
				Thickness: 1.5, Length: 806, Width: 42, Purchased: true},
		},
	}
}

// TestWriteLabelsListsEveryPart — в ведомости строка на каждую деталь пакета.
//
// Строк должно быть столько же, сколько деталей: каждая деталь получает
// наклейку. Меньше — значит деталь потеряна, больше — значит дубли.
func TestWriteLabelsListsEveryPart(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, labelsPackage()); err != nil {
		t.Fatalf("WriteLabels: %v", err)
	}
	rows := splitCSV(buf.String())
	// Первая строка — заголовок.
	if len(rows) != 3 {
		t.Fatalf("строк в ведомости %d (заголовок + 2 детали), want 3:\n%s", len(rows), buf.String())
	}
	// Порядок — как в пакете: наклейки печатаются в порядке сборки.
	if !containsRow(rows[1], "STR-01") || !containsRow(rows[2], "PST-01") {
		t.Errorf("порядок строк не совпадает с порядком деталей:\n%s", buf.String())
	}
}

// TestWriteLabelsCarriesWhere — место детали попадает в строку.
//
// Без «куда» наклейка — просто номер, по которому место не узнать.
func TestWriteLabelsCarriesWhere(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, labelsPackage()); err != nil {
		t.Fatalf("WriteLabels: %v", err)
	}
	if !containsRow(buf.String(), "косоур А, кусок 1 из 2") {
		t.Errorf("в строке нет места детали:\n%s", buf.String())
	}
}

// TestWriteLabelsLeavesEmptyWhereVisible — пустое «куда» остаётся пустым, а
// не превращается в прочерк или «неизвестно».
//
// Покупная деталь ограждения места не знает (тела ограждения в Model не
// входят). Прочерк выглядел бы как заполненное поле — и монтажник решил бы,
// что место указано. Пустое поле видно.
func TestWriteLabelsLeavesEmptyWhereVisible(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, labelsPackage()); err != nil {
		t.Fatalf("WriteLabels: %v", err)
	}
	for _, row := range splitCSV(buf.String())[1:] {
		if !containsRow(row, "PST-01") {
			continue
		}
		// Вторая колонка (куда) пустая.
		cols := splitRow(row)
		if len(cols) != 7 {
			t.Fatalf("в строке %d колонок, want 7", len(cols))
		}
		if cols[1] != "" {
			t.Errorf("у покупной детали «куда» = %q, want пусто (видно, что места нет)", cols[1])
		}
		return
	}
	t.Error("строки PST-01 нет в ведомости")
}

// TestWriteLabelsSeparatorsAreStable — разделитель и заголовок зафиксированы.
//
// Принтер этикеток настраивается один раз на колонки. Смена разделителя с «;»
// на «,» сломает все настроенные шаблоны печати молча: файл откроется, а
// колонки съедут.
func TestWriteLabelsSeparatorsAreStable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, labelsPackage()); err != nil {
		t.Fatalf("WriteLabels: %v", err)
	}
	lines := splitCSV(buf.String())
	if lines[0] != "номер;куда;деталь;материал;толщина_мм;длина_мм;ширина_мм" {
		t.Errorf("заголовок изменился: %q", lines[0])
	}
}

// TestWriteLabelsRequiresPackage — без пакета ведомости нет.
func TestWriteLabelsRequiresPackage(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, nil); err == nil {
		t.Fatal("ведомость без пакета не отменена")
	}
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if line != "" {
				out = append(out, line)
			}
			start = i + 1
		}
	}
	if tail := s[start:]; tail != "" {
		out = append(out, tail)
	}
	return out
}

func splitRow(s string) []string {
	var out []string
	start := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '"':
			inQuote = !inQuote
		case s[i] == ';' && !inQuote:
			out = append(out, unquote(s[start:i]))
			start = i + 1
		}
	}
	return append(out, unquote(s[start:]))
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func containsRow(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
