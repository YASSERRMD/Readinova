use crate::{
    engine::kahan_sum,
    score,
    types::{
        DimensionDef, Framework, IndicatorDef, QuestionDef, Response, ScoringError,
        SubDimensionDef,
    },
};

/// Assert two f64 values are within 1e-9 of each other.
macro_rules! assert_f64_eq {
    ($left:expr, $right:expr) => {
        let diff = ($left - $right).abs();
        assert!(diff < 1e-9, "assertion failed: {} ≈ {} (diff = {})", $left, $right, diff);
    };
}

// ── helpers ──────────────────────────────────────────────────────────────────

/// Build a minimal single-dimension framework with `n_questions` questions in
/// a single sub-dimension and indicator.
fn single_dim_framework(n_questions: usize) -> Framework {
    let questions: Vec<QuestionDef> = (0..n_questions)
        .map(|i| QuestionDef {
            slug: format!("q{i}"),
        })
        .collect();
    Framework {
        slug: "test-fw".into(),
        version: "1.0".into(),
        dimensions: vec![DimensionDef {
            slug: "dim_a".into(),
            weight: 1.0,
            sub_dimensions: vec![SubDimensionDef {
                slug: "sd_a".into(),
                weight: 1.0,
                indicators: vec![IndicatorDef {
                    slug: "ind_a".into(),
                    questions,
                }],
            }],
        }],
    }
}

/// Build responses where every question gets the same level.
fn uniform_responses(fw: &Framework, level: u8) -> Vec<Response> {
    fw.dimensions
        .iter()
        .flat_map(|d| d.sub_dimensions.iter())
        .flat_map(|sd| sd.indicators.iter())
        .flat_map(|ind| ind.questions.iter())
        .map(|q| Response {
            question_slug: q.slug.clone(),
            level,
        })
        .collect()
}

/// Minimal two-dimension framework for composite tests.
fn two_dim_framework() -> Framework {
    Framework {
        slug: "two-dim".into(),
        version: "1.0".into(),
        dimensions: vec![
            DimensionDef {
                slug: "dim_x".into(),
                weight: 0.6,
                sub_dimensions: vec![SubDimensionDef {
                    slug: "sd_x".into(),
                    weight: 1.0,
                    indicators: vec![IndicatorDef {
                        slug: "ind_x".into(),
                        questions: vec![QuestionDef { slug: "q_x".into() }],
                    }],
                }],
            },
            DimensionDef {
                slug: "dim_y".into(),
                weight: 0.4,
                sub_dimensions: vec![SubDimensionDef {
                    slug: "sd_y".into(),
                    weight: 1.0,
                    indicators: vec![IndicatorDef {
                        slug: "ind_y".into(),
                        questions: vec![QuestionDef { slug: "q_y".into() }],
                    }],
                }],
            },
        ],
    }
}

// ── unit tests ────────────────────────────────────────────────────────────────

#[test]
fn kahan_sum_integer_values() {
    let vals = [1.0_f64, 2.0, 3.0, 4.0];
    assert_f64_eq!(kahan_sum(vals.iter().copied()), 10.0);
}

#[test]
fn kahan_sum_empty() {
    assert_f64_eq!(kahan_sum(std::iter::empty()), 0.0);
}

#[test]
fn all_zero_responses_produce_composite_zero() {
    let fw = single_dim_framework(4);
    let responses = uniform_responses(&fw, 0);
    let result = score(&fw, &responses).unwrap();
    assert_f64_eq!(result.composite_layer_a, 0.0);
}

#[test]
fn all_max_responses_produce_composite_100() {
    let fw = single_dim_framework(4);
    let responses = uniform_responses(&fw, 4);
    let result = score(&fw, &responses).unwrap();
    assert_f64_eq!(result.composite_layer_a, 100.0);
}

#[test]
fn level_2_mid_score() {
    // level 2 → 2/4 * 100 = 50.0
    let fw = single_dim_framework(1);
    let responses = uniform_responses(&fw, 2);
    let result = score(&fw, &responses).unwrap();
    assert_f64_eq!(result.composite_layer_a, 50.0);
}

