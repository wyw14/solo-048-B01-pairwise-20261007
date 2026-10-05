package dispatch

import (
	"fmt"
	"time"

	"heatpulse/internal/audit"
	"heatpulse/internal/station"
)

type Publisher struct {
	planner *Planner
	targets *station.TargetService
	audit   *audit.Recorder
}

func NewPublisher(planner *Planner, targets *station.TargetService, recorder *audit.Recorder) *Publisher {
	return &Publisher{planner: planner, targets: targets, audit: recorder}
}

func (p *Publisher) Publish(planID, actor string) (Plan, error) {
	plan, ok := p.planner.Plan(planID)
	if !ok {
		return Plan{}, fmt.Errorf("plan %s not found", planID)
	}
	if plan.Status != PlanCalculating {
		return plan, fmt.Errorf("plan %s is %s", plan.ID, plan.Status)
	}
	if err := p.targets.CompareAndPublish(plan.SourceRevision, plan.Targets); err != nil {
		plan.Status = PlanStale
		plan.Message = err.Error()
		p.planner.save(plan)
		_ = p.audit.Rejected("dispatch.publish", plan.ID, actor, err.Error(), map[string]any{"source_revision": plan.SourceRevision})
		return plan, err
	}
	plan.Status = PlanPublished
	plan.PublishedAt = time.Now().UTC()
	p.planner.save(plan)
	if err := p.audit.Accepted("dispatch.publish", plan.ID, actor, "dispatch targets published", map[string]any{"source_revision": plan.SourceRevision}); err != nil {
		return plan, err
	}
	return plan, nil
}
