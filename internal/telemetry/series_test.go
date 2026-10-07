package telemetry

import (
	"errors"
	"testing"
)

func sampleAt(station string, kind Kind, watermark uint64, value float64) Sample {
	return Sample{StationID: station, Kind: kind, Watermark: watermark, Value: value}
}

func TestAddKeepsWatermarksSortedAndUnique(t *testing.T) {
	series := NewSeries(0)
	// Out-of-order arrival must still be stored in watermark order.
	for _, smp := range []Sample{
		sampleAt("st-1", KindFlow, 3, 30),
		sampleAt("st-1", KindFlow, 1, 10),
		sampleAt("st-1", KindFlow, 2, 20),
	} {
		if err := series.Add(smp); err != nil {
			t.Fatalf("Add(%d): unexpected error: %v", smp.Watermark, err)
		}
	}

	latest, ok := series.Latest("st-1", KindFlow)
	if !ok || latest.Watermark != 3 || latest.Value != 30 {
		t.Fatalf("Latest = %+v ok=%v, want watermark 3 value 30", latest, ok)
	}

	// Identical retransmission is idempotent and introduces no duplicate.
	if err := series.Add(sampleAt("st-1", KindFlow, 2, 20)); err != nil {
		t.Fatalf("idempotent retransmission returned error: %v", err)
	}
	if got := series.Watermarks("st-1", []Kind{KindFlow}); len(got) != 3 {
		t.Fatalf("Watermarks after retransmission = %v, want exactly [1 2 3]", got)
	}
}

func TestAddRejectsConflictingWatermark(t *testing.T) {
	series := NewSeries(0)
	if err := series.Add(sampleAt("st-1", KindSupplyPressure, 5, 0.78)); err != nil {
		t.Fatalf("initial Add: %v", err)
	}
	err := series.Add(sampleAt("st-1", KindSupplyPressure, 5, 0.91))
	if !errors.Is(err, ErrWatermarkConflict) {
		t.Fatalf("conflicting Add error = %v, want ErrWatermarkConflict", err)
	}

	// The first value at the watermark must survive, and no duplicate exists.
	got, ok := series.At("st-1", KindSupplyPressure, 5)
	if !ok || got.Value != 0.78 {
		t.Fatalf("At(5) = %+v ok=%v, want original value 0.78", got, ok)
	}
	latest, _ := series.Latest("st-1", KindSupplyPressure)
	if latest.Value != 0.78 {
		t.Fatalf("Latest value = %v, want original 0.78", latest.Value)
	}
	if watermarks := series.Watermarks("st-1", []Kind{KindSupplyPressure}); len(watermarks) != 1 {
		t.Fatalf("Watermarks = %v, want a single entry [5]", watermarks)
	}

	// A conflicting retransmission must not poison earlier or later readings.
	if err := series.Add(sampleAt("st-1", KindSupplyPressure, 4, 0.70)); err != nil {
		t.Fatalf("Add(4): %v", err)
	}
	if err := series.Add(sampleAt("st-1", KindSupplyPressure, 6, 0.80)); err != nil {
		t.Fatalf("Add(6): %v", err)
	}
	for _, want := range []struct {
		watermark uint64
		value     float64
	}{{4, 0.70}, {5, 0.78}, {6, 0.80}} {
		got, ok := series.At("st-1", KindSupplyPressure, want.watermark)
		if !ok || got.Value != want.value {
			t.Fatalf("At(%d) = %+v ok=%v, want value %v", want.watermark, got, ok, want.value)
		}
	}
}

func TestAddTrimsToLimitKeepingNewest(t *testing.T) {
	series := NewSeries(4)
	for watermark := uint64(1); watermark <= 6; watermark++ {
		if err := series.Add(sampleAt("st-1", KindFlow, watermark, float64(watermark))); err != nil {
			t.Fatalf("Add(%d): %v", watermark, err)
		}
	}
	if got := series.Watermarks("st-1", []Kind{KindFlow}); len(got) != 4 || got[0] != 3 {
		t.Fatalf("Watermarks after trim = %v, want [3 4 5 6]", got)
	}
	latest, _ := series.Latest("st-1", KindFlow)
	if latest.Watermark != 6 {
		t.Fatalf("Latest watermark = %d, want 6", latest.Watermark)
	}
}

