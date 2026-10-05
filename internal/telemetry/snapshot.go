package telemetry

import (
	"fmt"
	"time"
)

type HydraulicSnapshot struct {
	StationID     string    `json:"station_id"`
	Watermark     uint64    `json:"watermark"`
	Supply        float64   `json:"supply_pressure"`
	Return        float64   `json:"return_pressure"`
	Flow          float64   `json:"flow"`
	ValvePosition float64   `json:"valve_position"`
	CapturedAt    time.Time `json:"captured_at"`
}

func (s HydraulicSnapshot) DifferentialPressure() float64 {
	return s.Supply - s.Return
}

type SnapshotBuilder struct {
	series *Series
}

func NewSnapshotBuilder(series *Series) *SnapshotBuilder {
	return &SnapshotBuilder{series: series}
}

func (b *SnapshotBuilder) Build(stationID string) (HydraulicSnapshot, error) {
	kinds := []Kind{KindSupplyPressure, KindReturnPressure, KindFlow, KindValvePosition}
	watermarks := b.series.Watermarks(stationID, kinds)
	if len(watermarks) == 0 {
		return HydraulicSnapshot{}, fmt.Errorf("station %s has no aligned hydraulic telemetry", stationID)
	}
	watermark := watermarks[len(watermarks)-1]
	supply, ok := b.series.At(stationID, KindSupplyPressure, watermark)
	if !ok {
		return HydraulicSnapshot{}, fmt.Errorf("supply pressure missing at %d", watermark)
	}
	ret, ok := b.series.At(stationID, KindReturnPressure, watermark)
	if !ok {
		return HydraulicSnapshot{}, fmt.Errorf("return pressure missing at %d", watermark)
	}
	flow, ok := b.series.At(stationID, KindFlow, watermark)
	if !ok {
		return HydraulicSnapshot{}, fmt.Errorf("flow missing at %d", watermark)
	}
	position, ok := b.series.At(stationID, KindValvePosition, watermark)
	if !ok {
		return HydraulicSnapshot{}, fmt.Errorf("valve position missing at %d", watermark)
	}
	return HydraulicSnapshot{
		StationID: stationID, Watermark: watermark, Supply: supply.Value, Return: ret.Value,
		Flow: flow.Value, ValvePosition: position.Value, CapturedAt: time.Now().UTC(),
	}, nil
}

func (b *SnapshotBuilder) Ready(stationID string) bool {
	_, err := b.Build(stationID)
	return err == nil
}
