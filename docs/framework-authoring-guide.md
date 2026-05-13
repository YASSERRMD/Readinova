# Framework Authoring Guide

This guide explains how to author, extend, and validate AI readiness framework
dimension files for the Readinova platform.

## Overview

The AI readiness framework is stored as YAML files under
`seed/frameworks/ai-readiness-v1/`. Each file represents one dimension. The
`seedframework` CLI reads these files, validates them, and loads them into
PostgreSQL in a single published transaction.

The canonical framework has 12 dimensions summing to a total weight of 1.0000.
Target question count is ~150 across all dimensions.

---

## File Layout

```
seed/frameworks/ai-readiness-v1/
  README.md
  01-strategy.yaml
  02-leadership.yaml
  03-data-foundations.yaml
  04-data-governance.yaml
  05-technical-infrastructure.yaml
  06-ai-ml-engineering.yaml
  07-talent-skills.yaml
  08-culture-change.yaml
  09-process-workflow.yaml
  10-grc.yaml
  11-security-responsible-ai.yaml
  12-value-realisation.yaml
```

Files are loaded in lexicographic order. The numeric prefix controls
`display_order` only by convention; the YAML field `display_order` is
authoritative.

---

## Dimension File Schema

```yaml
slug: strategy_and_business_alignment     # snake_case, unique across all dimensions
name: Strategy and Business Alignment      # human-readable title
description: >                             # multi-line, wrapped at 80 chars
  Measures how well...
default_weight: "0.1000"                   # string representation of numeric(5,4)
display_order: 1                           # integer >= 1, unique per framework

sub_dimensions:
  - slug: ai_strategy_definition           # snake_case, unique within dimension
    name: AI Strategy Definition
    description: Single sentence.
    weight_within_dimension: "0.2500"      # must sum to 1.0000 within dimension
    display_order: 1

    indicators:
      - slug: strategy_document_existence
        name: Strategy Document Existence
        description: Single sentence.
        display_order: 1

        questions:
          - slug: strategy_documented_and_current
            prompt: "Question text ending with a question mark?"
            target_role: executive          # one of: executive cio risk ops any
            display_order: 1
            regulatory_references:
              nist_ai_rmf:
                - "GOVERN-1.1"
            rubric:
              - level: 0
                label: Absent
                description: "Observable description of what is absent."
                score: "0.00"
              - level: 1
                label: Ad Hoc
                description: "Observable description of an informal state."
                score: "25.00"
              - level: 2
                label: Defined
                description: "Observable description of a documented state."
                score: "50.00"
              - level: 3
                label: Managed
                description: "Observable description of a managed state."
                score: "75.00"
              - level: 4
                label: Optimised
                description: "Observable description of an optimised state."
                score: "100.00"
```

---

## Invariants

The loader validates all of the following. Violations prevent the framework
from being inserted.

| Invariant | Rule |
|-----------|------|
| Dimension weights | `SUM(default_weight)` across all dimensions = 1.0000 |
| Sub-dimension weights | `SUM(weight_within_dimension)` within each dimension = 1.0000 |
| Rubric levels | Each question must have exactly 5 rubric levels, one at each of levels 0, 1, 2, 3, 4 |
| Rubric labels | Level 0 = Absent, 1 = Ad Hoc, 2 = Defined, 3 = Managed, 4 = Optimised |
| Target roles | Must be one of: `executive`, `cio`, `risk`, `ops`, `any` |
| Scores | Must be quoted numerics: `"0.00"`, `"25.00"`, `"50.00"`, `"75.00"`, `"100.00"` |
| Weights | Must be quoted 4-decimal-place numerics: `"0.1000"`, `"0.2500"` |
| Prompts | Must end with a question mark `?` and be answerable at a single level |
| Slugs | All slugs must be lowercase snake_case |
| Display orders | Must be unique positive integers within their parent |
| Published framework | Cannot be overwritten once published |

---

## Regulatory References

Each question must include at least one regulatory reference. Use the following
keys:

| Key | Example values |
|-----|---------------|
| `nist_ai_rmf` | `GOVERN-1.1`, `MAP-1.5`, `MEASURE-2.2`, `MANAGE-4.1` |
| `eu_ai_act` | `Article 9`, `Article 10`, `Article 13`, `Article 14` |
| `iso_42001` | `5.1`, `6.1`, `8.3`, `9.1`, `10.2` |

