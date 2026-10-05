package station

import (
	"fmt"
	"sync"
	"time"
)

type TargetService struct {
	registry *Registry
	mu       sync.Mutex
	revision uint64
}

func NewTargetService(registry *Registry) *TargetService {
	var revision uint64
	for _, item := range registry.List() {
		if item.TargetRevision > revision {
			revision = item.TargetRevision
		}
	}
	return &TargetService{registry: registry, revision: revision}
}

func (s *TargetService) Revision() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revision
}

func (s *TargetService) ManualHold(stationIDs []string, actor string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	for _, stationID := range stationIDs {
		if _, err := s.registry.Update(stationID, func(item *Station) error {
			item.Hold = true
			item.TargetRevision = s.revision
			return nil
		}); err != nil {
			return 0, fmt.Errorf("hold %s by %s: %w", stationID, actor, err)
		}
	}
	return s.revision, nil
}

func (s *TargetService) ReleaseHold(stationID string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	_, err := s.registry.Update(stationID, func(item *Station) error {
		item.Hold = false
		item.TargetRevision = s.revision
		return nil
	})
	return s.revision, err
}

func (s *TargetService) CompareAndPublish(sourceRevision uint64, targets map[string]float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sourceRevision != s.revision {
		return fmt.Errorf("dispatch source revision %d is stale; current is %d", sourceRevision, s.revision)
	}
	s.revision++
	for stationID, celsius := range targets {
		if celsius < 35 || celsius > 95 {
			return fmt.Errorf("target %.1f for %s outside operating envelope", celsius, stationID)
		}
		_, err := s.registry.Update(stationID, func(item *Station) error {
			if item.Hold {
				return fmt.Errorf("station %s is manually held", stationID)
			}
			item.TargetCelsius = celsius
			item.TargetRevision = s.revision
			item.UpdatedAt = time.Now().UTC()
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
