// Package perception implements the Layer B evidence aggregation and perception gap engine.
package perception

import (
	"encoding/json"
	"math"
)

// DimensionEvidence holds evidence signals for a single dimension.
type DimensionEvidence struct {
	DimensionSlug string
	Signals       []EvidenceSignal
}

// EvidenceSignal is a single evidence datum from a connector.
type EvidenceSignal struct {
	ConnectorType string
	SignalKey     string
	SignalValue   json.RawMessage
}

// LayerBResult is the output of Layer B aggregation.
type LayerBResult struct {
	DimensionScores map[string]float64 `json:"dimension_scores_b"`
	Composite       float64            `json:"composite_layer_b"`
}

// GapResult holds the full perception gap computation.
type GapResult struct {
	LayerAScore     float64            `json:"layer_a_score"`
	LayerBScore     float64            `json:"layer_b_score"`
	GapScore        float64            `json:"gap_score"`  // S - E
	MasterComposite float64            `json:"master_composite"` // 0.4S + 0.5E + 0.1*(100-|P|)
	DimensionScoresA map[string]float64 `json:"dimension_scores_a"`
	DimensionScoresB map[string]float64 `json:"dimension_scores_b"`
}

// ComputeLayerB aggregates evidence signals per dimension into a 0-100 score.
// Each dimension score is derived from the average of normalised signal values.
// Boolean true = 100, false = 0; percentages (0-1) scaled ×100; raw counts capped at 100.
func ComputeLayerB(evidence []DimensionEvidence) LayerBResult {
	dimScores := map[string]float64{}

	for _, de := range evidence {
		if len(de.Signals) == 0 {
			continue
		}
		total := 0.0
		for _, sig := range de.Signals {
			total += normalise(sig.SignalValue)
		}
		dimScores[de.DimensionSlug] = math.Min(100, total/float64(len(de.Signals)))
	}

	composite := 0.0
	if len(dimScores) > 0 {
		sum := 0.0
		for _, v := range dimScores {
			sum += v
		}
		composite = sum / float64(len(dimScores))
	}

	return LayerBResult{
		DimensionScores: dimScores,
		Composite:       math.Round(composite*100) / 100,
	}
}

// ComputeGap computes the perception gap and master composite.
func ComputeGap(layerA float64, dimScoresA map[string]float64, layerB LayerBResult) GapResult {
	gap := layerA - layerB.Composite
	absGap := math.Abs(gap)
	master := 0.4*layerA + 0.5*layerB.Composite + 0.1*(100-absGap)
	master = math.Min(100, math.Max(0, master))

	return GapResult{
		LayerAScore:      math.Round(layerA*100) / 100,
		LayerBScore:      math.Round(layerB.Composite*100) / 100,
		GapScore:         math.Round(gap*100) / 100,
		MasterComposite:  math.Round(master*100) / 100,
		DimensionScoresA: dimScoresA,
		DimensionScoresB: layerB.DimensionScores,
	}
}

// normalise converts a JSON signal value to a 0-100 score.
func normalise(raw json.RawMessage) float64 {
	// Boolean
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		if b {
			return 100
		}
		return 0
	}
	// Numeric
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		if f >= 0 && f <= 1 {
			return f * 100 // treat as fraction
		}
		return math.Min(100, f) // treat as raw count / score
	}
	return 0
}
