package db_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestFrameworkMigrationsForwardAndBackward(t *testing.T) {
	db := openDatabase(t)
	resetSchema(t, db)

	if err := goose.Up(db, migrationDir(t)); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	var tableCount int
	err := db.QueryRow(`
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name IN (
			'frameworks',
			'dimensions',
			'sub_dimensions',
			'indicators',
			'questions',
			'rubric_levels',
			'industry_weight_overrides'
		  )
	`).Scan(&tableCount)
	if err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tableCount != 7 {
		t.Fatalf("expected 7 framework tables, got %d", tableCount)
	}

	if err := goose.DownTo(db, migrationDir(t), 0); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
}

func TestFrameworkInvariantTriggersRejectBadInserts(t *testing.T) {
	tests := map[string]func(*testing.T, *sql.DB){
		"dimension weights must sum to one":     insertBadDimensionWeights,
		"rubric levels must cover zero to four": insertBadRubricCoverage,
	}

	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			db := openMigratedDatabase(t)
			run(t, db)
		})
	}
}

func TestPublishedFrameworkDefinitionIsImmutable(t *testing.T) {
	db := openMigratedDatabase(t)
	frameworkID := insertValidFramework(t, db, true)

	_, err := db.Exec(`
		UPDATE dimensions
		SET name = 'Changed'
		WHERE framework_id = $1
	`, frameworkID)
	if err == nil {
		t.Fatal("expected published framework dimension update to fail")
	}
}

func openMigratedDatabase(t *testing.T) *sql.DB {
	t.Helper()

	db := openDatabase(t)
	resetSchema(t, db)
	if err := goose.Up(db, migrationDir(t)); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	return db
}

func openDatabase(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("READINOVA_DATABASE_URL")
	if dsn == "" {
		t.Skip("READINOVA_DATABASE_URL is required for PostgreSQL integration tests")
	}

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := db.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	return db
}

func resetSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	mustExec(t, db, "DROP SCHEMA public CASCADE")
	mustExec(t, db, "CREATE SCHEMA public")
}

func migrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../migrations"))
}

func insertBadDimensionWeights(t *testing.T, db *sql.DB) {
	t.Helper()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var frameworkID string
	err = tx.QueryRow(`
		INSERT INTO frameworks (slug, name, version_major, version_minor)
		VALUES ('bad_weights', 'Bad Weights', 1, 0)
		RETURNING id
	`).Scan(&frameworkID)
	if err != nil {
		t.Fatalf("insert framework: %v", err)
	}

	_, err = tx.Exec(`
		INSERT INTO dimensions (framework_id, slug, name, description, default_weight, display_order)
		VALUES ($1, 'strategy', 'Strategy', 'Strategy dimension', 0.9000, 1)
	`, frameworkID)
	if err != nil {
		t.Fatalf("insert dimension: %v", err)
	}

	if err := tx.Commit(); err == nil {
		t.Fatal("expected dimension weight invariant to reject commit")
	}
}

func insertBadRubricCoverage(t *testing.T, db *sql.DB) {
	t.Helper()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	questionID := insertQuestionTree(t, tx, "bad_rubric", 1.0000, 1.0000)

	for level := 0; level < 4; level++ {
		_, err = tx.Exec(`
			INSERT INTO rubric_levels (question_id, level, label, description, score)
			VALUES ($1, $2, $3, $4, $5)
		`, questionID, level, rubricLabel(level), fmt.Sprintf("Level %d description", level), level*25)
		if err != nil {
			t.Fatalf("insert rubric level %d: %v", level, err)
		}
	}

	if err := tx.Commit(); err == nil {
		t.Fatal("expected rubric coverage invariant to reject commit")
	}
}

