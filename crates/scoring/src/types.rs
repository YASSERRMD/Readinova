use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

/// A question descriptor used by the scoring engine.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct QuestionDef {
    pub slug: String,
}

/// An indicator (group of questions) within a sub-dimension.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IndicatorDef {
    pub slug: String,
    pub questions: Vec<QuestionDef>,
}

/// A sub-dimension within a dimension.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SubDimensionDef {
    pub slug: String,
    /// Weight within the parent dimension; must sum to `1.0` across siblings.
    pub weight: f64,
    pub indicators: Vec<IndicatorDef>,
}

/// A top-level dimension in the framework.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DimensionDef {
    pub slug: String,
    /// Fractional weight of this dimension; must sum to `1.0` across all dimensions.
    pub weight: f64,
    pub sub_dimensions: Vec<SubDimensionDef>,
}

/// Minimal framework definition consumed by the scoring engine.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Framework {
    pub slug: String,
    pub version: String,
    pub dimensions: Vec<DimensionDef>,
}

/// A single response from an assessor.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Response {
    pub question_slug: String,
    /// Maturity level: `0` = Absent … `4` = Optimised.
    pub level: u8,
}

/// Four derived composite indices computed from subsets of dimension scores.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DerivedIndices {
    /// Mean of `strategy_*`, `leadership_*`, `governance_risk_*` dimensions.
    pub readiness_index: f64,
    /// Mean of `governance_risk_*`, `security_*`, `data_governance_*` dimensions.
    pub governance_risk_score: f64,
    /// Mean of `technical_infra*`, `ai_ml_*`, `talent_*`, `process_*` dimensions.
    pub execution_capacity_score: f64,
    /// `value_*` dimension score.
    pub value_realisation_score: f64,
}

/// The full result returned by [`crate::score`].
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ScoringResult {
    /// Overall Layer A composite score (`0.0` – `100.0`).
    pub composite_layer_a: f64,
    /// Score per dimension slug.
    pub dimension_scores: BTreeMap<String, f64>,
    /// Score per sub-dimension slug.
    pub sub_dimension_scores: BTreeMap<String, f64>,
    /// Slug of the dimension with the lowest score.
    pub binding_constraint_dimension: String,
    /// Score of that dimension.
    pub binding_constraint_score: f64,
    pub derived: DerivedIndices,
    /// Compile-time git SHA of the engine.
    pub engine_version: String,
    /// Framework `slug-version` baked into the result.
    pub framework_version: String,
}

/// Errors returned by [`crate::score`].
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind")]
pub enum ScoringError {
    /// One or more required question slugs have no response.
    MissingResponses { slugs: Vec<String> },
    /// A response references a question slug not in the framework.
    UnknownQuestion { slug: String },
    /// A response level is outside `0..=4`.
    InvalidLevel { slug: String, level: u8 },
    /// The framework definition itself is malformed.
    FrameworkInvariantViolation { description: String },
}

impl std::fmt::Display for ScoringError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::MissingResponses { slugs } => {
                write!(f, "missing responses for: {}", slugs.join(", "))
            }
            Self::UnknownQuestion { slug } => write!(f, "unknown question slug: {slug}"),
            Self::InvalidLevel { slug, level } => {
                write!(f, "invalid level {level} for question {slug}")
            }
            Self::FrameworkInvariantViolation { description } => {
                write!(f, "framework invariant violation: {description}")
            }
        }
    }
}

impl std::error::Error for ScoringError {}
