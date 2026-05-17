// Package recommend generates prioritised, wave-grouped recommendations from scoring results.
package recommend

import (
	"embed"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*.yaml
var templateFS embed.FS

// templateCache holds the parsed template files so we only read and unmarshal
// the embedded YAML once per process lifetime (the embed.FS is static).
var (
	templateOnce    sync.Once
	cachedTemplates []templateFile
	cacheErr        error
)

// Recommendation is a single actionable recommendation.
type Recommendation struct {
	ID            string  `json:"id"`
	DimensionSlug string  `json:"dimension_slug"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Effort        string  `json:"effort"`   // low | medium | high
	Impact        string  `json:"impact"`   // low | medium | high
	Priority      float64 `json:"priority"` // higher = more urgent
	Wave          int     `json:"wave"`     // 1 = now, 2 = next, 3 = later
}

type templateFile struct {
	Dimension       string `yaml:"dimension"`
	Recommendations []struct {
		ID          string  `yaml:"id"`
		Title       string  `yaml:"title"`
		Description string  `yaml:"description"`
		Effort      string  `yaml:"effort"`
		Impact      string  `yaml:"impact"`
		Threshold   float64 `yaml:"threshold"`
	} `yaml:"recommendations"`
}

// loadTemplates parses all embedded YAML templates once and caches the result.
func loadTemplates() ([]templateFile, error) {
	templateOnce.Do(func() {
		entries, err := templateFS.ReadDir("templates")
		if err != nil {
			cacheErr = fmt.Errorf("read templates: %w", err)
			return
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}
			data, err := templateFS.ReadFile("templates/" + entry.Name())
			if err != nil {
				continue
			}
			var tf templateFile
			if err := yaml.Unmarshal(data, &tf); err != nil {
				continue
			}
			cachedTemplates = append(cachedTemplates, tf)
		}
	})
	return cachedTemplates, cacheErr
}

// Generate returns prioritised, wave-grouped recommendations based on dimension scores.
// dimensionScores is a map of dimension_slug → score (0–100).
func Generate(dimensionScores map[string]float64) ([]Recommendation, error) {
	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}

	var all []Recommendation

	for _, tf := range templates {
		score, hasScore := dimensionScores[tf.Dimension]

		for _, t := range tf.Recommendations {
			// Only include recommendation if dimension score is below the threshold.
			if hasScore && score >= t.Threshold {
				continue
			}
			// If no score for this dimension, include all recommendations.
			gap := t.Threshold - score
			if !hasScore {
				gap = 50 // default gap
			}
			priority := computePriority(gap, t.Impact, t.Effort)
			wave := waveFor(priority)

			all = append(all, Recommendation{
				ID:            t.ID,
				DimensionSlug: tf.Dimension,
				Title:         t.Title,
				Description:   t.Description,
				Effort:        t.Effort,
				Impact:        t.Impact,
				Priority:      math.Round(priority*100) / 100,
				Wave:          wave,
			})
		}
	}

	// Sort by priority descending.
	sort.Slice(all, func(i, j int) bool {
		if all[i].Wave != all[j].Wave {
			return all[i].Wave < all[j].Wave
		}
		return all[i].Priority > all[j].Priority
	})

	return all, nil
}

// computePriority scores a recommendation: gap contribution × impact bonus ÷ effort penalty.
func computePriority(gap float64, impact, effort string) float64 {
	impactMult := map[string]float64{"low": 0.5, "medium": 1.0, "high": 1.5}
	effortDiv := map[string]float64{"low": 0.8, "medium": 1.0, "high": 1.3}

	im := impactMult[impact]
	if im == 0 {
		im = 1.0
	}
	ed := effortDiv[effort]
	if ed == 0 {
		ed = 1.0
	}
	return (gap * im) / ed
}

// waveFor maps priority score to a wave number.
func waveFor(priority float64) int {
	if priority >= 60 {
		return 1
	}
	if priority >= 30 {
		return 2
	}
	return 3
}
