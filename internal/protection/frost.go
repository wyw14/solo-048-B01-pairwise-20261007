package protection

import (
	"fmt"

	"heatpulse/internal/valve"
)

type FrostProtection struct {
	operations *valve.Operations
}

func NewFrostProtection(operations *valve.Operations) *FrostProtection {
	return &FrostProtection{operations: operations}
}

func (f *FrostProtection) OpenValve(stationID, valveID string) (valve.Operation, error) {
	operation, err := f.operations.Start(valveID, "frost:"+stationID, 100)
	if err != nil {
		return valve.Operation{}, fmt.Errorf("frost protection valve admission: %w", err)
	}
	completed, err := f.operations.Complete(operation.ID)
	if err != nil {
		return valve.Operation{}, fmt.Errorf("complete frost valve operation: %w", err)
	}
	return completed, nil
}

func (f *FrostProtection) Available(valveID string) bool {
	return f.operations.Owner(valveID) == ""
}
