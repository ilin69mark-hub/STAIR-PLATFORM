package cad

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	dommfg "stairplatform/internal/domain/manufacturing"
)

// СПЕЦИФИКАЦИЯ (Фаза 0.6).
//
// Отличие от ведомости наклеек. Наклейки — поименный перечень: строка на
// каждую деталь со стикером и местом. Спецификация — позиционный перечень:
// строка на позицию (вид детали), с количеством и перечнем стикеров,
// входящих в позицию.
//
// Вместе они отвечают на вопрос монтажника с двух сторон:
//
//	«что это за деталь» → наклейка (номер → место);
//	«сколько таких и из чего» → спецификация (позиция → количество, материал,
//	габариты, стикеры).
//
// Колонка стикеров в спецификации обязательна: без неё строка отвечает
// «15 проступей», но не отвечает, какие наклейки в эту пятнашку входят, и
// связать спецификацию с наклейками не по чему.

// SpecColumns — заголовок спецификации. Порядок зафиксирован, как у наклеек:
// документ читают и люди, и шаблоны печати.
var SpecColumns = []string{
	"поз", "деталь", "материал", "толщина_мм", "длина_мм", "ширина_мм", "колво", "стикеры",
}

// WriteSpec — спецификация изделия в CSV.
//
// Строки — в порядке BOM: он детерминирован по первому появлению позиции.
// Стикеры позиции — через запятую в одной ячейке, в том же порядке, в каком
// детали пришли из разложения.
func WriteSpec(w io.Writer, pkg *dommfg.ManufacturingPackage) error {
	if pkg == nil {
		return ErrNoPackage
	}
	out := csv.NewWriter(w)
	out.Comma = ';'
	if err := out.Write(SpecColumns); err != nil {
		return fmt.Errorf("cad: spec header: %w", err)
	}
	for _, l := range pkg.BOM.Lines {
		stickers := make([]string, 0, len(l.Numbers))
		for _, n := range l.Numbers {
			stickers = append(stickers, string(n))
		}
		row := []string{
			fmt.Sprintf("%d", l.Number),
			l.Description,
			string(l.MaterialCode),
			formatMM(l.Thickness.Millimeters()),
			formatMM(l.Length.Millimeters()),
			formatMM(l.Width.Millimeters()),
			fmt.Sprintf("%d", l.Quantity),
			strings.Join(stickers, ", "),
		}
		if err := out.Write(row); err != nil {
			return fmt.Errorf("cad: spec row %d: %w", l.Number, err)
		}
	}
	out.Flush()
	return out.Error()
}