func TestConfirmedSnapshotImmutableAcrossRetransmissions(t *testing.T) {
	series := NewSeries(0)
	builder := NewSnapshotBuilder(series)
	station := "st-1"

	initial := map[Kind]float64{
		KindSupplyPressure: 0.78,
		KindReturnPressure: 0.46,
		KindFlow:           8.1,
		KindValvePosition:  72,
	}
	for kind, value := range initial {
		if err := series.Add(sampleAt(station, kind, 1, value)); err != nil {
			t.Fatalf("seed Add(%s): %v", kind, err)
		}
	}

	confirmed, err := builder.Build(station)
	if err != nil {
		t.Fatalf("initial Build: %v", err)
	}
	if confirmed.Supply != 0.78 || confirmed.Return != 0.46 ||
		confirmed.Flow != 8.1 || confirmed.ValvePosition != 72 {
		t.Fatalf("initial snapshot has mixed values: %+v", confirmed)
	}

	// Retransmit every kind at watermark 1 with tampered values, in the order
	// that previously left different kinds pointing at different versions.
	tampered := []struct {
		kind  Kind
		value float64
	}{
		{KindSupplyPressure, 0.91},
		{KindReturnPressure, 0.22},
		{KindFlow, 12.7},
		{KindValvePosition, 35},
	}
	for _, item := range tampered {
		if err := series.Add(sampleAt(station, item.kind, 1, item.value)); !errors.Is(err, ErrWatermarkConflict) {
			t.Fatalf("tampered Add(%s) error = %v, want ErrWatermarkConflict", item.kind, err)
		}
	}

	again, err := builder.Build(station)
	if err != nil {
		t.Fatalf("Build after retransmissions: %v", err)
	}
	if again.StationID != confirmed.StationID ||
		again.Watermark != confirmed.Watermark ||
		again.Supply != confirmed.Supply ||
		again.Return != confirmed.Return ||
		again.Flow != confirmed.Flow ||
		again.ValvePosition != confirmed.ValvePosition {
		t.Fatalf("snapshot changed after conflicting telemetry:\nbefore=%+v\nafter =%+v", confirmed, again)
	}

	// A newer aligned watermark advances the snapshot independently.
	for kind, value := range map[Kind]float64{
		KindSupplyPressure: 0.80,
		KindReturnPressure: 0.48,
		KindFlow:           8.4,
		KindValvePosition:  70,
	} {
		if err := series.Add(sampleAt(station, kind, 2, value)); err != nil {
			t.Fatalf("Add watermark 2 (%s): %v", kind, err)
		}
	}
	advanced, err := builder.Build(station)
	if err != nil {
		t.Fatalf("Build after new watermark: %v", err)
	}
	if advanced.Watermark != 2 || advanced.Supply != 0.80 {
		t.Fatalf("advanced snapshot = %+v, want watermark 2 with new readings", advanced)
	}
}

func TestWatermarksOnlyAlignsKindsPresentForAll(t *testing.T) {
	series := NewSeries(0)
	kinds := []Kind{KindSupplyPressure, KindReturnPressure, KindFlow, KindValvePosition}
	if err := series.Add(sampleAt("st-1", KindSupplyPressure, 1, 1)); err != nil {
		t.Fatal(err)
	}
	if got := series.Watermarks("st-1", kinds); len(got) != 0 {
		t.Fatalf("Watermarks with one kind = %v, want none aligned", got)
	}
	for _, kind := range kinds[1:] {
		if err := series.Add(sampleAt("st-1", kind, 1, 1)); err != nil {
			t.Fatal(err)
		}
	}
	if got := series.Watermarks("st-1", kinds); len(got) != 1 || got[0] != 1 {
		t.Fatalf("Watermarks with all kinds = %v, want [1]", got)
	}
}