func insertValidFramework(t *testing.T, db *sql.DB, publish bool) string {
	t.Helper()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	questionID := insertQuestionTree(t, tx, "valid_framework", 1.0000, 1.0000)
	for level := 0; level <= 4; level++ {
		_, err = tx.Exec(`
			INSERT INTO rubric_levels (question_id, level, label, description, score)
			VALUES ($1, $2, $3, $4, $5)
		`, questionID, level, rubricLabel(level), fmt.Sprintf("Level %d description", level), level*25)
		if err != nil {
			t.Fatalf("insert rubric level %d: %v", level, err)
		}
	}

	var frameworkID string
	err = tx.QueryRow(`
		SELECT f.id
		FROM frameworks f
		JOIN dimensions d ON d.framework_id = f.id
		JOIN sub_dimensions sd ON sd.dimension_id = d.id
		JOIN indicators i ON i.sub_dimension_id = sd.id
		JOIN questions q ON q.indicator_id = i.id
		WHERE q.id = $1
	`, questionID).Scan(&frameworkID)
	if err != nil {
		t.Fatalf("lookup framework id: %v", err)
	}

	if publish {
		_, err = tx.Exec(`
			UPDATE frameworks
			SET status = 'published', published_at = now()
			WHERE id = $1
		`, frameworkID)
		if err != nil {
			t.Fatalf("publish framework: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit valid framework: %v", err)
	}

	return frameworkID
}

func insertQuestionTree(t *testing.T, tx *sql.Tx, slug string, dimensionWeight float64, subDimensionWeight float64) string {
	t.Helper()

	var frameworkID string
	err := tx.QueryRow(`
		INSERT INTO frameworks (slug, name, version_major, version_minor)
		VALUES ($1, $2, 1, 0)
		RETURNING id
	`, slug, slug).Scan(&frameworkID)
	if err != nil {
		t.Fatalf("insert framework: %v", err)
	}

	var dimensionID string
	err = tx.QueryRow(`
		INSERT INTO dimensions (framework_id, slug, name, description, default_weight, display_order)
		VALUES ($1, 'strategy', 'Strategy', 'Strategy dimension', $2, 1)
		RETURNING id
	`, frameworkID, dimensionWeight).Scan(&dimensionID)
	if err != nil {
		t.Fatalf("insert dimension: %v", err)
	}

	var subDimensionID string
	err = tx.QueryRow(`
		INSERT INTO sub_dimensions (dimension_id, slug, name, description, weight_within_dimension, display_order)
		VALUES ($1, 'portfolio', 'Portfolio', 'Portfolio sub-dimension', $2, 1)
		RETURNING id
	`, dimensionID, subDimensionWeight).Scan(&subDimensionID)
	if err != nil {
		t.Fatalf("insert sub_dimension: %v", err)
	}

	var indicatorID string
	err = tx.QueryRow(`
		INSERT INTO indicators (sub_dimension_id, slug, name, description, display_order)
		VALUES ($1, 'backlog', 'Backlog', 'Use-case backlog indicator', 1)
		RETURNING id
	`, subDimensionID).Scan(&indicatorID)
	if err != nil {
		t.Fatalf("insert indicator: %v", err)
	}

	var questionID string
	err = tx.QueryRow(`
		INSERT INTO questions (indicator_id, slug, prompt, target_role, regulatory_references, display_order)
		VALUES ($1, 'backlog_exists', 'Does a ranked backlog exist?', 'executive', '{"nist_ai_rmf":["MAP-1.1"]}'::jsonb, 1)
		RETURNING id
	`, indicatorID).Scan(&questionID)
	if err != nil {
		t.Fatalf("insert question: %v", err)
	}

	return questionID
}

func rubricLabel(level int) string {
	switch level {
	case 0:
		return "Absent"
	case 1:
		return "Ad Hoc"
	case 2:
		return "Defined"
	case 3:
		return "Managed"
	case 4:
		return "Optimised"
	default:
		panic("invalid rubric level")
	}
}

func mustExec(t *testing.T, db *sql.DB, statement string) {
	t.Helper()

	if _, err := db.Exec(statement); err != nil {
		t.Fatalf("exec %q: %v", statement, err)
	}
}
