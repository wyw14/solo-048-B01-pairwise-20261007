package exchanger

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"heatpulse/internal/station"
	"heatpulse/internal/telemetry"
	"heatpulse/internal/valve"
)

type SwitchStatus string

const (
	SwitchPriming   SwitchStatus = "priming"
	SwitchCompleted SwitchStatus = "completed"
	SwitchFailed    SwitchStatus = "failed"
)

type SwitchOperation struct {
	ID              string       `json:"id"`
	StationID       string       `json:"station_id"`
	SourceExchanger string       `json:"source_exchanger"`
	TargetExchanger string       `json:"target_exchanger"`
	Status          SwitchStatus `json:"status"`
	StartedAt       time.Time    `json:"started_at"`
	CompletedAt     time.Time    `json:"completed_at,omitempty"`
}

type SwitchService struct {
	stations *station.Registry
	receipts *telemetry.FlowReceipts
	primary  *valve.PrimaryService
	mu       sync.RWMutex
	active   map[string]SwitchOperation
}

func NewSwitchService(stations *station.Registry, receipts *telemetry.FlowReceipts, primary *valve.PrimaryService) *SwitchService {
	return &SwitchService{stations: stations, receipts: receipts, primary: primary, active: make(map[string]SwitchOperation)}
}

func (s *SwitchService) Begin(stationID, sourceID, targetID string) (SwitchOperation, error) {
	if sourceID == targetID || sourceID == "" || targetID == "" {
		return SwitchOperation{}, fmt.Errorf("source and target exchangers must differ")
	}
	if _, ok := s.stations.Get(stationID); !ok {
		return SwitchOperation{}, fmt.Errorf("station %s not found", stationID)
	}
	operation := SwitchOperation{
		ID: uuid.NewString(), StationID: stationID, SourceExchanger: sourceID,
		TargetExchanger: targetID, Status: SwitchPriming, StartedAt: time.Now().UTC(),
	}
	if _, err := s.stations.SetExchanger(stationID, sourceID, station.ExchangerDraining); err != nil {
		return SwitchOperation{}, err
	}
	if _, err := s.stations.SetExchanger(stationID, targetID, station.ExchangerPriming); err != nil {
		return SwitchOperation{}, err
	}
	s.mu.Lock()
	s.active[operation.ID] = operation
	s.mu.Unlock()
	return operation, nil
}

func (s *SwitchService) Advance(operationID string) (SwitchOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.active[operationID]
	if !ok {
		return SwitchOperation{}, fmt.Errorf("switch operation %s not found", operationID)
	}
	confirmed := s.receipts.Confirmed(operation.StationID, operation.TargetExchanger, operation.ID)
	if !confirmed {
		return operation, fmt.Errorf("target exchanger flow not confirmed")
	}
	if _, err := s.primary.Open(operation.StationID, operation.TargetExchanger, operation.ID, confirmed); err != nil {
		return operation, err
	}
	if _, err := s.stations.SetExchanger(operation.StationID, operation.SourceExchanger, station.ExchangerStandby); err != nil {
		return operation, err
	}
	if _, err := s.stations.SetExchanger(operation.StationID, operation.TargetExchanger, station.ExchangerCarrying); err != nil {
		return operation, err
	}
	operation.Status = SwitchCompleted
	operation.CompletedAt = time.Now().UTC()
	s.active[operationID] = operation
	return operation, nil
}

func (s *SwitchService) Operation(operationID string) (SwitchOperation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	operation, ok := s.active[operationID]
	return operation, ok
}
