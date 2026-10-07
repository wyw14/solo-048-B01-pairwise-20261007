package telemetry

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ErrWatermarkConflict is returned when a sample repeats a watermark already
// recorded for the same station and kind but carries a different value. The
// first sample at a watermark wins and remains immutable.
var ErrWatermarkConflict = errors.New("watermark conflict: sample for station and kind already recorded with a different value")

type Kind string

const (
	KindSupplyPressure Kind = "supply_pressure"
	KindReturnPressure Kind = "return_pressure"
	KindFlow           Kind = "flow"
	KindValvePosition  Kind = "valve_position"
	KindTemperature    Kind = "temperature"
)

type Sample struct {
	StationID string    `json:"station_id"`
	Kind      Kind      `json:"kind"`
	Watermark uint64    `json:"watermark"`
	Value     float64   `json:"value"`
	At        time.Time `json:"at"`
}

type Series struct {
	mu      sync.RWMutex
	samples map[string]map[Kind][]Sample
	limit   int
}

func NewSeries(limit int) *Series {
	if limit < 4 {
		limit = 64
	}
	return &Series{samples: make(map[string]map[Kind][]Sample), limit: limit}
}

func (s *Series) Add(sample Sample) error {
	if sample.StationID == "" || sample.Kind == "" || sample.Watermark == 0 {
		return fmt.Errorf("station, kind and watermark are required")
	}
	if sample.At.IsZero() {
		sample.At = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	byKind := s.samples[sample.StationID]
	if byKind == nil {
		byKind = make(map[Kind][]Sample)
		s.samples[sample.StationID] = byKind
	}
	items := byKind[sample.Kind]
	index := sort.Search(len(items), func(i int) bool { return items[i].Watermark >= sample.Watermark })
	if index < len(items) && items[index].Watermark == sample.Watermark {
		// A watermark identifies an immutable reading for a station/kind.
		// An identical retransmission is idempotent; a differing value is a
		// conflict and must not overwrite the confirmed reading.
		if items[index].Value != sample.Value {
			return fmt.Errorf("%w: station=%s kind=%s watermark=%d recorded=%g received=%g",
				ErrWatermarkConflict, sample.StationID, sample.Kind, sample.Watermark,
				items[index].Value, sample.Value)
		}
		return nil
	}
	items = insertSample(items, index, sample)
	if len(items) > s.limit {
		items = items[len(items)-s.limit:]
	}
	byKind[sample.Kind] = items
	return nil
}

// insertSample places sample at index without aliasing the backing array of
// items, so in-flight readers of the previous slice keep an intact snapshot.
func insertSample(items []Sample, index int, sample Sample) []Sample {
	updated := make([]Sample, 0, len(items)+1)
	updated = append(updated, items[:index]...)
	updated = append(updated, sample)
	updated = append(updated, items[index:]...)
	return updated
}

func (s *Series) Latest(stationID string, kind Kind) (Sample, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.samples[stationID][kind]
	if len(items) == 0 {
		return Sample{}, false
	}
	return items[len(items)-1], true
}

func (s *Series) At(stationID string, kind Kind, watermark uint64) (Sample, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := s.samples[stationID][kind]
	index := sort.Search(len(items), func(i int) bool { return items[i].Watermark >= watermark })
	if index >= len(items) || items[index].Watermark != watermark {
		return Sample{}, false
	}
	return items[index], true
}

func (s *Series) Watermarks(stationID string, kinds []Kind) []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := make(map[uint64]int)
	for _, kind := range kinds {
		seen := make(map[uint64]struct{})
		for _, sample := range s.samples[stationID][kind] {
			seen[sample.Watermark] = struct{}{}
		}
		for watermark := range seen {
			counts[watermark]++
		}
	}
	common := make([]uint64, 0)
	for watermark, count := range counts {
		if count == len(kinds) {
			common = append(common, watermark)
		}
	}
	sort.Slice(common, func(i, j int) bool { return common[i] < common[j] })
	return common
}
