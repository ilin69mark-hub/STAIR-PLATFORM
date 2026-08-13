package manufacturing

import (
	"testing"
)

func validDataset() *ManufacturingCostDataset {
	return &ManufacturingCostDataset{
		MaterialConsumption: []MaterialConsumption{{
			MaterialCode: "M", PartArea: 216000, WasteArea: 729400,
			SheetArea: 945400, Volume: 8640000, Mass: 67.824,
		}},
		PartCount:      1,
		OperationCount: 2,
		FastenerCount:  0,
		PartArea:       216000,
		WasteArea:      729400,
		SheetArea:      945400,
		WastePercent:   729400.0 / 945400,
		Utilization:    216000.0 / 945400,
		Volume:         8640000,
		SurfaceArea:    517600,
		Mass:           67.824,
		OperationPlan: &OperationPlan{Parts: []PartOperationPlan{{
			PartNumber: "P-1",
			Operations: []Operation{
				{ID: 1, PartNumber: "P-1", Type: OpCutting, Sequence: 1, Machine: MachineLaserCutter, EstimatedTime: 3, OperatorRequired: true},
				{ID: 2, PartNumber: "P-1", Type: OpFinishing, Sequence: 2, Machine: MachineManualWorkstation, EstimatedTime: 2, OperatorRequired: true},
			},
		}}},
		EstimatedMachineTime:    3,
		EstimatedLaborTime:      2,
		EstimatedProductionTime: 5,
	}
}

func TestManufacturingCostDatasetValidate(t *testing.T) {
	if err := validDataset().Validate(); err != nil {
		t.Fatalf("valid dataset must pass: %v", err)
	}
	if err := (*ManufacturingCostDataset)(nil).Validate(); err == nil {
		t.Fatal("nil dataset must be rejected")
	}

	d := validDataset()
	d.MaterialConsumption = nil
	if err := d.Validate(); err == nil {
		t.Fatal("empty consumption must be rejected")
	}

	d = validDataset()
	d.PartCount = 0
	if err := d.Validate(); err == nil {
		t.Fatal("zero part count must be rejected")
	}

	d = validDataset()
	d.OperationCount = 0
	if err := d.Validate(); err == nil {
		t.Fatal("zero operation count must be rejected")
	}

	d = validDataset()
	d.FastenerCount = -1
	if err := d.Validate(); err == nil {
		t.Fatal("negative fastener count must be rejected")
	}

	d = validDataset()
	d.SheetArea = 0
	if err := d.Validate(); err == nil {
		t.Fatal("zero sheet area must be rejected")
	}

	d = validDataset()
	d.PartArea = d.SheetArea + 1
	if err := d.Validate(); err == nil {
		t.Fatal("part area above sheet area must be rejected")
	}

	d = validDataset()
	d.WastePercent = 2
	if err := d.Validate(); err == nil {
		t.Fatal("waste percent above 1 must be rejected")
	}

	d = validDataset()
	d.MaterialConsumption[0].Mass = 0
	if err := d.Validate(); err == nil {
		t.Fatal("consumption totals mismatch must be rejected")
	}

	d = validDataset()
	d.MaterialConsumption[0].MaterialCode = ""
	if err := d.Validate(); err == nil {
		t.Fatal("consumption without material must be rejected")
	}

	d = validDataset()
	d.Volume = -1
	if err := d.Validate(); err == nil {
		t.Fatal("negative volume must be rejected")
	}

	d = validDataset()
	d.OperationPlan = nil
	if err := d.Validate(); err == nil {
		t.Fatal("dataset without operation plan must be rejected")
	}

	d = validDataset()
	d.OperationPlan.Parts[0].Operations = d.OperationPlan.Parts[0].Operations[:1]
	if err := d.Validate(); err == nil {
		t.Fatal("operation count mismatch must be rejected")
	}

	d = validDataset()
	d.EstimatedProductionTime = 4
	if err := d.Validate(); err == nil {
		t.Fatal("production time mismatch must be rejected")
	}

	d = validDataset()
	d.EstimatedMachineTime = -1
	if err := d.Validate(); err == nil {
		t.Fatal("negative machine time must be rejected")
	}
}
