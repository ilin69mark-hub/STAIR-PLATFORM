package manufacturing

import "testing"

func TestOperationTypeIsValid(t *testing.T) {
	for _, op := range []OperationType{
		OpCutting, OpDrilling, OpMilling, OpTurning, OpGrinding, OpBending,
		OpPunching, OpWelding, OpThreading, OpPainting, OpPowderCoating,
		OpGalvanizing, OpHeatTreatment, OpAssembly, OpFinishing, OpPackaging, OpCustom,
	} {
		if !op.IsValid() {
			t.Fatalf("operation %q must be valid", op)
		}
	}
	if OperationType("bogus").IsValid() {
		t.Fatal("unknown operation must be invalid")
	}
}

func TestMachineTypeIsValid(t *testing.T) {
	for _, m := range []MachineType{
		MachineLaserCutter, MachineWaterjet, MachinePlasmaCutter, MachineBandSaw,
		MachineCNCMill, MachineCNCLathe, MachinePressBrake, MachineWeldingStation,
		MachinePaintingBooth, MachineAssemblyStation, MachineManualWorkstation,
	} {
		if !m.IsValid() {
			t.Fatalf("machine %q must be valid", m)
		}
	}
	if MachineType("bogus").IsValid() {
		t.Fatal("unknown machine must be invalid")
	}
}

func validPlan() *OperationPlan {
	return &OperationPlan{Parts: []PartOperationPlan{{
		PartNumber: "P-1",
		Operations: []Operation{
			{ID: 1, PartNumber: "P-1", Type: OpCutting, Sequence: 1, Machine: MachineLaserCutter, EstimatedTime: 8.8, OperatorRequired: true},
			{ID: 2, PartNumber: "P-1", Type: OpFinishing, Sequence: 2, Machine: MachineManualWorkstation, EstimatedTime: 3, OperatorRequired: true},
		},
	}}}
}

func TestOperationPlanValidate(t *testing.T) {
	if err := validPlan().Validate(); err != nil {
		t.Fatalf("valid plan must pass: %v", err)
	}
	if err := (*OperationPlan)(nil).Validate(); err == nil {
		t.Fatal("nil plan must be rejected")
	}

	p := validPlan()
	p.Parts = nil
	if err := p.Validate(); err == nil {
		t.Fatal("plan without parts must be rejected")
	}

	p = validPlan()
	p.Parts[0].Operations = nil
	if err := p.Validate(); err == nil {
		t.Fatal("part without operations must be rejected")
	}

	p = validPlan()
	p.Parts[0].PartNumber = ""
	if err := p.Validate(); err == nil {
		t.Fatal("part without number must be rejected")
	}

	p = validPlan()
	p.Parts[0].Operations[1].Sequence = 1
	if err := p.Validate(); err == nil {
		t.Fatal("non-sequential operations must be rejected")
	}

	p = validPlan()
	p.Parts[0].Operations[1].ID = 1
	if err := p.Validate(); err == nil {
		t.Fatal("duplicate operation ID must be rejected")
	}

	p = validPlan()
	p.Parts[0].Operations[0].PartNumber = "OTHER"
	if err := p.Validate(); err == nil {
		t.Fatal("operation referencing another part must be rejected")
	}

	p = validPlan()
	p.Parts[0].Operations[0].Type = OperationType("bogus")
	if err := p.Validate(); err == nil {
		t.Fatal("invalid operation type must be rejected")
	}

	p = validPlan()
	p.Parts[0].Operations[0].Machine = MachineType("bogus")
	if err := p.Validate(); err == nil {
		t.Fatal("invalid machine type must be rejected")
	}

	p = validPlan()
	p.Parts[0].Operations[0].EstimatedTime = -1
	if err := p.Validate(); err == nil {
		t.Fatal("negative estimated time must be rejected")
	}
}

func TestOperationPlanTotalTime(t *testing.T) {
	if got := validPlan().TotalTime(); got != 11.8 {
		t.Fatalf("total time = %v, want 11.8", got)
	}
}
