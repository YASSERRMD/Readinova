// Command seedframework loads the AI readiness framework YAML seed files into
// the database. It reads dimension files from a seed directory, validates them,
// and inserts the full framework in a single transaction.
//
// Usage:
//
//	seedframework [flags]
//
// Flags:
//
//	-dsn string       PostgreSQL DSN (default: $READINOVA_DATABASE_URL)
//	-seed-dir string  path to the seed directory (default: seed/frameworks/ai-readiness-v1)
//	-dry-run          validate YAML and print summary without writing to the database
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"

	"github.com/YASSERRMD/Readinova/apps/api/internal/seed"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seedframework failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	dsn := flag.String("dsn", os.Getenv("READINOVA_DATABASE_URL"), "PostgreSQL DSN")
	seedDir := flag.String("seed-dir", "seed/frameworks/ai-readiness-v1", "path to YAML seed directory")
	dryRun := flag.Bool("dry-run", false, "validate YAML and print summary without writing to the database")
	flag.Parse()

	slog.Info("loading seed files", "dir", *seedDir)

	dims, err := seed.LoadDir(*seedDir)
	if err != nil {
		return fmt.Errorf("load seed dir: %w", err)
	}

	// Count questions for the summary.
	totalQuestions := 0
	for _, d := range dims {
		for _, sd := range d.SubDimensions {
			for _, ind := range sd.Indicators {
				totalQuestions += len(ind.Questions)
			}
		}
	}

	slog.Info("seed files loaded and validated",
		"dimensions", len(dims),
		"questions", totalQuestions,
	)

	if *dryRun {
		printSummary(dims)
		slog.Info("dry-run complete, no database changes made")
		return nil
	}

	if *dsn == "" {
		return fmt.Errorf("database DSN is required: set -dsn flag or READINOVA_DATABASE_URL environment variable")
	}

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, *dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	fw := seed.Framework{
		Slug:         "ai-readiness-v1",
		Name:         "AI Readiness Framework v1",
		VersionMajor: 1,
		VersionMinor: 0,
		Dimensions:   dims,
	}

	result, err := seed.Insert(ctx, conn, fw, dims)
	if err != nil {
		if errors.Is(err, seed.ErrFrameworkPublished) {
			return fmt.Errorf("cannot overwrite a published framework: %w", err)
		}
		if errors.Is(err, seed.ErrFrameworkExists) {
			slog.Warn("framework already exists as draft, skipping insertion (idempotent)")
			return nil
		}
		return fmt.Errorf("insert framework: %w", err)
	}

	slog.Info("framework inserted and published",
		"framework_id", result.FrameworkID,
		"dimensions", result.Dimensions,
		"sub_dimensions", result.SubDimensions,
		"indicators", result.Indicators,
		"questions", result.Questions,
		"rubric_levels", result.RubricLevels,
	)

	return nil
}

func printSummary(dims []Dimension) {
	fmt.Printf("\n%-5s  %-45s  %s\n", "Order", "Dimension", "Weight")
	fmt.Printf("%-5s  %-45s  %s\n", "-----", "---------", "------")
	for _, d := range dims {
		qCount := 0
		for _, sd := range d.SubDimensions {
			for _, ind := range sd.Indicators {
				qCount += len(ind.Questions)
			}
		}
		fmt.Printf("%-5d  %-45s  %s  (%d questions, %d sub-dims)\n",
			d.DisplayOrder, d.Name, d.DefaultWeight, qCount, len(d.SubDimensions))
	}
	fmt.Println()
}

// Dimension is a local alias to avoid an import cycle when printing from main.
type Dimension = seed.Dimension
