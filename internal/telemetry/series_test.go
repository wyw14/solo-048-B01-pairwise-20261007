package telemetry

import (
	"errors"
	"testing"
	"time"
)

func TestSeriesAddRejectsConflictingDuplicate(t *testing.T) {
	series := NewSeries(16)
	first := Sample{StationID: "st-1", Kind: KindFlow, Watermark: 7, Value: 8.1}
	if err := series.Add(first); err != nil {
		t.Fatalf("first add: %v", err)
	}

	err := series.Add(Sample{StationID: "st-1", Kind: KindFlow, Watermark: 7, Value: 9.9})
	var conflict *SampleConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected SampleConflictError, got %v", err)
	}
	if conflict.Existing != 8.1 || conflict.Incoming != 9.9 {
		t.Fatalf("conflict carries wrong values: %+v", conflict)
	}

	latest, ok := series.Latest("st-1", KindFlow)
	if !ok || latest.Value != 8.1 {
		t.Fatalf("latest changed after conflict: %+v", latest)
	}
	at, ok := series.At("st-1", KindFlow, 7)
	if !ok || at.Value != 8.1 {
		t.Fatalf("value at watermark 7 changed after conflict: %+v", at)
	}
}

func TestSeriesAddIdempotentDuplicate(t *testing.T) {
	series := NewSeries(16)
	at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	first := Sample{StationID: "st-1", Kind: KindSupplyPressure, Watermark: 3, Value: 0.78, At: at}
	if err := series.Add(first); err != nil {
		t.Fatalf("first add: %v", err)
	}
	retry := first
	retry.At = at.Add(time.Minute) // retries may carry a fresh receipt time
	if err := series.Add(retry); err != nil {
		t.Fatalf("identical retry must succeed: %v", err)
	}

	stored, ok := series.At("st-1", KindSupplyPressure, 3)
	if !ok {
		t.Fatal("sample missing at watermark 3")
	}
	if !stored.At.Equal(at) {
		t.Fatalf("stored sample replaced by retry: got At=%v, want %v", stored.At, at)
	}
	if got := len(series.Watermarks("st-1", []Kind{KindSupplyPressure})); got != 1 {
		t.Fatalf("duplicate watermark stored, %d entries", got)
	}
}

func TestSeriesAddKeepsWatermarkOrder(t *testing.T) {
	series := NewSeries(16)
	for _, watermark := range []uint64{5, 2, 9, 1} {
		if err := series.Add(Sample{StationID: "st-1", Kind: KindFlow, Watermark: watermark, Value: float64(watermark)}); err != nil {
			t.Fatalf("add watermark %d: %v", watermark, err)
		}
	}
	latest, ok := series.Latest("st-1", KindFlow)
	if !ok || latest.Watermark != 9 {
		t.Fatalf("latest = %+v, want watermark 9", latest)
	}
	for _, watermark := range []uint64{1, 2, 5, 9} {
		if _, ok := series.At("st-1", KindFlow, watermark); !ok {
			t.Fatalf("watermark %d not found", watermark)
		}
	}
}

func TestSnapshotUnaffectedByDuplicateTelemetry(t *testing.T) {
	series := NewSeries(16)
	builder := NewSnapshotBuilder(series)
	kinds := []Kind{KindSupplyPressure, KindReturnPressure, KindFlow, KindValvePosition}
	values := []float64{0.78, 0.46, 8.1, 72}
	for i, kind := range kinds {
		if err := series.Add(Sample{StationID: "st-1", Kind: kind, Watermark: 4, Value: values[i]}); err != nil {
			t.Fatalf("seed %s: %v", kind, err)
		}
	}

	confirmed, err := builder.Build("st-1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Conflicting rewrites of the same watermark must be rejected and must
	// not leak into the already confirmed snapshot.
	for i, kind := range kinds {
		dup := Sample{StationID: "st-1", Kind: kind, Watermark: 4, Value: values[i] + 100}
		if err := series.Add(dup); err == nil {
			t.Fatalf("conflicting %s rewrite accepted", kind)
		}
	}
	rebuilt, err := builder.Build("st-1")
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if rebuilt.Watermark != confirmed.Watermark ||
		rebuilt.Supply != confirmed.Supply || rebuilt.Return != confirmed.Return ||
		rebuilt.Flow != confirmed.Flow || rebuilt.ValvePosition != confirmed.ValvePosition {
		t.Fatalf("confirmed snapshot mutated by duplicates: %+v -> %+v", confirmed, rebuilt)
	}
}
