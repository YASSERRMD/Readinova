use std::collections::{BTreeMap, BTreeSet};

use crate::types::{DerivedIndices, Framework, Response, ScoringError, ScoringResult};

const ENGINE_VERSION: &str = env!("SCORING_ENGINE_VERSION");

/// Kahan compensated summation to reduce floating-point error in long sums.
pub(crate) fn kahan_sum(values: impl Iterator<Item = f64>) -> f64 {
    let mut sum = 0.0_f64;
    let mut c = 0.0_f64;
    for v in values {
        let y = v - c;
        let t = sum + y;
        c = (t - sum) - y;
        sum = t;
    }
    sum
}

/// Round a float to 4 decimal places.
#[inline]
pub(crate) fn round4(x: f64) -> f64 {
    (x * 10_000.0).round() / 10_000.0
}

/// Validate the framework definition and all responses, returning a response index.
fn validate(
    framework: &Framework,
    responses: &[Response],
) -> Result<BTreeMap<String, u8>, ScoringError> {
    // Collect all known question slugs from the framework.
    let mut known: BTreeSet<String> = BTreeSet::new();
    for dim in &framework.dimensions {
        for sd in &dim.sub_dimensions {
            for ind in &sd.indicators {
                for q in &ind.questions {
                    known.insert(q.slug.clone());
                }
            }
        }
    }

    if known.is_empty() {
        return Err(ScoringError::FrameworkInvariantViolation {
            description: "framework contains no questions".into(),
        });
    }

    // Validate dimension weight sum.
    let dim_weight_sum = kahan_sum(framework.dimensions.iter().map(|d| d.weight));
    if (round4(dim_weight_sum) - 1.0).abs() > 1e-6 {
        return Err(ScoringError::FrameworkInvariantViolation {
            description: format!(
                "dimension weights sum to {dim_weight_sum:.4}, expected 1.0000"
            ),
        });
    }

    // Validate each dimension's sub-dimension weight sum.
    for dim in &framework.dimensions {
        let sd_sum = kahan_sum(dim.sub_dimensions.iter().map(|sd| sd.weight));
        if (round4(sd_sum) - 1.0).abs() > 1e-6 {
            return Err(ScoringError::FrameworkInvariantViolation {
                description: format!(
                    "sub-dimension weights in '{}' sum to {sd_sum:.4}, expected 1.0000",
                    dim.slug
                ),
            });
        }
    }

    // Index responses by slug, checking for unknowns and invalid levels.
    let mut response_map: BTreeMap<String, u8> = BTreeMap::new();
    for r in responses {
        if !known.contains(&r.question_slug) {
            return Err(ScoringError::UnknownQuestion {
                slug: r.question_slug.clone(),
            });
        }
        if r.level > 4 {
            return Err(ScoringError::InvalidLevel {
                slug: r.question_slug.clone(),
                level: r.level,
            });
        }
        response_map.insert(r.question_slug.clone(), r.level);
    }

    // Check for missing responses.
    let missing: Vec<String> = known
        .iter()
        .filter(|slug| !response_map.contains_key(*slug))
        .cloned()
        .collect();
    if !missing.is_empty() {
        return Err(ScoringError::MissingResponses { slugs: missing });
    }

    Ok(response_map)
}

/// Compute Layer A scores.
///
/// # Algorithm
/// 1. `sub_dimension_score  = mean(question.level for q in sub_dim) * 25.0`
/// 2. `dimension_score      = Σ(sub_dimension_score × weight_within_dimension)`
/// 3. `composite_layer_a    = Σ(dimension_score × default_weight)`
///
/// # Errors
/// Returns [`ScoringError`] on validation failures.
///
/// # Panics
/// Panics if the framework contains a sub-dimension with no questions that
/// manages to pass validation (an internal invariant violation).
pub fn score(
    framework: &Framework,
    responses: &[Response],
) -> Result<ScoringResult, ScoringError> {
    let response_map = validate(framework, responses)?;

    let mut dimension_scores: BTreeMap<String, f64> = BTreeMap::new();
    let mut sub_dimension_scores: BTreeMap<String, f64> = BTreeMap::new();

    for dim in &framework.dimensions {
        let dim_score = kahan_sum(dim.sub_dimensions.iter().map(|sd| {
            let levels: Vec<f64> = sd
                .indicators
                .iter()
                .flat_map(|ind| ind.questions.iter())
                .map(|q| f64::from(*response_map.get(&q.slug).unwrap()))
                .collect();

            let n = levels.len();
            #[allow(clippy::cast_precision_loss)]
            let sd_score = if n == 0 {
                0.0
            } else {
                kahan_sum(levels.iter().copied()) / (n as f64) * 25.0
            };
            let sd_score = round4(sd_score);
            sub_dimension_scores.insert(sd.slug.clone(), sd_score);
            sd_score * sd.weight
        }));

        let dim_score = round4(dim_score);
        dimension_scores.insert(dim.slug.clone(), dim_score);
    }

    let composite_layer_a = round4(kahan_sum(
        framework
            .dimensions
            .iter()
            .map(|d| dimension_scores[&d.slug] * d.weight),
    ));

    // Binding constraint: dimension with the minimum score.
    let (bc_dim, bc_score) = dimension_scores
        .iter()
        .min_by(|a, b| a.1.partial_cmp(b.1).unwrap_or(std::cmp::Ordering::Equal))
        .map(|(k, &v)| (k.clone(), v))
        .unwrap_or_default();

    let derived = compute_derived(&dimension_scores);
    let framework_version = format!("{}-{}", framework.slug, framework.version);

    Ok(ScoringResult {
        composite_layer_a,
        dimension_scores,
        sub_dimension_scores,
        binding_constraint_dimension: bc_dim,
        binding_constraint_score: bc_score,
        derived,
        engine_version: ENGINE_VERSION.to_owned(),
        framework_version,
    })
}

/// Compute four derived indices from dimension scores.
///
/// Dimension slug prefixes used:
/// - Readiness:        `strategy_*`, `leadership_*`, `governance_risk_*`
/// - Governance/risk:  `governance_risk_*`, `security_*`, `data_governance_*`
/// - Execution:        `technical_infra*`, `ai_ml_*`, `talent_*`, `process_*`
/// - Value:            `value_*`
fn compute_derived(dim_scores: &BTreeMap<String, f64>) -> DerivedIndices {
    let mean_of = |prefixes: &[&str]| -> f64 {
        let vals: Vec<f64> = dim_scores
            .iter()
            .filter(|(k, _)| prefixes.iter().any(|p| k.starts_with(p)))
            .map(|(_, &v)| v)
            .collect();
        let n = vals.len();
        if n == 0 {
            0.0
        } else {
            #[allow(clippy::cast_precision_loss)]
            let avg = round4(kahan_sum(vals.iter().copied()) / (n as f64));
            avg
        }
    };

    DerivedIndices {
        readiness_index: mean_of(&["strategy", "leadership", "governance_risk"]),
        governance_risk_score: mean_of(&["governance_risk", "security", "data_governance"]),
        execution_capacity_score: mean_of(&["technical_infra", "ai_ml", "talent", "process"]),
        value_realisation_score: mean_of(&["value"]),
    }
}
