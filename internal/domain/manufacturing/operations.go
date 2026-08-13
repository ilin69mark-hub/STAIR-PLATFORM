package manufacturing

import (
	"fmt"
	"math"
)

// OperationType — категория технологической операции (MFG-0009).
// Список расширяемый: добавление новых категорий не ломает существующие.
type OperationType string

const (
	OpCutting       OperationType = "cutting"
	OpDrilling      OperationType = "drilling"
	OpMilling       OperationType = "milling"
	OpTurning       OperationType = "turning"
	OpGrinding      OperationType = "grinding"
	OpBending       OperationType = "bending"
	OpPunching      OperationType = "punching"
	OpWelding       OperationType = "welding"
	OpThreading     OperationType = "threading"
	OpPainting      OperationType = "painting"
	OpPowderCoating OperationType = "powder-coating"
	OpGalvanizing   OperationType = "galvanizing"
	OpHeatTreatment OperationType = "heat-treatment"
	OpAssembly      OperationType = "assembly"
	OpFinishing     OperationType = "finishing"
	OpPackaging     OperationType = "packaging"
	OpCustom        OperationType = "custom"
)

// IsValid проверяет корректность категории операции.
func (t OperationType) IsValid() bool {
	switch t {
	case OpCutting, OpDrilling, OpMilling, OpTurning, OpGrinding, OpBending,
		OpPunching, OpWelding, OpThreading, OpPainting, OpPowderCoating,
		OpGalvanizing, OpHeatTreatment, OpAssembly, OpFinishing, OpPackaging, OpCustom:
		return true
	}
	return false
}

// MachineType — тип производственного оборудования (MFG-0009).
type MachineType string

const (
	MachineLaserCutter       MachineType = "laser-cutter"
	MachineWaterjet          MachineType = "waterjet"
	MachinePlasmaCutter      MachineType = "plasma-cutter"
	MachineBandSaw           MachineType = "band-saw"
	MachineCNCMill           MachineType = "cnc-mill"
	MachineCNCLathe          MachineType = "cnc-lathe"
	MachinePressBrake        MachineType = "press-brake"
	MachineWeldingStation    MachineType = "welding-station"
	MachinePaintingBooth     MachineType = "painting-booth"
	MachineAssemblyStation   MachineType = "assembly-station"
	MachineManualWorkstation MachineType = "manual-workstation"
)

// IsValid проверяет корректность типа оборудования.
func (m MachineType) IsValid() bool {
	switch m {
	case MachineLaserCutter, MachineWaterjet, MachinePlasmaCutter, MachineBandSaw,
		MachineCNCMill, MachineCNCLathe, MachinePressBrake, MachineWeldingStation,
		MachinePaintingBooth, MachineAssemblyStation, MachineManualWorkstation:
		return true
	}
	return false
}

// Operation — одна операция технологического маршрута (MFG-0009).
// Каждая операция принадлежит одной детали (PartNumber), имеет
// последовательность выполнения (Sequence) и оценку времени (минуты).
type Operation struct {
	ID               int
	PartNumber       PartNumber
	Type             OperationType
	Sequence         int
	Machine          MachineType
	EstimatedTime    float64 // минуты
	OperatorRequired bool
}

// PartOperationPlan — технологический маршрут одной детали (MFG-0009):
// упорядоченная последовательность операций.
type PartOperationPlan struct {
	PartNumber PartNumber
	Operations []Operation
}

// OperationPlan — агрегат технологических маршрутов (MFG-0009, Operation
// Plan): маршруты всех деталей изделия. Используется для подготовки CNC,
// оценки трудоёмкости и передачи в Pricing (Machine Cost, PRC-0008).
type OperationPlan struct {
	Parts []PartOperationPlan
}

// Validate проверяет инварианты плана: непустые маршруты, уникальные
// глобальные ID операций, строго возрастающие последовательности с 1,
// неотрицательные времена, валидные типы и оборудование.
func (p *OperationPlan) Validate() error {
	if p == nil {
		return fmt.Errorf("manufacturing: operation plan is required")
	}
	if len(p.Parts) == 0 {
		return fmt.Errorf("manufacturing: operation plan has no parts")
	}
	seenOpIDs := make(map[int]bool)
	for i, part := range p.Parts {
		if part.PartNumber == "" {
			return fmt.Errorf("manufacturing: operation plan part %d has no number", i)
		}
		if len(part.Operations) == 0 {
			return fmt.Errorf("manufacturing: operation plan part %q has no operations", part.PartNumber)
		}
		for j, op := range part.Operations {
			if op.ID <= 0 {
				return fmt.Errorf("manufacturing: operation %d of part %q has non-positive ID", j, part.PartNumber)
			}
			if seenOpIDs[op.ID] {
				return fmt.Errorf("manufacturing: duplicate operation ID %d", op.ID)
			}
			seenOpIDs[op.ID] = true
			if op.PartNumber != part.PartNumber {
				return fmt.Errorf("manufacturing: operation %d of part %q references part %q", op.ID, part.PartNumber, op.PartNumber)
			}
			if op.Sequence != j+1 {
				return fmt.Errorf("manufacturing: operation %d of part %q has sequence %d, want %d", op.ID, part.PartNumber, op.Sequence, j+1)
			}
			if !op.Type.IsValid() {
				return fmt.Errorf("manufacturing: operation %d of part %q has invalid type %q", op.ID, part.PartNumber, op.Type)
			}
			if !op.Machine.IsValid() {
				return fmt.Errorf("manufacturing: operation %d of part %q has invalid machine %q", op.ID, part.PartNumber, op.Machine)
			}
			if math.IsNaN(op.EstimatedTime) || math.IsInf(op.EstimatedTime, 0) || op.EstimatedTime < 0 {
				return fmt.Errorf("manufacturing: operation %d of part %q has invalid estimated time", op.ID, part.PartNumber)
			}
		}
	}
	return nil
}

// TotalTime возвращает суммарное оценённое время всех операций плана, мин.
func (p *OperationPlan) TotalTime() float64 {
	if p == nil {
		return 0
	}
	var total float64
	for _, part := range p.Parts {
		for _, op := range part.Operations {
			total += op.EstimatedTime
		}
	}
	return total
}
