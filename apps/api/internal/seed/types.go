// Package seed defines the YAML types and validation logic for framework seed files.
package seed

import "fmt"

// Framework is the root structure representing one dimension YAML file merged
// into a full framework definition. The loader collects all dimension files and
// combines them into a single Framework for insertion.
type Framework struct {
	Slug         string      `yaml:"slug"`
	Name         string      `yaml:"name"`
	Description  string      `yaml:"description"`
	VersionMajor int         `yaml:"version_major"`
	VersionMinor int         `yaml:"version_minor"`
	Dimensions   []Dimension `yaml:"-"` // populated by loader, not decoded directly
}

// Dimension maps to a single YAML seed file.
type Dimension struct {
	Slug          string         `yaml:"slug"`
	Name          string         `yaml:"name"`
	Description   string         `yaml:"description"`
	DefaultWeight string         `yaml:"default_weight"`
	DisplayOrder  int            `yaml:"display_order"`
	SubDimensions []SubDimension `yaml:"sub_dimensions"`
}

// SubDimension is a sub-dimension within a dimension.
type SubDimension struct {
	Slug                  string      `yaml:"slug"`
	Name                  string      `yaml:"name"`
	Description           string      `yaml:"description"`
	WeightWithinDimension string      `yaml:"weight_within_dimension"`
	DisplayOrder          int         `yaml:"display_order"`
	Indicators            []Indicator `yaml:"indicators"`
}

// Indicator is an indicator within a sub-dimension.
type Indicator struct {
	Slug         string     `yaml:"slug"`
	Name         string     `yaml:"name"`
	Description  string     `yaml:"description"`
	DisplayOrder int        `yaml:"display_order"`
	Questions    []Question `yaml:"questions"`
}

// Question is a single assessment question.
type Question struct {
	Slug                  string                 `yaml:"slug"`
	Prompt                string                 `yaml:"prompt"`
	TargetRole            string                 `yaml:"target_role"`
	DisplayOrder          int                    `yaml:"display_order"`
	RegulatoryReferences  map[string]interface{} `yaml:"regulatory_references"`
	Rubric                []RubricLevel          `yaml:"rubric"`
}

