package station

import (
	"fmt"

	"heatpulse/internal/valve"
)

type ManualService struct {
	operations *valve.Operations
}

func NewManualService(operations *valve.Operations) *ManualService {
	return &ManualService{operations: operations}
}

func (s *ManualService) MoveValve(stationID, valveID string, target float64) (valve.Operation, error) {
	if target < 0 || target > 100 {
		return valve.Operation{}, fmt.Errorf("valve target %.1f outside range", target)
	}
	return s.operations.Start(valveID, "manual:"+stationID, target)
}

func (s *ManualService) Cancel(operationID string) (valve.Operation, error) {
	return s.operations.Abort(operationID)
}

func (s *ManualService) Complete(operationID string) (valve.Operation, error) {
	return s.operations.Complete(operationID)
}

func (s *ManualService) Operation(operationID string) (valve.Operation, bool) {
	return s.operations.Get(operationID)
}
