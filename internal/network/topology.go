package network

import (
	"fmt"
	"sort"
)

type Section struct {
	ID             string   `json:"id"`
	ZoneID         string   `json:"zone_id"`
	Upstream       string   `json:"upstream"`
	Downstream     string   `json:"downstream"`
	BoundaryValves []string `json:"boundary_valves"`
	ReliefValve    string   `json:"relief_valve"`
}

type Topology struct {
	sections map[string]Section
	byZone   map[string][]string
}

func NewTopology(sections []Section) *Topology {
	t := &Topology{sections: make(map[string]Section), byZone: make(map[string][]string)}
	for _, section := range sections {
		section.BoundaryValves = append([]string(nil), section.BoundaryValves...)
		t.sections[section.ID] = section
		t.byZone[section.ZoneID] = append(t.byZone[section.ZoneID], section.ID)
	}
	for zoneID := range t.byZone {
		sort.Strings(t.byZone[zoneID])
	}
	return t
}

func (t *Topology) Section(sectionID string) (Section, error) {
	section, ok := t.sections[sectionID]
	if !ok {
		return Section{}, fmt.Errorf("section %s not found", sectionID)
	}
	section.BoundaryValves = append([]string(nil), section.BoundaryValves...)
	return section, nil
}

func (t *Topology) SectionsForZone(zoneID string) []Section {
	ids := t.byZone[zoneID]
	result := make([]Section, 0, len(ids))
	for _, id := range ids {
		section, err := t.Section(id)
		if err == nil {
			result = append(result, section)
		}
	}
	return result
}

func (t *Topology) Validate() error {
	for _, section := range t.sections {
		if section.ZoneID == "" || section.Upstream == "" || section.Downstream == "" {
			return fmt.Errorf("section %s has incomplete topology", section.ID)
		}
		if len(section.BoundaryValves) < 2 || section.ReliefValve == "" {
			return fmt.Errorf("section %s lacks safe isolation devices", section.ID)
		}
	}
	return nil
}
