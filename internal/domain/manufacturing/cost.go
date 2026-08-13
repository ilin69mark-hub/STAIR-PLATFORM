package manufacturing

import (
	"fmt"
	"math"

	"stairplatform/internal/domain/engineering"
)

// MaterialConsumption — расход одного вида материала (MFG-0015):
// площади деталей и листов, отходы, объём и масса. Масса считается по
// плотности материала (физическая величина, не стоимость).
type MaterialConsumption struct {
	MaterialCode MaterialCode
	PartArea     float64 // мм² — площадь деталей
	WasteArea    float64 // мм² — отходы на листах этого материала
	SheetArea    float64 // мм² — площадь использованных листов
	Volume       float64 // мм³
	Mass         float64 // кг
}

// ManufacturingCostDataset — производственный набор данных (MFG-0015,
// Pricing Input Package): агрегированные физико-технические показатели
// изделия БЕЗ финансовых значений (цены, тарифы, прибыль, налоги —
// правила MFG-0015). Временные оценки и технологический маршрут поставляются
// Machine Operations (MFG-0009) и готовы к использованию Pricing.
type ManufacturingCostDataset struct {
	MaterialConsumption []MaterialConsumption

	PartCount      Quantity
	OperationCount int
	FastenerCount  Quantity

	PartArea     float64 // мм²
	WasteArea    float64 // мм²
	SheetArea    float64 // мм²
	WastePercent float64 // 0..1 — WasteArea / SheetArea
	Utilization  float64 // 0..1 — PartArea / SheetArea

	Volume      float64 // мм³
	SurfaceArea float64 // мм²
	Mass        float64 // кг

	OperationPlan           *OperationPlan
	EstimatedMachineTime    float64 // мин — машинное время (Cutting и пр.)
	EstimatedLaborTime      float64 // мин — ручное время (Finishing/QC)
	EstimatedProductionTime float64 // мин — Machine + Labor
}

// Validate проверяет инварианты производственного набора: непустое
// потребление, суммы сходятся к итогам, показатели в допустимых
// диапазонах. Финансовых проверок нет — набор не содержит денежных значений.
func (d *ManufacturingCostDataset) Validate() error {
	if d == nil {
		return fmt.Errorf("manufacturing: cost dataset is required")
	}
	if len(d.MaterialConsumption) == 0 {
		return fmt.Errorf("manufacturing: cost dataset has no material consumption")
	}
	if d.PartCount <= 0 {
		return fmt.Errorf("manufacturing: cost dataset has no parts")
	}
	if d.OperationCount <= 0 {
		return fmt.Errorf("manufacturing: cost dataset has non-positive operation count")
	}
	if d.FastenerCount < 0 {
		return fmt.Errorf("manufacturing: cost dataset has negative fastener count")
	}
	if d.OperationPlan == nil {
		return fmt.Errorf("manufacturing: cost dataset has no operation plan")
	}
	if err := d.OperationPlan.Validate(); err != nil {
		return err
	}
	if len(d.OperationPlan.Parts) != int(d.PartCount) {
		return fmt.Errorf("manufacturing: operation plan parts (%d) must match part count (%d)", len(d.OperationPlan.Parts), int(d.PartCount))
	}
	var planOps int
	for _, part := range d.OperationPlan.Parts {
		planOps += len(part.Operations)
	}
	if planOps != d.OperationCount {
		return fmt.Errorf("manufacturing: operation plan operations (%d) must match operation count (%d)", planOps, d.OperationCount)
	}
	for _, m := range []float64{
		d.PartArea, d.WasteArea, d.SheetArea, d.Volume, d.SurfaceArea, d.Mass,
		d.EstimatedMachineTime, d.EstimatedLaborTime, d.EstimatedProductionTime,
	} {
		if math.IsNaN(m) || math.IsInf(m, 0) || m < 0 {
			return fmt.Errorf("manufacturing: cost dataset metrics are invalid")
		}
	}
	if math.Abs(d.EstimatedProductionTime-(d.EstimatedMachineTime+d.EstimatedLaborTime)) > precision(d.EstimatedProductionTime) {
		return fmt.Errorf("manufacturing: cost dataset production time must equal machine plus labor time")
	}
	if d.SheetArea <= 0 {
		return fmt.Errorf("manufacturing: cost dataset sheet area must be positive")
	}
	if d.PartArea > d.SheetArea+engineering.Precision {
		return fmt.Errorf("manufacturing: cost dataset part area exceeds sheet area")
	}
	for _, v := range []float64{d.WastePercent, d.Utilization} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1+engineering.Precision {
			return fmt.Errorf("manufacturing: cost dataset ratios are out of range")
		}
	}

	var sumPart, sumWaste, sumSheet, sumVol, sumMass float64
	for i, c := range d.MaterialConsumption {
		if c.MaterialCode == "" {
			return fmt.Errorf("manufacturing: consumption %d has no material", i)
		}
		for _, v := range []float64{c.PartArea, c.WasteArea, c.SheetArea, c.Volume, c.Mass} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return fmt.Errorf("manufacturing: consumption %q has invalid metrics", c.MaterialCode)
			}
		}
		if c.SheetArea < c.PartArea-engineering.Precision {
			return fmt.Errorf("manufacturing: consumption %q part area exceeds sheet area", c.MaterialCode)
		}
		sumPart += c.PartArea
		sumWaste += c.WasteArea
		sumSheet += c.SheetArea
		sumVol += c.Volume
		sumMass += c.Mass
	}
	if math.Abs(sumPart-d.PartArea) > precision(sumPart) ||
		math.Abs(sumWaste-d.WasteArea) > precision(sumWaste) ||
		math.Abs(sumSheet-d.SheetArea) > precision(sumSheet) ||
		math.Abs(sumVol-d.Volume) > precision(sumVol) ||
		math.Abs(sumMass-d.Mass) > precision(sumMass) {
		return fmt.Errorf("manufacturing: cost dataset totals do not match consumption")
	}
	if math.Abs(d.WasteArea-(d.SheetArea-d.PartArea)) > precision(d.SheetArea) {
		return fmt.Errorf("manufacturing: cost dataset waste area is inconsistent")
	}
	return nil
}

// precision — допуск для больших площадей/объёмов: относительная точность.
func precision(v float64) float64 {
	if v == 0 {
		return engineering.Precision
	}
	return v * 1e-9
}
