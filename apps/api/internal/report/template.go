package report

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
)

// Data holds all data needed to render the report.
type Data struct {
	OrgName          string
	AssessmentTitle  string
	Composite        float64
	FrameworkVersion string
	EngineVersion    string
	Dimensions       []DimensionScore
	DerivedIndices   DerivedIndices
	Binding          string
	BindingScore     float64
	FreeTier         bool
}

// DimensionScore is a single dimension result row.
type DimensionScore struct {
	Label   string
	Score   float64
	Rounded int
	Binding bool
}

// DerivedIndices mirrors the scoring engine output.
type DerivedIndices struct {
	ReadinessIndex         float64
	GovernanceRiskScore    float64
	ExecutionCapacityScore float64
	ValueRealisationScore  float64
}

var funcMap = template.FuncMap{
	"round": func(f float64) int { return int(math.Round(f)) },
	"pctWidth": func(f float64) string {
		return fmt.Sprintf("%.1f%%", math.Min(f, 100))
	},
}

// RenderHTML renders the report as an HTML string.
func RenderHTML(d Data) (string, error) {
	t, err := template.New("report").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}