// RubricLevel is one of the five maturity levels for a question.
type RubricLevel struct {
	Level       int    `yaml:"level"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
	Score       string `yaml:"score"`
}

var validRoles = map[string]bool{
	"executive": true,
	"cio":       true,
	"risk":      true,
	"ops":       true,
	"any":       true,
}

var expectedLabels = map[int]string{
	0: "Absent",
	1: "Ad Hoc",
	2: "Defined",
	3: "Managed",
	4: "Optimised",
}

// Validate checks that the dimension satisfies all framework invariants that can
// be checked without a database connection. It returns the first error found.
func (d *Dimension) Validate() error {
	if d.Slug == "" {
		return fmt.Errorf("dimension missing slug")
	}
	if d.Name == "" {
		return fmt.Errorf("dimension %q missing name", d.Slug)
	}
	if d.Description == "" {
		return fmt.Errorf("dimension %q missing description", d.Slug)
	}
	if d.DefaultWeight == "" {
		return fmt.Errorf("dimension %q missing default_weight", d.Slug)
	}
	if d.DisplayOrder < 1 {
		return fmt.Errorf("dimension %q: display_order must be >= 1", d.Slug)
	}
	if len(d.SubDimensions) < 1 {
		return fmt.Errorf("dimension %q has no sub_dimensions", d.Slug)
	}
	for i, sd := range d.SubDimensions {
		if err := sd.validate(d.Slug, i+1); err != nil {
			return err
		}
	}
	return nil
}

func (sd *SubDimension) validate(dimSlug string, pos int) error {
	if sd.Slug == "" {
		return fmt.Errorf("dimension %q sub_dimension[%d] missing slug", dimSlug, pos)
	}
	if sd.Name == "" {
		return fmt.Errorf("dimension %q sub_dimension %q missing name", dimSlug, sd.Slug)
	}
	if sd.WeightWithinDimension == "" {
		return fmt.Errorf("dimension %q sub_dimension %q missing weight_within_dimension", dimSlug, sd.Slug)
	}
	if sd.DisplayOrder < 1 {
		return fmt.Errorf("dimension %q sub_dimension %q: display_order must be >= 1", dimSlug, sd.Slug)
	}
	if len(sd.Indicators) < 1 {
		return fmt.Errorf("dimension %q sub_dimension %q has no indicators", dimSlug, sd.Slug)
	}
	for i, ind := range sd.Indicators {
		if err := ind.validate(dimSlug, sd.Slug, i+1); err != nil {
			return err
		}
	}
	return nil
}

func (ind *Indicator) validate(dimSlug, sdSlug string, pos int) error {
	if ind.Slug == "" {
		return fmt.Errorf("dimension %q sub_dimension %q indicator[%d] missing slug", dimSlug, sdSlug, pos)
	}
	if ind.Name == "" {
		return fmt.Errorf("dimension %q sub_dimension %q indicator %q missing name", dimSlug, sdSlug, ind.Slug)
	}
	if ind.DisplayOrder < 1 {
		return fmt.Errorf("dimension %q sub_dimension %q indicator %q: display_order must be >= 1", dimSlug, sdSlug, ind.Slug)
	}
	if len(ind.Questions) < 1 {
		return fmt.Errorf("dimension %q sub_dimension %q indicator %q has no questions", dimSlug, sdSlug, ind.Slug)
	}
	for i, q := range ind.Questions {
		if err := q.validate(dimSlug, sdSlug, ind.Slug, i+1); err != nil {
			return err
		}
	}
	return nil
}

func (q *Question) validate(dimSlug, sdSlug, indSlug string, pos int) error {
	loc := fmt.Sprintf("dimension %q sub_dimension %q indicator %q question[%d]", dimSlug, sdSlug, indSlug, pos)
	if q.Slug == "" {
		return fmt.Errorf("%s missing slug", loc)
	}
	if q.Prompt == "" {
		return fmt.Errorf("%s missing prompt", loc)
	}
	if !validRoles[q.TargetRole] {
		return fmt.Errorf("%s invalid target_role %q (must be executive, cio, risk, ops, or any)", loc, q.TargetRole)
	}
	if q.DisplayOrder < 1 {
		return fmt.Errorf("%s display_order must be >= 1", loc)
	}
	if len(q.Rubric) != 5 {
		return fmt.Errorf("%s must have exactly 5 rubric levels, got %d", loc, len(q.Rubric))
	}
	for _, rl := range q.Rubric {
		if err := rl.validate(loc); err != nil {
			return err
		}
	}
	// verify all levels 0-4 are present
	levelSeen := make(map[int]bool)
	for _, rl := range q.Rubric {
		levelSeen[rl.Level] = true
	}
	for level := 0; level <= 4; level++ {
		if !levelSeen[level] {
			return fmt.Errorf("%s missing rubric level %d", loc, level)
		}
	}
	return nil
}

func (rl *RubricLevel) validate(questionLoc string) error {
	if rl.Level < 0 || rl.Level > 4 {
		return fmt.Errorf("%s rubric level %d out of range [0,4]", questionLoc, rl.Level)
	}
	expected, ok := expectedLabels[rl.Level]
	if !ok || rl.Label != expected {
		return fmt.Errorf("%s rubric level %d label must be %q, got %q", questionLoc, rl.Level, expected, rl.Label)
	}
	if rl.Description == "" {
		return fmt.Errorf("%s rubric level %d missing description", questionLoc, rl.Level)
	}
	if rl.Score == "" {
		return fmt.Errorf("%s rubric level %d missing score", questionLoc, rl.Level)
	}
	return nil
}
