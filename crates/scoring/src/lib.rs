#![warn(clippy::pedantic)]
#![allow(clippy::missing_errors_doc, clippy::module_name_repetitions)]

mod engine;
pub mod ffi;
pub mod types;

pub use engine::score;
pub use types::{
    DerivedIndices, DimensionDef, Framework, IndicatorDef, QuestionDef, Response, ScoringError,
    ScoringResult, SubDimensionDef,
};

#[cfg(test)]
mod tests;
