package exchanger

import (
	"fmt"
	"sync"
	"time"

	"heatpulse/internal/station"
)

type WarmupProof struct {
	StationID   string    `json:"station_id"`
	ExchangerID string    `json:"exchanger_id"`
	Revision    uint64    `json:"revision"`
	Outlet      float64   `json:"outlet_celsius"`
	Ready       bool      `json:"ready"`
	At          time.Time `json:"at"`
}

type WarmupService struct {
	stations *station.Registry
	mu       sync.RWMutex
	proofs   map[string]WarmupProof
}

func NewWarmupService(stations *station.Registry) *WarmupService {
	return &WarmupService{stations: stations, proofs: make(map[string]WarmupProof)}
}

func (s *WarmupService) Proof(stationID, exchangerID string, revision uint64, outlet float64) (WarmupProof, error) {
	item, ok := s.stations.Get(stationID)
	if !ok {
		return WarmupProof{}, fmt.Errorf("station %s not found", stationID)
	}
	phase := item.Exchangers[exchangerID]
	proof := WarmupProof{
		StationID: stationID, ExchangerID: exchangerID, Revision: revision,
		Outlet: outlet, Ready: outlet >= 38 && (phase == station.ExchangerPriming || phase == station.ExchangerCarrying),
		At: time.Now().UTC(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.proofs[stationID]
	if !exists || revision >= current.Revision {
		s.proofs[stationID] = proof
	}
	return proof, nil
}

func (s *WarmupService) Ready(stationID string, revision uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	proof, ok := s.proofs[stationID]
	return ok && proof.Revision == revision && proof.Ready
}

func (s *WarmupService) Latest(stationID string) (WarmupProof, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	proof, ok := s.proofs[stationID]
	return proof, ok
}
