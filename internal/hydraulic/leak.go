package hydraulic

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type LeakIncident struct {
	ID        string    `json:"id"`
	StationID string    `json:"station_id"`
	Watermark uint64    `json:"watermark"`
	Ratio     float64   `json:"ratio"`
	Confirmed bool      `json:"confirmed"`
	At        time.Time `json:"at"`
}

type LeakDetector struct {
	mu        sync.Mutex
	last      map[string]Balance
	incidents []LeakIncident
}

func NewLeakDetector() *LeakDetector {
	return &LeakDetector{last: make(map[string]Balance), incidents: make([]LeakIncident, 0)}
}

func (d *LeakDetector) Observe(balance Balance) (*LeakIncident, error) {
	if balance.StationID == "" || balance.Watermark == 0 {
		return nil, fmt.Errorf("balance identity is incomplete")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	previous, seen := d.last[balance.StationID]
	d.last[balance.StationID] = balance
	if !balance.LeakLikely {
		return nil, nil
	}
	confirmed := seen && previous.LeakLikely && previous.Watermark < balance.Watermark
	incident := LeakIncident{
		ID: uuid.NewString(), StationID: balance.StationID, Watermark: balance.Watermark,
		Ratio: balance.ImbalanceRatio, Confirmed: confirmed, At: time.Now().UTC(),
	}
	d.incidents = append(d.incidents, incident)
	return &incident, nil
}

func (d *LeakDetector) Incidents() []LeakIncident {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]LeakIncident(nil), d.incidents...)
}
