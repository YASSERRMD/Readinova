package scoring_test

import (
	"context"
	"math"
	"testing"

	"github.com/YASSERRMD/Readinova/libs/go-scoring/scoring"
)

// minimalFramework builds a single-dimension framework for testing.
func minimalFramework(nQuestions int) scoring.Framework {
	questions := make([]scoring.QuestionDef, nQuestions)
	for i := range questions {
		questions[i] = scoring.QuestionDef{Slug: string(rune('a'+i)) + "_q"}
	}
	return scoring.Framework{
		Slug:    "test-fw",
		Version: "1.0",
		Dimensions: []scoring.DimensionDef{
			{
				Slug:   "dim_a",
				Weight: 1.0,
				SubDimensions: []scoring.SubDimensionDef{
					{
						Slug:   "sd_a",
						Weight: 1.0,
						Indicators: []scoring.IndicatorDef{
							{Slug: "ind_a", Questions: questions},
						},
					},
				},
			},
		},
	}
}

func uniformResponses(fw scoring.Framework, level uint8) []scoring.Response {
	var rs []scoring.Response
	for _, d := range fw.Dimensions {
		for _, sd := range d.SubDimensions {
			for _, ind := range sd.Indicators {
				for _, q := range ind.Questions {
					rs = append(rs, scoring.Response{QuestionSlug: q.Slug, Level: level})
				}
			}
		}
	}
	return rs
}

func TestScore_AllZero(t *testing.T) {
	fw := minimalFramework(3)
	result, err := scoring.Score(context.Background(), fw, uniformResponses(fw, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CompositeLayerA != 0.0 {
		t.Errorf("want 0.0, got %f", result.CompositeLayerA)
	}
}

func TestScore_AllMax(t *testing.T) {
	fw := minimalFramework(3)
	result, err := scoring.Score(context.Background(), fw, uniformResponses(fw, 4))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(result.CompositeLayerA-100.0) > 1e-9 {
		t.Errorf("want 100.0, got %f", result.CompositeLayerA)
	}
}

func TestScore_MidLevel(t *testing.T) {
	fw := minimalFramework(1)
	result, err := scoring.Score(context.Background(), fw, uniformResponses(fw, 2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(result.CompositeLayerA-50.0) > 1e-9 {
		t.Errorf("want 50.0, got %f", result.CompositeLayerA)
	}
}

func TestScore_BindingConstraint(t *testing.T) {
	fw := scoring.Framework{
		Slug:    "bc-fw",
		Version: "1.0",
		Dimensions: []scoring.DimensionDef{
			{
				Slug: "dim_high", Weight: 0.6,
				SubDimensions: []scoring.SubDimensionDef{{
					Slug: "sd_high", Weight: 1.0,
					Indicators: []scoring.IndicatorDef{{
						Slug:      "ind_high",
						Questions: []scoring.QuestionDef{{Slug: "q_high"}},
					}},
				}},
			},
			{
				Slug: "dim_low", Weight: 0.4,
				SubDimensions: []scoring.SubDimensionDef{{
					Slug: "sd_low", Weight: 1.0,
					Indicators: []scoring.IndicatorDef{{
						Slug:      "ind_low",
						Questions: []scoring.QuestionDef{{Slug: "q_low"}},
					}},
				}},
			},
		},
	}
	responses := []scoring.Response{
		{QuestionSlug: "q_high", Level: 4},
		{QuestionSlug: "q_low", Level: 0},
	}
	result, err := scoring.Score(context.Background(), fw, responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BindingConstraintDimension != "dim_low" {
		t.Errorf("want dim_low, got %s", result.BindingConstraintDimension)
	}
}

func TestScore_Idempotent(t *testing.T) {
	fw := minimalFramework(4)
	responses := uniformResponses(fw, 3)
	r1, err := scoring.Score(context.Background(), fw, responses)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	r2, err := scoring.Score(context.Background(), fw, responses)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if r1.CompositeLayerA != r2.CompositeLayerA {
		t.Errorf("not idempotent: %f != %f", r1.CompositeLayerA, r2.CompositeLayerA)
	}
	if r1.EngineVersion != r2.EngineVersion {
		t.Errorf("engine version changed: %s != %s", r1.EngineVersion, r2.EngineVersion)
	}
}

func TestScore_ErrorCases(t *testing.T) {
	fw := minimalFramework(1)
	validQ := fw.Dimensions[0].SubDimensions[0].Indicators[0].Questions[0].Slug

	tests := []struct {
		name      string
		responses []scoring.Response
		wantKind  string
	}{
		{
			name:      "missing response",
			responses: []scoring.Response{},
			wantKind:  "MissingResponses",
		},
		{
			name: "unknown question",
			responses: []scoring.Response{
				{QuestionSlug: validQ, Level: 1},
				{QuestionSlug: "ghost_question", Level: 1},
			},
			wantKind: "UnknownQuestion",
		},
		{
			name:      "invalid level",
			responses: []scoring.Response{{QuestionSlug: validQ, Level: 9}},
			wantKind:  "InvalidLevel",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := scoring.Score(context.Background(), fw, tc.responses)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			se, ok := err.(*scoring.ScoringError)
			if !ok {
				t.Fatalf("expected *ScoringError, got %T: %v", err, err)
			}
			if se.Kind != tc.wantKind {
				t.Errorf("want kind %s, got %s", tc.wantKind, se.Kind)
			}
		})
	}
}

func TestScore_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fw := minimalFramework(1)
	_, err := scoring.Score(ctx, fw, uniformResponses(fw, 2))
	if err == nil {
		t.Fatal("expected cancelled context error, got nil")
	}
}
