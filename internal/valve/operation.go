package valve

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type OperationStatus string

const (
	OperationPending   OperationStatus = "pending"
	OperationMoving    OperationStatus = "moving"
	OperationCompleted OperationStatus = "completed"
	OperationCanceled  OperationStatus = "canceled"
	OperationFailed    OperationStatus = "failed"
)

type Operation struct {
	ID        string          `json:"id"`
	ValveID   string          `json:"valve_id"`
	Owner     string          `json:"owner"`
	Target    float64         `json:"target"`
	Status    OperationStatus `json:"status"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   time.Time       `json:"ended_at,omitempty"`
}

type Operations struct {
	mu         sync.Mutex
	owners     map[string]string
	operations map[string]Operation
}

func NewOperations() *Operations {
	return &Operations{owners: make(map[string]string), operations: make(map[string]Operation)}
}

func (o *Operations) Start(valveID, owner string, target float64) (Operation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if current := o.owners[valveID]; current != "" {
		return Operation{}, fmt.Errorf("valve owned by %s", current)
	}
	operation := Operation{
		ID: uuid.NewString(), ValveID: valveID, Owner: owner, Target: target,
		Status: OperationMoving, StartedAt: time.Now().UTC(),
	}
	o.owners[valveID] = operation.ID
	o.operations[operation.ID] = operation
	return operation, nil
}

func (o *Operations) finish(operationID string, status OperationStatus) (Operation, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	operation, ok := o.operations[operationID]
	if !ok {
		return Operation{}, fmt.Errorf("operation %s not found", operationID)
	}
	if operation.Status != OperationMoving && operation.Status != OperationPending {
		return Operation{}, fmt.Errorf("operation %s already terminal", operationID)
	}
	operation.Status = status
	operation.EndedAt = time.Now().UTC()
	if o.owners[operation.ValveID] == operationID {
		delete(o.owners, operation.ValveID)
	}
	o.operations[operationID] = operation
	return operation, nil
}

func (o *Operations) Complete(operationID string) (Operation, error) {
	return o.finish(operationID, OperationCompleted)
}

func (o *Operations) Fail(operationID string) (Operation, error) {
	return o.finish(operationID, OperationFailed)
}

func (o *Operations) Abort(operationID string) (Operation, error) {
	return o.finish(operationID, OperationCanceled)
}

func (o *Operations) Owner(valveID string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.owners[valveID]
}

func (o *Operations) Get(operationID string) (Operation, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	operation, ok := o.operations[operationID]
	return operation, ok
}
