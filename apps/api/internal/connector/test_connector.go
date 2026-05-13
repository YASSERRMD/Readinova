package connector

import (
	"context"
	"fmt"
)

func init() {
	Register("test", func() Connector { return &TestConnector{} })
}

// TestConnector produces deterministic synthetic signals for dev/testing.
type TestConnector struct {
	connected bool
}

func (c *TestConnector) Type() string { return "test" }

func (c *TestConnector) Connect(_ context.Context, _ map[string]any) error {
	c.connected = true
	return nil
}

func (c *TestConnector) Collect(_ context.Context, dimensions []string) ([]Signal, error) {
	if !c.connected {
		return nil, fmt.Errorf("connector not connected")
	}

	all := []Signal{
		{DimensionSlug: "strategy_alignment", SignalKey: "policy_doc_count", SignalValue: 12},
		{DimensionSlug: "strategy_alignment", SignalKey: "ai_strategy_present", SignalValue: true},
		{DimensionSlug: "data_governance", SignalKey: "data_catalogue_entries", SignalValue: 340},
		{DimensionSlug: "data_governance", SignalKey: "data_quality_score", SignalValue: 0.82},
		{DimensionSlug: "technology_infrastructure", SignalKey: "cloud_adoption_pct", SignalValue: 0.65},
		{DimensionSlug: "technology_infrastructure", SignalKey: "api_coverage_pct", SignalValue: 0.71},
		{DimensionSlug: "talent_culture", SignalKey: "ai_trained_staff_pct", SignalValue: 0.28},
		{DimensionSlug: "talent_culture", SignalKey: "innovation_labs_count", SignalValue: 2},
		{DimensionSlug: "ethics_governance", SignalKey: "ai_ethics_policy", SignalValue: true},
		{DimensionSlug: "ethics_governance", SignalKey: "bias_audits_completed", SignalValue: 1},
	}

	if len(dimensions) == 0 {
		return all, nil
	}

	wanted := map[string]bool{}
	for _, d := range dimensions {
		wanted[d] = true
	}
	filtered := make([]Signal, 0, len(all))
	for _, s := range all {
		if wanted[s.DimensionSlug] {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

func (c *TestConnector) Disconnect(_ context.Context) error {
	c.connected = false
	return nil
}
