package hydraulic

import (
	"fmt"
	"math"
	"time"

	"heatpulse/internal/telemetry"
)

type Balance struct {
	StationID            string    `json:"station_id"`
	Watermark            uint64    `json:"watermark"`
	DifferentialPressure float64   `json:"differential_pressure"`
	ExpectedFlow         float64   `json:"expected_flow"`
	MeasuredFlow         float64   `json:"measured_flow"`
	ImbalanceRatio       float64   `json:"imbalance_ratio"`
	LeakLikely           bool      `json:"leak_likely"`
	EvaluatedAt          time.Time `json:"evaluated_at"`
}

type Balancer struct {
	leakThreshold float64
}

func NewBalancer(leakThreshold float64) *Balancer {
	if leakThreshold <= 0 {
		leakThreshold = 0.28
	}
	return &Balancer{leakThreshold: leakThreshold}
}

func (b *Balancer) Evaluate(snapshot telemetry.HydraulicSnapshot) (Balance, error) {
	if snapshot.Watermark == 0 {
		return Balance{}, fmt.Errorf("hydraulic snapshot has no committed watermark")
	}
	differential := snapshot.DifferentialPressure()
	if differential < 0 {
		return Balance{}, fmt.Errorf("negative differential pressure %.2f", differential)
	}
	opening := math.Max(snapshot.ValvePosition/100, 0.05)
	expected := math.Sqrt(differential) * opening * 18
	denominator := math.Max(expected, 1)
	ratio := math.Abs(expected-snapshot.Flow) / denominator
	return Balance{
		StationID: snapshot.StationID, Watermark: snapshot.Watermark,
		DifferentialPressure: differential, ExpectedFlow: expected,
		MeasuredFlow: snapshot.Flow, ImbalanceRatio: ratio,
		LeakLikely: ratio >= b.leakThreshold, EvaluatedAt: time.Now().UTC(),
	}, nil
}

func (b *Balancer) Threshold() float64 {
	return b.leakThreshold
}