#[test]
fn composite_is_weighted_average_of_dimensions() {
    let fw = two_dim_framework();
    // q_x = level 4 → dim_x = 100, weighted 0.6
    // q_y = level 0 → dim_y = 0, weighted 0.4
    // composite = 100*0.6 + 0*0.4 = 60.0
    let responses = vec![
        Response { question_slug: "q_x".into(), level: 4 },
        Response { question_slug: "q_y".into(), level: 0 },
    ];
    let result = score(&fw, &responses).unwrap();
    assert_f64_eq!(result.composite_layer_a, 60.0);
}

#[test]
fn binding_constraint_is_lowest_dimension() {
    let fw = two_dim_framework();
    let responses = vec![
        Response { question_slug: "q_x".into(), level: 4 },
        Response { question_slug: "q_y".into(), level: 1 },
    ];
    let result = score(&fw, &responses).unwrap();
    assert_eq!(result.binding_constraint_dimension, "dim_y");
    assert_f64_eq!(result.binding_constraint_score, 25.0);
}

#[test]
fn missing_response_returns_error() {
    let fw = single_dim_framework(2);
    let partial = vec![Response { question_slug: "q0".into(), level: 2 }];
    let err = score(&fw, &partial).unwrap_err();
    match err {
        ScoringError::MissingResponses { slugs } => assert!(slugs.contains(&"q1".to_owned())),
        _ => panic!("expected MissingResponses, got {err:?}"),
    }
}

#[test]
fn unknown_question_returns_error() {
    let fw = single_dim_framework(1);
    let responses = vec![
        Response { question_slug: "q0".into(), level: 1 },
        Response { question_slug: "ghost".into(), level: 1 },
    ];
    let err = score(&fw, &responses).unwrap_err();
    assert!(matches!(err, ScoringError::UnknownQuestion { slug } if slug == "ghost"));
}

#[test]
fn invalid_level_returns_error() {
    let fw = single_dim_framework(1);
    let responses = vec![Response { question_slug: "q0".into(), level: 5 }];
    let err = score(&fw, &responses).unwrap_err();
    assert!(matches!(err, ScoringError::InvalidLevel { level: 5, .. }));
}

#[test]
fn idempotent_on_same_inputs() {
    let fw = single_dim_framework(3);
    let responses = uniform_responses(&fw, 3);
    let r1 = score(&fw, &responses).unwrap();
    let r2 = score(&fw, &responses).unwrap();
    assert_eq!(
        serde_json::to_string(&r1).unwrap(),
        serde_json::to_string(&r2).unwrap()
    );
}

#[test]
fn engine_version_non_empty() {
    let fw = single_dim_framework(1);
    let responses = uniform_responses(&fw, 2);
    let result = score(&fw, &responses).unwrap();
    assert!(!result.engine_version.is_empty());
}

#[test]
fn framework_invariant_bad_dim_weights() {
    let mut fw = single_dim_framework(1);
    fw.dimensions[0].weight = 0.5;
    let responses = uniform_responses(&fw, 2);
    let err = score(&fw, &responses).unwrap_err();
    assert!(matches!(err, ScoringError::FrameworkInvariantViolation { .. }));
}

#[test]
fn sub_dimension_weights_validated() {
    let fw = Framework {
        slug: "bad".into(),
        version: "1.0".into(),
        dimensions: vec![DimensionDef {
            slug: "dim_a".into(),
            weight: 1.0,
            sub_dimensions: vec![
                SubDimensionDef {
                    slug: "sd_a".into(),
                    weight: 0.3,
                    indicators: vec![IndicatorDef {
                        slug: "ind_a".into(),
                        questions: vec![QuestionDef { slug: "qa".into() }],
                    }],
                },
                SubDimensionDef {
                    slug: "sd_b".into(),
                    weight: 0.3,
                    indicators: vec![IndicatorDef {
                        slug: "ind_b".into(),
                        questions: vec![QuestionDef { slug: "qb".into() }],
                    }],
                },
            ],
        }],
    };
    let responses = vec![
        Response { question_slug: "qa".into(), level: 2 },
        Response { question_slug: "qb".into(), level: 2 },
    ];
    let err = score(&fw, &responses).unwrap_err();
    assert!(matches!(err, ScoringError::FrameworkInvariantViolation { .. }));
}

