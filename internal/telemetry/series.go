package telemetry

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

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

// SampleConflictError reports a sample whose station, kind and watermark are
// already recorded with a different value. The stored sample is left
// untouched: first write wins, so a confirmed snapshot can never be mutated
// by duplicate telemetry.
type SampleConflictError struct {
	StationID string
	Kind      Kind
	Watermark uint64
	Existing  float64
	Incoming  float64
}

func (e *SampleConflictError) Error() string {
	return fmt.Sprintf("station %s %s watermark %d already recorded as %g, rejecting conflicting value %g",
		e.StationID, e.Kind, e.Watermark, e.Existing, e.Incoming)
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
	if s.samples[sample.StationID] == nil {
		s.samples[sample.StationID] = make(map[Kind][]Sample)
	}
	items := s.samples[sample.StationID][sample.Kind]
	index := sort.Search(len(items), func(i int) bool { return items[i].Watermark >= sample.Watermark })
	if index < len(items) && items[index].Watermark == sample.Watermark {
		if items[index].Value == sample.Value {
			return nil
		}
		return &SampleConflictError{
			StationID: sample.StationID, Kind: sample.Kind, Watermark: sample.Watermark,
			Existing: items[index].Value, Incoming: sample.Value,
		}
	}
	items = append(items, Sample{})
	copy(items[index+1:], items[index:])
	items[index] = sample
	if len(items) > s.limit {
		items = append([]Sample(nil), items[len(items)-s.limit:]...)
	}
	s.samples[sample.StationID][sample.Kind] = items
	return nil
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
