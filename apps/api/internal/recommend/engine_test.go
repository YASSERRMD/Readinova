package recommend_test

import (
	"testing"

	"github.com/YASSERRMD/Readinova/apps/api/internal/recommend"
)

func TestGenerateNone(t *testing.T) {
	// All dimensions at 100 — no recommendations expected.
	scores := map[string]float64{
		"strategy_alignment":      100,
		"data_governance":         100,
		"technology_infrastructure": 100,
		"talent_culture":          100,
		"ethics_governance":       100,
	}
	recs, err := recommend.Generate(scores)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations at 100, got %d", len(recs))
	}
}

func TestGenerateLowScores(t *testing.T) {
	scores := map[string]float64{
		"strategy_alignment":      20,
		"data_governance":         20,
		"technology_infrastructure": 20,
		"talent_culture":          20,
		"ethics_governance":       20,
	}
	recs, err := recommend.Generate(scores)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected recommendations for low scores")
	}
	// Wave 1 should come before wave 2 and 3.
	for i := 1; i < len(recs); i++ {
		if recs[i-1].Wave > recs[i].Wave {
			t.Fatalf("recommendations out of wave order at index %d: wave %d > wave %d",
				i, recs[i-1].Wave, recs[i].Wave)
		}
	}
}

func TestGenerateWaveAssignment(t *testing.T) {
	scores := map[string]float64{"strategy_alignment": 10}
	recs, err := recommend.Generate(scores)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range recs {
		if r.Wave < 1 || r.Wave > 3 {
			t.Fatalf("wave out of range 1-3: %d", r.Wave)
		}
		if r.Priority <= 0 {
			t.Fatalf("non-positive priority: %f", r.Priority)
		}
	}
}