#[test]
fn monotonicity_manual() {
    let fw = two_dim_framework();
    let low = vec![
        Response { question_slug: "q_x".into(), level: 2 },
        Response { question_slug: "q_y".into(), level: 2 },
    ];
    let high = vec![
        Response { question_slug: "q_x".into(), level: 3 },
        Response { question_slug: "q_y".into(), level: 2 },
    ];
    let r_low = score(&fw, &low).unwrap();
    let r_high = score(&fw, &high).unwrap();
    assert!(r_high.composite_layer_a >= r_low.composite_layer_a);
}

// ── property tests ────────────────────────────────────────────────────────────

#[cfg(test)]
mod proptests {
    use proptest::prelude::*;

    use super::*;
    use crate::{score, types::*};

    fn level_strategy() -> impl Strategy<Value = u8> {
        0u8..=4u8
    }

    fn framework_4q() -> Framework {
        single_dim_framework(4)
    }

    proptest! {
        #[test]
        fn prop_boundary_zero(_level in 0u8..=0u8) {
            let fw = framework_4q();
            let rs = uniform_responses(&fw, 0);
            let result = score(&fw, &rs).unwrap();
            prop_assert!(result.composite_layer_a.abs() < 1e-9);
        }

        #[test]
        fn prop_boundary_max(_level in 4u8..=4u8) {
            let fw = framework_4q();
            let rs = uniform_responses(&fw, 4);
            let result = score(&fw, &rs).unwrap();
            prop_assert!((result.composite_layer_a - 100.0).abs() < 1e-9);
        }

        #[test]
        fn prop_idempotence(
            l0 in level_strategy(), l1 in level_strategy(),
            l2 in level_strategy(), l3 in level_strategy()
        ) {
            let fw = framework_4q();
            let rs = vec![
                Response { question_slug: "q0".into(), level: l0 },
                Response { question_slug: "q1".into(), level: l1 },
                Response { question_slug: "q2".into(), level: l2 },
                Response { question_slug: "q3".into(), level: l3 },
            ];
            let r1 = score(&fw, &rs).unwrap();
            let r2 = score(&fw, &rs).unwrap();
            let j1 = serde_json::to_string(&r1).unwrap();
            let j2 = serde_json::to_string(&r2).unwrap();
            prop_assert_eq!(j1, j2);
        }

        #[test]
        fn prop_monotonicity(
            base in 0u8..4u8,
            raised_idx in 0usize..4usize,
        ) {
            let fw = framework_4q();
            let low_levels = vec![base; 4];
            let mut high_levels = low_levels.clone();
            high_levels[raised_idx] = base + 1;

            let slugs = ["q0", "q1", "q2", "q3"];
            let mk = |levels: &[u8]| {
                levels.iter().enumerate().map(|(i, &l)| Response {
                    question_slug: slugs[i].into(),
                    level: l,
                }).collect::<Vec<_>>()
            };

            let r_low = score(&fw, &mk(&low_levels)).unwrap();
            let r_high = score(&fw, &mk(&high_levels)).unwrap();
            prop_assert!(r_high.composite_layer_a >= r_low.composite_layer_a);
        }

        #[test]
        fn prop_score_in_range(
            l0 in level_strategy(), l1 in level_strategy(),
            l2 in level_strategy(), l3 in level_strategy()
        ) {
            let fw = framework_4q();
            let rs = vec![
                Response { question_slug: "q0".into(), level: l0 },
                Response { question_slug: "q1".into(), level: l1 },
                Response { question_slug: "q2".into(), level: l2 },
                Response { question_slug: "q3".into(), level: l3 },
            ];
            let result = score(&fw, &rs).unwrap();
            prop_assert!(result.composite_layer_a >= 0.0);
            prop_assert!(result.composite_layer_a <= 100.0);
        }
    }
}
