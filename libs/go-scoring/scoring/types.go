package scoring

import "encoding/json"

// Framework is the minimal framework definition passed to the scoring core.
type Framework struct {
	Slug       string         `json:"slug"`
	Version    string         `json:"version"`
	Dimensions []DimensionDef `json:"dimensions"`
}

// DimensionDef is a top-level dimension.
type DimensionDef struct {
	Slug          string            `json:"slug"`
	Weight        float64           `json:"weight"`
	SubDimensions []SubDimensionDef `json:"sub_dimensions"`
}

// SubDimensionDef is a sub-dimension within a dimension.
type SubDimensionDef struct {
	Slug       string         `json:"slug"`
	Weight     float64        `json:"weight"`
	Indicators []IndicatorDef `json:"indicators"`
}

// IndicatorDef groups questions within a sub-dimension.
type IndicatorDef struct {
	Slug      string        `json:"slug"`
	Questions []QuestionDef `json:"questions"`
}

// QuestionDef holds a question slug.
type QuestionDef struct {
	Slug string `json:"slug"`
}

// Response is a single assessor response.
type Response struct {
	QuestionSlug string `json:"question_slug"`
	Level        uint8  `json:"level"`
}

// DerivedIndices holds the four composite indices.
type DerivedIndices struct {
	ReadinessIndex         float64 `json:"readiness_index"`
	GovernanceRiskScore    float64 `json:"governance_risk_score"`
	ExecutionCapacityScore float64 `json:"execution_capacity_score"`
	ValueRealisationScore  float64 `json:"value_realisation_score"`
}

// ScoringResult is the full result from the Rust scoring core.
type ScoringResult struct {
	CompositeLayerA            float64            `json:"composite_layer_a"`
	DimensionScores            map[string]float64 `json:"dimension_scores"`
	SubDimensionScores         map[string]float64 `json:"sub_dimension_scores"`
	BindingConstraintDimension string             `json:"binding_constraint_dimension"`
	BindingConstraintScore     float64            `json:"binding_constraint_score"`
	Derived                    DerivedIndices     `json:"derived"`
	EngineVersion              string             `json:"engine_version"`
	FrameworkVersion           string             `json:"framework_version"`
}

// ScoringError mirrors the ScoringError enum from Rust.
type ScoringError struct {
	Kind        string   `json:"kind"`
	Slugs       []string `json:"slugs,omitempty"`
	Slug        string   `json:"slug,omitempty"`
	Level       *uint8   `json:"level,omitempty"`
	Description string   `json:"description,omitempty"`
}

func (e *ScoringError) Error() string {
	switch e.Kind {
	case "MissingResponses":
		return "missing responses for: " + joinStrings(e.Slugs)
	case "UnknownQuestion":
		return "unknown question slug: " + e.Slug
	case "InvalidLevel":
		return "invalid level for question: " + e.Slug
	case "FrameworkInvariantViolation":
		return "framework invariant violation: " + e.Description
	default:
		return "scoring error: " + e.Kind
	}
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += ", " + s
	}
	return out
}

// ffiInput is the JSON body sent to the Rust engine.
type ffiInput struct {
	Framework Framework  `json:"framework"`
	Responses []Response `json:"responses"`
}

// ffiEnvelope is the JSON body returned by the Rust engine.
type ffiEnvelope struct {
	OK     bool           `json:"ok"`
	Result *ScoringResult `json:"result,omitempty"`
	Error  *ScoringError  `json:"error,omitempty"`
}

// ScoringInput is the public input to Score.
type ScoringInput struct {
	Framework json.RawMessage `json:"framework"`
	Responses json.RawMessage `json:"responses"`
}
