package cad

import (
	"bufio"
	"fmt"
	"io"
	"sort"

	dommfg "stairplatform/internal/domain/manufacturing"
	kerngeo "stairplatform/internal/geometry"
)

// СБОРОЧНЫЙ ЧЕРТЁЖ (Фаза 0.5).
//
// Что это и чем отличается от writeSVG. writeSVG рисует wireframe меша: вид
// сверху без единого номера. По нему нельзя собрать изделие — на нём нет
// ответа «какая деталь где». Сборочный чертёж рисует то же сверху, но
// добавляет к каждой детали её НОМЕР на месте детали. Номер берётся из
// Placement детали, а не из порядка в списке: чертёж и положение обязаны
// совпадать, иначе монтажник ставит деталь 7 туда, где на чертеже деталь 4.
//
// Вид — сверху, как и у writeSVG: лестница собирается снизу вверх, и план
// сверху — это вид монтажника, стоящего у первой ступени. (Разрезы и вид
// сбоку с размерами — отдельная работа, не эта.)
//
// Формат — SVG: векторный, масштабируется, открывается в браузере без
// редактора. PDF собирается из того же SVG в браузере печатью (Фаза 0.4,
// тот же механизм, что и лист наклеек): отдельный PDF-писатель на сервере
// означал бы вторую реализацию вёрстки, а две вёрстки разъезжаются.

// labeledPart — деталь с местом для подписи: центр в проекции и номер.
type labeledPart struct {
	number string
	where  string
	x, y   float64
}

// WriteAssembly — сборочный чертёж: вид сверху с номерами деталей.
//
// parts — детали пакета со своим Placement. Детали без положения (Known =
// false, покупные ограждения) не рисуются меткой на плане — их места сервер
// не знает. Они перечислены отдельным списком под планом, чтобы было видно,
// что их нет на чертеже, а не что их нет вообще.
func WriteAssembly(w io.Writer, mesh *kerngeo.Mesh, parts []dommfg.Part) error {
	if mesh == nil {
		return ErrEmptyMesh
	}
	bw := bufio.NewWriter(w)
	if _, err := bw.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"); err != nil {
		return err
	}
	edges := collectEdges(mesh)
	sortEdges(edges)
	minX, minY, maxX, maxY := bbox2D(mesh)
	pad := 4.0
	vb := fmt.Sprintf("%.3f %.3f %.3f %.3f", minX-pad, minY-pad,
		(maxX-minX)+2*pad, (maxY-minY)+2*pad)
	if _, err := fmt.Fprintf(bw,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="%s" width="1000" height="700">`+"\n"+`<title>Сборочный чертёж: вид сверху с номерами деталей</title>`+"\n",
		vb); err != nil {
		return err
	}
	if _, err := bw.WriteString(`<g fill="none" stroke="#1f2937" stroke-width="0.5">` + "\n"); err != nil {
		return err
	}
	for _, e := range edges {
		ax, ay := project(e.a.X, e.a.Y)
		bx, by := project(e.b.X, e.b.Y)
		if _, err := fmt.Fprintf(bw, "<line x1=\"%.3f\" y1=\"%.3f\" x2=\"%.3f\" y2=\"%.3f\"/>\n",
			ax, ay, bx, by); err != nil {
			return err
		}
	}
	if _, err := bw.WriteString("</g>\n"); err != nil {
		return err
	}

	labeled, unlabeled := splitByPlacement(parts)
	if _, err := bw.WriteString("<g>\n"); err != nil {
		return err
	}
	for _, p := range labeled {
		// Подпись: номер крупно, место мелко под ним. Якорь по центру текста,
		// чтобы номер стоял ровно над центром детали, а не съезжал на соседнюю.
		if _, err := fmt.Fprintf(bw,
			"<text x=\"%.3f\" y=\"%.3f\" text-anchor=\"middle\" font-size=\"14\" font-weight=\"bold\" fill=\"#b91c1c\">%s</text>\n",
			p.x, p.y, escapeXML(p.number)); err != nil {
			return err
		}
		if p.where != "" {
			if _, err := fmt.Fprintf(bw,
				"<text x=\"%.3f\" y=\"%.3f\" text-anchor=\"middle\" font-size=\"8\" fill=\"#374151\">%s</text>\n",
				p.x, p.y+11, escapeXML(p.where)); err != nil {
				return err
			}
		}
	}
	if _, err := bw.WriteString("</g>\n"); err != nil {
		return err
	}

	// Детали без положения — отдельным списком. Их нельзя нарисовать на
	// плане (сервер не знает, где они стоят), но скрывать их нельзя: тогда
	// чертёж выглядел бы полным, хотя части деталей на нём нет.
	if len(unlabeled) > 0 {
		y := maxY + 40
		if _, err := fmt.Fprintf(bw,
			"<text x=\"%.3f\" y=\"%.3f\" font-size=\"12\" font-weight=\"bold\" fill=\"#111827\">"+
				"Без места на плане (покупные, ставятся по месту):</text>\n",
			minX, y); err != nil {
			return err
		}
		y += 16
		names := append([]string(nil), unlabeled...)
		sort.Strings(names)
		if _, err := fmt.Fprintf(bw,
			"<text x=\"%.3f\" y=\"%.3f\" font-size=\"10\" fill=\"#374151\">%s</text>\n",
			minX, y, escapeXML(joinComma(names))); err != nil {
			return err
		}
	}
	if _, err := bw.WriteString("</svg>\n"); err != nil {
		return err
	}
	return bw.Flush()
}

// splitByPlacement — детали с положением (для меток на плане) и без него.
//
// Центр детали для метки — проекция Placement.Center на плоскость XY.
// Брать центр габарита вместо Placement означало бы, что чертёж и
// положение детали разъедутся при первой же детали с нестандартной формой.
func splitByPlacement(parts []dommfg.Part) (labeled []labeledPart, unlabeled []string) {
	for _, p := range parts {
		if !p.Placement.Known {
			unlabeled = append(unlabeled, string(p.Number))
			continue
		}
		c := p.Placement.Center(p.Length.Millimeters(), p.Width.Millimeters())
		x, y := project(c.X, c.Y)
		labeled = append(labeled, labeledPart{
			number: string(p.Number),
			where:  p.Where,
			x:      x,
			y:      y,
		})
	}
	return labeled, unlabeled
}

// escapeXML — экранирование для SVG-текста. Номера деталей и «куда»
// содержат кириллицу и дефисы, но не должны содержать угловых скобок;
// экранируем всё равно, потому что чертёж не имеет права ломаться от строки.
func escapeXML(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			out = append(out, "&amp;"...)
		case '<':
			out = append(out, "&lt;"...)
		case '>':
			out = append(out, "&gt;"...)
		case '"':
			out = append(out, "&quot;"...)
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}

// joinComma — перечень через запятую с пробелом.
func joinComma(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
