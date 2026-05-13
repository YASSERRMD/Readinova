package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/YASSERRMD/Readinova/apps/api/internal/report"
)

func init() {
	// Report routes registered in server.go via reportRoutes()
}

// reportRoutes adds report endpoints to the mux.
func (s *Server) reportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/assessments/{id}/report", s.withAuth(s.handleReport))
}

// GET /v1/assessments/{id}/report?format=html|pdf
func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	claims := claimsFromCtx(r)
	assessmentID := r.PathValue("id")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "html"
	}

	// Resolve org name and assessment title.
	var orgName, assessmentTitle string
	if err := s.db.QueryRow(r.Context(),
		`SELECT o.name, a.title
		 FROM assessments a JOIN organisations o ON o.id = a.organisation_id
		 WHERE a.id = $1 AND a.organisation_id = $2`,
		assessmentID, claims.OrgID,
	).Scan(&orgName, &assessmentTitle); err != nil {
		writeError(w, http.StatusNotFound, "assessment not found")
		return
	}

	// Load latest scoring result.
	var resultJSON []byte
	if err := s.db.QueryRow(r.Context(),
		`SELECT result_json FROM scoring_runs
		 WHERE assessment_id = $1 AND status = 'completed'
		 ORDER BY created_at DESC LIMIT 1`,
		assessmentID,
	).Scan(&resultJSON); err != nil {
		writeError(w, http.StatusNotFound, "no scoring result available")
		return
	}

	var sr struct {
		CompositeLayerA            float64            `json:"composite_layer_a"`
		DimensionScores            map[string]float64 `json:"dimension_scores"`
		BindingConstraintDimension string             `json:"binding_constraint_dimension"`
		BindingConstraintScore     float64            `json:"binding_constraint_score"`
		Derived                    struct {
			ReadinessIndex         float64 `json:"readiness_index"`
			GovernanceRiskScore    float64 `json:"governance_risk_score"`
			ExecutionCapacityScore float64 `json:"execution_capacity_score"`
			ValueRealisationScore  float64 `json:"value_realisation_score"`
		} `json:"derived"`
		EngineVersion    string `json:"engine_version"`
		FrameworkVersion string `json:"framework_version"`
	}
	if err := json.Unmarshal(resultJSON, &sr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse scoring result")
		return
	}

	// Determine tier (free tier = viewer role).
	freeTier := claims.Role == "viewer"

	// Build dimension list.
	dims := make([]report.DimensionScore, 0, len(sr.DimensionScores))
	for slug, score := range sr.DimensionScores {
		label := strings.ReplaceAll(slug, "_", " ")
		label = strings.ToUpper(label[:1]) + label[1:]
		rounded := int(score + 0.5)
		dims = append(dims, report.DimensionScore{
			Label:   label,
			Score:   score,
			Rounded: rounded,
			Binding: slug == sr.BindingConstraintDimension,
		})
	}

	data := report.Data{
		OrgName:         orgName,
		AssessmentTitle: assessmentTitle,
		Composite:       sr.CompositeLayerA,
		FrameworkVersion: sr.FrameworkVersion,
		EngineVersion:   sr.EngineVersion,
		Dimensions:      dims,
		DerivedIndices: report.DerivedIndices{
			ReadinessIndex:         sr.Derived.ReadinessIndex,
			GovernanceRiskScore:    sr.Derived.GovernanceRiskScore,
			ExecutionCapacityScore: sr.Derived.ExecutionCapacityScore,
			ValueRealisationScore:  sr.Derived.ValueRealisationScore,
		},
		Binding:      sr.BindingConstraintDimension,
		BindingScore: sr.BindingConstraintScore,
		FreeTier:     freeTier,
	}

	htmlContent, err := report.RenderHTML(data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to render report")
		return
	}

	if format == "pdf" {
		pdfBytes, err := report.GeneratePDF(r.Context(), htmlContent)
		if err != nil {
			// Fall back to HTML if Chrome is not available.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("X-Report-Fallback", "chromedp-unavailable")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(htmlContent))
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="readiness-report.pdf"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdfBytes)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(htmlContent))
}
