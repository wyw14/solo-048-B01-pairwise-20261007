package isolation

import (
	"context"
	"fmt"
	"sync"

	"heatpulse/internal/hydraulic"
)

type Coordinator struct {
	service *Service
	mu      sync.Mutex
	active  map[string]string
}

func NewCoordinator(service *Service) *Coordinator {
	return &Coordinator{service: service, active: make(map[string]string)}
}

func (c *Coordinator) FromIncident(ctx context.Context, incident hydraulic.LeakIncident, sectionID string) (Session, error) {
	if !incident.Confirmed {
		return Session{}, fmt.Errorf("leak incident %s is not confirmed", incident.ID)
	}
	c.mu.Lock()
	if sessionID := c.active[sectionID]; sessionID != "" {
		c.mu.Unlock()
		if session, ok := c.service.Get(sessionID); ok {
			return session, fmt.Errorf("section %s already has an isolation session", sectionID)
		}
	} else {
		c.active[sectionID] = "starting"
	}
	c.mu.Unlock()
	session, err := c.service.Isolate(ctx, sectionID, "leak-protection")
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		delete(c.active, sectionID)
		return session, err
	}
	c.active[sectionID] = session.ID
	return session, nil
}

func (c *Coordinator) Active(sectionID string) (Session, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sessionID := c.active[sectionID]
	if sessionID == "" || sessionID == "starting" {
		return Session{}, false
	}
	return c.service.Get(sessionID)
}