All values are lists of strings.

**NIST AI RMF functions:**
- `GOVERN` (GV) — organisational policies and accountability
- `MAP` (MP) — context and use-case identification
- `MEASURE` (MS) — analysis and assessment
- `MANAGE` (MG) — risk prioritisation and response

---

## Writing Good Rubric Descriptions

The descriptions at each level must be **observable** and **verifiable by a
third-party assessor** without requiring the assessor to make subjective
judgements.

### Rules

**Avoid aspirational language.** Describe what *is*, not what the organisation
*wants* or *intends*.

| Aspirational (reject) | Observable (accept) |
|----------------------|---------------------|
| "Strives to maintain..." | "A documented policy exists and was reviewed within the last 12 months." |
| "Leaders are committed to..." | "At least one C-suite executive has attended two AI governance meetings in the last quarter." |
| "Best-in-class approach..." | "SLO compliance is above 99% and tracked in a published dashboard." |

**Use verifiable evidence signals.** Each description should name a specific
artefact or action that an assessor can ask to see:

- documents (policies, registers, plans)
- meetings (attendance records, minutes)
- system states (dashboard, tool output, code in a repository)
- metrics (percentages, counts, rates with thresholds)

**Use a monotone progression.** Each level must be strictly better than the
previous level. The progression should be:
- **0 (Absent):** The capability does not exist.
- **1 (Ad Hoc):** Activity exists but is undocumented, inconsistent, or informal.
- **2 (Defined):** A documented standard or process exists and is followed for most cases.
- **3 (Managed):** The process is followed consistently for all cases, measured, and reviewed.
- **4 (Optimised):** The process is measured, published, compared to targets, and drives improvement.

---

## Writing Good Questions

Each question must be **answerable at a single maturity level** without
requiring the assessor to answer multiple sub-questions.

| Rule | Example |
|------|---------|
| End with `?` | `"Does the organisation have a documented AI strategy?"` |
| Single-part | Do not combine two conditions with "and" unless both are required for the level |
| Direct | Ask what exists now, not what is planned |
| Unambiguous | Avoid "sufficient", "appropriate", "adequate" — quantify instead |

**Allowed target roles:** `executive`, `cio`, `risk`, `ops`, `any`

Assign the role whose perspective is most relevant to answering the question
accurately. Do not assign `any` unless the question is equally answerable by
all roles.

---

## Running the Loader

### Validate only (no database writes)

```bash
go run ./apps/api/cmd/seedframework \
  -seed-dir seed/frameworks/ai-readiness-v1 \
  -dry-run
```

### Load into database

```bash
export READINOVA_DATABASE_URL="postgres://user:pass@host:5432/readinova"

go run ./apps/api/cmd/seedframework \
  -seed-dir seed/frameworks/ai-readiness-v1
```

The command is idempotent: if the framework already exists as a draft it
returns without error. If it is already published, it returns an error.

### Run integration tests

```bash
export READINOVA_DATABASE_URL="postgres://user:pass@host:5432/readinova_test"

go test ./apps/api/internal/seed/... -v -run TestSeed
```

Tests require a real PostgreSQL database. They apply all migrations to a
clean schema before each test, so the test database can be shared.

---

## Adding a New Dimension

1. Create `seed/frameworks/ai-readiness-v1/NN-slug.yaml` using the schema
   above.
2. Set `default_weight` so that the sum of all 12 dimension weights remains
   1.0000. Reduce one or more existing dimensions to compensate.
3. Run the validator: `go run ./apps/api/cmd/seedframework -dry-run`
4. Verify the question count and weight summary in the output.

**A new dimension requires a new framework version** (increment
`version_major`) if the existing framework is already published, because
published frameworks are immutable.

---

## Versioning

| Change type | Version action |
|-------------|---------------|
| New framework from scratch | `version_major = 1, version_minor = 0` |
| Minor wording fix (pre-publish) | Edit in place; no version change |
| Structural change (post-publish) | Increment `version_major` |
| Weight recalibration (post-publish) | Increment `version_major` |
| New regulatory reference added | Increment `version_minor` |

The slug uniqueness constraint is `(slug, version_major)`, so
`ai-readiness-v1` with `version_major = 2` is a distinct framework from
`version_major = 1`.
