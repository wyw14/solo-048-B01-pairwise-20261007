package pump

import (
	"fmt"
	"sync"
	"time"
)

type PressureProof struct {
	StationID    string    `json:"station_id"`
	Revision     uint64    `json:"revision"`
	Stage        int       `json:"stage"`
	Differential float64   `json:"differential"`
	Ready        bool      `json:"ready"`
	At           time.Time `json:"at"`
}

type PressureMonitor struct {
	mu     sync.RWMutex
	proofs map[string]PressureProof
}

func NewPressureMonitor() *PressureMonitor {
	return &PressureMonitor{proofs: make(map[string]PressureProof)}
}

func (m *PressureMonitor) Record(stationID string, revision uint64, stage int, differential float64) (PressureProof, error) {
	if stationID == "" || revision == 0 {
		return PressureProof{}, fmt.Errorf("station and recovery revision are required")
	}
	proof := PressureProof{
		StationID: stationID, Revision: revision, Stage: stage, Differential: differential,
		Ready: stage > 0 && differential >= 0.12, At: time.Now().UTC(),
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.proofs[stationID]
	if !ok || revision >= current.Revision {
		m.proofs[stationID] = proof
	}
	return proof, nil
}

func (m *PressureMonitor) Ready(stationID string, revision uint64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	proof, ok := m.proofs[stationID]
	return ok && proof.Revision == revision && proof.Ready
}

func (m *PressureMonitor) Proof(stationID string) (PressureProof, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	proof, ok := m.proofs[stationID]
	return proof, ok
}
