package perception_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/YASSERRMD/Readinova/apps/api/internal/perception"
)

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestComputeLayerBEmpty(t *testing.T) {
	result := perception.ComputeLayerB(nil)
	if result.Composite != 0 {
		t.Fatalf("expected 0, got %f", result.Composite)
	}
}

func TestComputeLayerBBoolean(t *testing.T) {
	evidence := []perception.DimensionEvidence{
		{
			DimensionSlug: "strategy",
			Signals: []perception.EvidenceSignal{
				{SignalKey: "has_policy", SignalValue: raw(true)},
				{SignalKey: "has_roadmap", SignalValue: raw(false)},
			},
		},
	}
	result := perception.ComputeLayerB(evidence)
	// avg(100, 0) = 50
	if result.DimensionScores["strategy"] != 50 {
		t.Fatalf("expected 50, got %f", result.DimensionScores["strategy"])
	}
}

func TestComputeLayerBFraction(t *testing.T) {
	evidence := []perception.DimensionEvidence{
		{
			DimensionSlug: "data",
			Signals: []perception.EvidenceSignal{
				{SignalKey: "quality", SignalValue: raw(0.8)},
			},
		},
	}
	result := perception.ComputeLayerB(evidence)
	if result.DimensionScores["data"] != 80 {
		t.Fatalf("expected 80, got %f", result.DimensionScores["data"])
	}
}

func TestComputeGapFormula(t *testing.T) {
	layerB := perception.LayerBResult{
		DimensionScores: map[string]float64{"d": 60},
		Composite:       60,
	}
	result := perception.ComputeGap(80, map[string]float64{"d": 80}, layerB)
	// master = 0.4*80 + 0.5*60 + 0.1*(100-20) = 32+30+8 = 70
	if math.Abs(result.MasterComposite-70) > 0.01 {
		t.Fatalf("expected master=70, got %f", result.MasterComposite)
	}
	if result.GapScore != 20 {
		t.Fatalf("expected gap=20, got %f", result.GapScore)
	}
}
