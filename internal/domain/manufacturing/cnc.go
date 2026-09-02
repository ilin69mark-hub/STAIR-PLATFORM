package manufacturing

import (
	"fmt"
	"time"
)

// CNCFormat — формат CNC-экспорта (MFG-0011).
type CNCFormat string

const (
	CNCFormatSTEP  CNCFormat = "step"
	CNCFormatIGES  CNCFormat = "iges"
	CNCFormatOBJ   CNCFormat = "obj"
	CNCFormat3MF   CNCFormat = "3mf"
	CNCFormatDXF   CNCFormat = "dxf"
	CNCFormatSTL   CNCFormat = "stl"
	CNCFormatCSV   CNCFormat = "csv"
	CNCFormatJSON  CNCFormat = "json"
	CNCFormatXML   CNCFormat = "xml"
)

// IsValid проверяет, что CNC-формат известен.
func (f CNCFormat) IsValid() bool {
	switch f {
	case CNCFormatSTEP, CNCFormatIGES, CNCFormatOBJ, CNCFormat3MF,
		CNCFormatDXF, CNCFormatSTL, CNCFormatCSV, CNCFormatJSON, CNCFormatXML:
		return true
	}
	return false
}

// MIME возвращает Content-Type для CNC-формата.
func (f CNCFormat) MIME() string {
	switch f {
	case CNCFormatSTEP:
		return "application/step"
	case CNCFormatIGES:
		return "application/iges"
	case CNCFormatOBJ:
		return "model/obj"
	case CNCFormat3MF:
		return "model/3mf"
	case CNCFormatDXF:
		return "application/dxf"
	case CNCFormatSTL:
		return "model/stl"
	case CNCFormatCSV:
		return "text/csv"
	case CNCFormatJSON:
		return "application/json"
	case CNCFormatXML:
		return "application/xml"
	}
	return "application/octet-stream"
}

// Extension возвращает расширение файла (с точкой).
func (f CNCFormat) Extension() string {
	return "." + string(f)
}

// CNCJobStatus — статус CNC-задания.
type CNCJobStatus string

const (
	CNCJobStatusPending  CNCJobStatus = "pending"
	CNCJobStatusRunning  CNCJobStatus = "running"
	CNCJobStatusDone     CNCJobStatus = "done"
	CNCJobStatusFailed   CNCJobStatus = "failed"
)

// IsValid проверяет, что статус CNC-задания известен.
func (s CNCJobStatus) IsValid() bool {
	switch s {
	case CNCJobStatusPending, CNCJobStatusRunning, CNCJobStatusDone, CNCJobStatusFailed:
		return true
	}
	return false
}

// CNCJob — задание на CNC-экспорт (MFG-0011).
type CNCJob struct {
	ID           string
	ConfigID     string
	Format       CNCFormat
	Status       CNCJobStatus
	OutputPath   string
	ErrorMessage string
	CreatedAt    time.Time
	CompletedAt  *time.Time
}

// Validate проверяет инварианты CNC-задания.
func (j *CNCJob) Validate() error {
	if j == nil {
		return fmt.Errorf("manufacturing: CNC job is required")
	}
	if j.ID == "" {
		return fmt.Errorf("manufacturing: CNC job id is required")
	}
	if j.ConfigID == "" {
		return fmt.Errorf("manufacturing: CNC job config_id is required")
	}
	if !j.Format.IsValid() {
		return fmt.Errorf("manufacturing: CNC job has invalid format %q", j.Format)
	}
	if !j.Status.IsValid() {
		return fmt.Errorf("manufacturing: CNC job has invalid status %q", j.Status)
	}
	return nil
}

// AssemblyNode — узел дерева сборки (MFG-0010).
type AssemblyNodeType string

const (
	AssemblyNodeRoot    AssemblyNodeType = "root"
	AssemblyNodeGroup   AssemblyNodeType = "group"
	AssemblyNodePart    AssemblyNodeType = "part"
)

// IsValid проверяет корректность типа узла сборки.
func (t AssemblyNodeType) IsValid() bool {
	switch t {
	case AssemblyNodeRoot, AssemblyNodeGroup, AssemblyNodePart:
		return true
	}
	return false
}

// AssemblyNode — узел дерева сборки.
type AssemblyNode struct {
	ID           string
	Type         AssemblyNodeType
	Name         string
	PartNumber   PartNumber // заполнено только для типа "part"
	Quantity     int
	Children     []*AssemblyNode
	Position     [3]float64 // X, Y, Z позиция в сборке (мм)
	Rotation     [3]float64 // Rx, Ry, Rz поворот в сборке (радианы)
}

// Validate проверяет инварианты дерева сборки.
func (n *AssemblyNode) Validate() error {
	if n == nil {
		return fmt.Errorf("manufacturing: assembly node is required")
	}
	if n.ID == "" {
		return fmt.Errorf("manufacturing: assembly node id is required")
	}
	if !n.Type.IsValid() {
		return fmt.Errorf("manufacturing: assembly node has invalid type %q", n.Type)
	}
	if n.Name == "" {
		return fmt.Errorf("manufacturing: assembly node name is required")
	}
	if n.Quantity <= 0 {
		return fmt.Errorf("manufacturing: assembly node quantity must be positive, got %d", n.Quantity)
	}
	if n.Type == AssemblyNodePart && n.PartNumber == "" {
		return fmt.Errorf("manufacturing: assembly part node must have part_number")
	}
	for i, child := range n.Children {
		if err := child.Validate(); err != nil {
			return fmt.Errorf("manufacturing: assembly child %d: %v", i, err)
		}
	}
	return nil
}

// AssemblyTree — дерево сборки изделия (MFG-0010).
type AssemblyTree struct {
	Root *AssemblyNode
}

// Validate проверяет инварианты дерева сборки.
func (t *AssemblyTree) Validate() error {
	if t == nil {
		return fmt.Errorf("manufacturing: assembly tree is required")
	}
	if t.Root == nil {
		return fmt.Errorf("manufacturing: assembly tree has no root")
	}
	if t.Root.Type != AssemblyNodeRoot {
		return fmt.Errorf("manufacturing: assembly tree root must be of type 'root'")
	}
	return t.Root.Validate()
}

// Parts возвращает все листовые узлы (детали) дерева в порядке обхода.
func (t *AssemblyTree) Parts() []PartNumber {
	if t == nil || t.Root == nil {
		return nil
	}
	var result []PartNumber
	collectParts(t.Root, &result)
	return result
}

func collectParts(node *AssemblyNode, result *[]PartNumber) {
	if node.Type == AssemblyNodePart {
		*result = append(*result, node.PartNumber)
	}
	for _, child := range node.Children {
		collectParts(child, result)
	}
}
