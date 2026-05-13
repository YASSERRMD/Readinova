package scoring_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/YASSERRMD/Readinova/libs/go-scoring/scoring"
)

type goldenFixture struct {
	Framework scoring.Framework  `json:"framework"`
	Responses []scoring.Response `json:"responses"`
}

type goldenExpected struct {
	CompositeLayerA            float64            `json:"composite_layer_a"`
	DimensionScores            map[string]float64 `json:"dimension_scores"`
	SubDimensionScores         map[string]float64 `json:"sub_dimension_scores"`
	BindingConstraintDimension string             `json:"binding_constraint_dimension"`
	BindingConstraintScore     float64            `json:"binding_constraint_score"`
}

func goldenDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve caller path")
	}
	// golden files live in crates/scoring/tests/golden/
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../crates/scoring/tests/golden"))
}

// TestGoldenParityWithRust ensures the Go FFI path produces the same composite
// and dimension scores as the Rust golden fixture.
func TestGoldenParityWithRust(t *testing.T) {
	dir := goldenDir(t)

	fixturePath := filepath.Join(dir, "fixture.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Skipf("golden fixture not found (%v); build the Rust crate first", err)
	}

	var fixture goldenFixture
	if err := json.Unmarshal(fixtureData, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	expectedPath := filepath.Join(dir, "expected_output.json")
	expectedData, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Skipf("golden expected_output not found (%v); run cargo test first", err)
	}

	var expected goldenExpected
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		t.Fatalf("parse expected: %v", err)
	}

	result, err := scoring.Score(context.Background(), fixture.Framework, fixture.Responses)
	if err != nil {
		t.Fatalf("Score failed: %v", err)
	}

	const eps = 1e-4
	if math.Abs(result.CompositeLayerA-expected.CompositeLayerA) > eps {
		t.Errorf("composite: want %.4f, got %.4f", expected.CompositeLayerA, result.CompositeLayerA)
	}

	for slug, want := range expected.DimensionScores {
		got, ok := result.DimensionScores[slug]
		if !ok {
			t.Errorf("dimension %q missing from result", slug)
			continue
		}
		if math.Abs(got-want) > eps {
			t.Errorf("dimension %q: want %.4f, got %.4f", slug, want, got)
		}
	}

	if result.BindingConstraintDimension != expected.BindingConstraintDimension {
		t.Errorf("binding constraint dimension: want %q, got %q",
			expected.BindingConstraintDimension, result.BindingConstraintDimension)
	}
}
