package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

// LoadDir reads all *.yaml files from dir in lexicographic order and returns
// a slice of validated Dimension values.
func LoadDir(dir string) ([]Dimension, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read seed dir %q: %w", dir, err)
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)

	if len(paths) == 0 {
		return nil, fmt.Errorf("no .yaml files found in %q", dir)
	}

	dims := make([]Dimension, 0, len(paths))
	for _, p := range paths {
		d, err := loadDimension(p)
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", p, err)
		}
		dims = append(dims, d)
	}

	return dims, nil
}

func loadDimension(path string) (Dimension, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Dimension{}, err
	}

	var d Dimension
	if err := yaml.Unmarshal(data, &d); err != nil {
		return Dimension{}, fmt.Errorf("parse yaml: %w", err)
	}

	if err := d.Validate(); err != nil {
		return Dimension{}, err
	}

	return d, nil
}

// InsertResult summarises what the loader inserted.
type InsertResult struct {
	FrameworkID    string
	Dimensions     int
	SubDimensions  int
	Indicators     int
	Questions      int
	RubricLevels   int
}

// Insert writes the given dimensions into the database under a single framework
// record. The operation runs in a single transaction.
//
// If a framework with slug and version_major already exists:
//   - and its status is "draft", the function returns ErrFrameworkExists.
//   - and its status is "published" or "archived", the function returns ErrFrameworkPublished.
//
// This makes the command idempotent when re-run on a clean database (the first
// run succeeds; subsequent runs return ErrFrameworkExists rather than silently
// inserting duplicates).
var ErrFrameworkExists = fmt.Errorf("framework already exists in the database (status: draft)")
var ErrFrameworkPublished = fmt.Errorf("framework is published or archived and cannot be overwritten")

// Insert inserts the framework and all its dimensions into the database.
func Insert(ctx context.Context, conn *pgx.Conn, fw Framework, dims []Dimension) (*InsertResult, error) {
	// Check for existing framework.
	var existingStatus string
	err := conn.QueryRow(ctx,
		`SELECT status FROM frameworks WHERE slug = $1 AND version_major = $2`,
		fw.Slug, fw.VersionMajor,
	).Scan(&existingStatus)
	if err == nil {
		// Framework exists.
		if existingStatus == "published" || existingStatus == "archived" {
			return nil, ErrFrameworkPublished
		}
		return nil, ErrFrameworkExists
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("check existing framework: %w", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result := &InsertResult{}

	// Insert framework.
	var frameworkID string
	err = tx.QueryRow(ctx, `
		INSERT INTO frameworks (slug, name, version_major, version_minor)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, fw.Slug, fw.Name, fw.VersionMajor, fw.VersionMinor).Scan(&frameworkID)
	if err != nil {
		return nil, fmt.Errorf("insert framework: %w", err)
	}
	result.FrameworkID = frameworkID

	for _, dim := range dims {
		if err := insertDimension(ctx, tx, frameworkID, dim, result); err != nil {
			return nil, err
		}
	}

	// Publish the framework.
	_, err = tx.Exec(ctx, `
		UPDATE frameworks
		SET status = 'published', published_at = now()
		WHERE id = $1
	`, frameworkID)
	if err != nil {
		return nil, fmt.Errorf("publish framework: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return result, nil
}

func insertDimension(ctx context.Context, tx pgx.Tx, frameworkID string, dim Dimension, res *InsertResult) error {
	var dimID string
	err := tx.QueryRow(ctx, `
		INSERT INTO dimensions (framework_id, slug, name, description, default_weight, display_order)
		VALUES ($1, $2, $3, $4, $5::numeric, $6)
		RETURNING id
	`, frameworkID, dim.Slug, dim.Name, dim.Description, dim.DefaultWeight, dim.DisplayOrder).Scan(&dimID)
	if err != nil {
		return fmt.Errorf("insert dimension %q: %w", dim.Slug, err)
	}
	res.Dimensions++

	for _, sd := range dim.SubDimensions {
		if err := insertSubDimension(ctx, tx, dimID, sd, res); err != nil {
			return err
		}
	}
	return nil
}

func insertSubDimension(ctx context.Context, tx pgx.Tx, dimID string, sd SubDimension, res *InsertResult) error {
	var sdID string
	err := tx.QueryRow(ctx, `
		INSERT INTO sub_dimensions (dimension_id, slug, name, description, weight_within_dimension, display_order)
		VALUES ($1, $2, $3, $4, $5::numeric, $6)
		RETURNING id
	`, dimID, sd.Slug, sd.Name, sd.Description, sd.WeightWithinDimension, sd.DisplayOrder).Scan(&sdID)
	if err != nil {
		return fmt.Errorf("insert sub_dimension %q: %w", sd.Slug, err)
	}
	res.SubDimensions++

	for _, ind := range sd.Indicators {
		if err := insertIndicator(ctx, tx, sdID, ind, res); err != nil {
			return err
		}
	}
	return nil
}

func insertIndicator(ctx context.Context, tx pgx.Tx, sdID string, ind Indicator, res *InsertResult) error {
	var indID string
	err := tx.QueryRow(ctx, `
		INSERT INTO indicators (sub_dimension_id, slug, name, description, display_order)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, sdID, ind.Slug, ind.Name, ind.Description, ind.DisplayOrder).Scan(&indID)
	if err != nil {
		return fmt.Errorf("insert indicator %q: %w", ind.Slug, err)
	}
	res.Indicators++

	for _, q := range ind.Questions {
		if err := insertQuestion(ctx, tx, indID, q, res); err != nil {
			return err
		}
	}
	return nil
}

func insertQuestion(ctx context.Context, tx pgx.Tx, indID string, q Question, res *InsertResult) error {
	regRefs, err := json.Marshal(q.RegulatoryReferences)
	if err != nil {
		return fmt.Errorf("marshal regulatory_references for question %q: %w", q.Slug, err)
	}

	var qID string
	err = tx.QueryRow(ctx, `
		INSERT INTO questions (indicator_id, slug, prompt, target_role, regulatory_references, display_order)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
		RETURNING id
	`, indID, q.Slug, q.Prompt, q.TargetRole, string(regRefs), q.DisplayOrder).Scan(&qID)
	if err != nil {
		return fmt.Errorf("insert question %q: %w", q.Slug, err)
	}
	res.Questions++

	for _, rl := range q.Rubric {
		_, err = tx.Exec(ctx, `
			INSERT INTO rubric_levels (question_id, level, label, description, score)
			VALUES ($1, $2, $3, $4, $5::numeric)
		`, qID, rl.Level, rl.Label, rl.Description, rl.Score)
		if err != nil {
			return fmt.Errorf("insert rubric_level %d for question %q: %w", rl.Level, q.Slug, err)
		}
		res.RubricLevels++
	}
	return nil
}
