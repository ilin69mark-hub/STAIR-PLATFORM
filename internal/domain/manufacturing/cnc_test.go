package manufacturing

import (
	"testing"
)

func TestCNCFormatIsValid(t *testing.T) {
	valid := []CNCFormat{
		CNCFormatSTEP, CNCFormatIGES, CNCFormatOBJ, CNCFormat3MF,
		CNCFormatDXF, CNCFormatSTL, CNCFormatCSV, CNCFormatJSON, CNCFormatXML,
	}
	for _, f := range valid {
		if !f.IsValid() {
			t.Errorf("expected CNC format %q to be valid", f)
		}
	}
	if CNCFormat("invalid").IsValid() {
		t.Error("expected 'invalid' CNC format to be invalid")
	}
}

func TestCNCFormatMIME(t *testing.T) {
	tests := []struct {
		format CNCFormat
		mime   string
	}{
		{CNCFormatSTEP, "application/step"},
		{CNCFormatIGES, "application/iges"},
		{CNCFormatOBJ, "model/obj"},
		{CNCFormat3MF, "model/3mf"},
		{CNCFormatDXF, "application/dxf"},
		{CNCFormatSTL, "model/stl"},
		{CNCFormatCSV, "text/csv"},
		{CNCFormatJSON, "application/json"},
		{CNCFormatXML, "application/xml"},
	}
	for _, tt := range tests {
		if got := tt.format.MIME(); got != tt.mime {
			t.Errorf("format %q MIME: got %q, want %q", tt.format, got, tt.mime)
		}
	}
}

func TestCNCFormatExtension(t *testing.T) {
	tests := []struct {
		format CNCFormat
		ext    string
	}{
		{CNCFormatSTEP, ".step"},
		{CNCFormatIGES, ".iges"},
		{CNCFormatOBJ, ".obj"},
		{CNCFormat3MF, ".3mf"},
		{CNCFormatCSV, ".csv"},
		{CNCFormatJSON, ".json"},
		{CNCFormatXML, ".xml"},
	}
	for _, tt := range tests {
		if got := tt.format.Extension(); got != tt.ext {
			t.Errorf("format %q Extension: got %q, want %q", tt.format, got, tt.ext)
		}
	}
}

func TestCNCJobValidation(t *testing.T) {
	job := &CNCJob{
		ID:       "job-1",
		ConfigID: "config-1",
		Format:   CNCFormatSTEP,
		Status:   CNCJobStatusPending,
	}
	if err := job.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCNCJobValidationEmptyID(t *testing.T) {
	job := &CNCJob{
		ConfigID: "config-1",
		Format:   CNCFormatSTEP,
		Status:   CNCJobStatusPending,
	}
	if err := job.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestCNCJobValidationInvalidFormat(t *testing.T) {
	job := &CNCJob{
		ID:       "job-1",
		ConfigID: "config-1",
		Format:   "invalid",
		Status:   CNCJobStatusPending,
	}
	if err := job.Validate(); err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestAssemblyNodeValidation(t *testing.T) {
	node := &AssemblyNode{
		ID:       "node-1",
		Type:     AssemblyNodeRoot,
		Name:     "Root",
		Quantity: 1,
	}
	if err := node.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssemblyNodePartRequiresPartNumber(t *testing.T) {
	node := &AssemblyNode{
		ID:       "node-1",
		Type:     AssemblyNodePart,
		Name:     "Part",
		Quantity: 1,
	}
	if err := node.Validate(); err == nil {
		t.Fatal("expected error for part node without part_number")
	}
}

func TestAssemblyNodePartWithPartNumber(t *testing.T) {
	node := &AssemblyNode{
		ID:         "node-1",
		Type:       AssemblyNodePart,
		Name:       "Part",
		PartNumber: "P-001",
		Quantity:   1,
	}
	if err := node.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssemblyTreeValidation(t *testing.T) {
	tree := &AssemblyTree{
		Root: &AssemblyNode{
			ID:       "root",
			Type:     AssemblyNodeRoot,
			Name:     "Root",
			Quantity: 1,
			Children: []*AssemblyNode{
				{
					ID:         "part-1",
					Type:       AssemblyNodePart,
					Name:       "Step",
					PartNumber: "P-001",
					Quantity:   2,
				},
			},
		},
	}
	if err := tree.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssemblyTreeParts(t *testing.T) {
	tree := &AssemblyTree{
		Root: &AssemblyNode{
			ID:       "root",
			Type:     AssemblyNodeRoot,
			Name:     "Root",
			Quantity: 1,
			Children: []*AssemblyNode{
				{
					ID:         "part-1",
					Type:       AssemblyNodePart,
					Name:       "Step 1",
					PartNumber: "P-001",
					Quantity:   2,
				},
				{
					ID:   "group-1",
					Type: AssemblyNodeGroup,
					Name: "Group",
					Quantity: 1,
					Children: []*AssemblyNode{
						{
							ID:         "part-2",
							Type:       AssemblyNodePart,
							Name:       "Step 2",
							PartNumber: "P-002",
							Quantity:   1,
						},
					},
				},
			},
		},
	}
	parts := tree.Parts()
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0] != "P-001" || parts[1] != "P-002" {
		t.Fatalf("unexpected parts: %v", parts)
	}
}

func TestAssemblyNodeTypeIsValid(t *testing.T) {
	valid := []AssemblyNodeType{AssemblyNodeRoot, AssemblyNodeGroup, AssemblyNodePart}
	for _, nt := range valid {
		if !nt.IsValid() {
			t.Errorf("expected type %q to be valid", nt)
		}
	}
	if AssemblyNodeType("invalid").IsValid() {
		t.Error("expected 'invalid' type to be invalid")
	}
}

func TestAssemblyNodeInvalidType(t *testing.T) {
	node := &AssemblyNode{
		ID:       "node-1",
		Type:     "invalid",
		Name:     "Test",
		Quantity: 1,
	}
	if err := node.Validate(); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestAssemblyNodeZeroQuantity(t *testing.T) {
	node := &AssemblyNode{
		ID:       "node-1",
		Type:     AssemblyNodePart,
		Name:     "Part",
		Quantity: 0,
	}
	if err := node.Validate(); err == nil {
		t.Fatal("expected error for zero quantity")
	}
}

func TestAssemblyTreeEmptyRoot(t *testing.T) {
	tree := &AssemblyTree{}
	if err := tree.Validate(); err == nil {
		t.Fatal("expected error for nil root")
	}
}

func TestAssemblyTreeInvalidRootType(t *testing.T) {
	tree := &AssemblyTree{
		Root: &AssemblyNode{
			ID:       "root",
			Type:     AssemblyNodePart,
			Name:     "Root",
			Quantity: 1,
		},
	}
	if err := tree.Validate(); err == nil {
		t.Fatal("expected error for non-root type at root")
	}
}

func TestAssemblyTreePartsEmpty(t *testing.T) {
	tree := &AssemblyTree{
		Root: &AssemblyNode{
			ID:       "root",
			Type:     AssemblyNodeRoot,
			Name:     "Root",
			Quantity: 1,
		},
	}
	parts := tree.Parts()
	if len(parts) != 0 {
		t.Fatalf("expected 0 parts, got %d", len(parts))
	}
}
