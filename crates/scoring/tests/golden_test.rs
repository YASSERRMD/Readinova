//! Golden-file regression test.
//!
//! The first time this runs it generates the expected output file.
//! Subsequent runs compare against it. Any algorithmic change will break this
//! test until `REGEN_GOLDEN=1` is set to regenerate the file.

use std::path::Path;

use scoring::{score, types::*};

#[derive(serde::Deserialize)]
struct Fixture {
    framework: Framework,
    responses: Vec<Response>,
}

fn load_fixture() -> Fixture {
    let path = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/golden/fixture.json");
    let content = std::fs::read_to_string(path).expect("fixture.json not found");
    serde_json::from_str(&content).expect("invalid fixture.json")
}

#[test]
fn golden_file_regression() {
    let fixture = load_fixture();
    let result = score(&fixture.framework, &fixture.responses)
        .expect("scoring failed on golden fixture");

    // Serialise with sorted keys for byte-stable comparison.
    let actual_json = serde_json::to_string_pretty(&result).unwrap();

    let golden_path =
        Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/golden/expected_output.json");

    if std::env::var("REGEN_GOLDEN").is_ok() || !golden_path.exists() {
        std::fs::write(&golden_path, &actual_json).expect("write golden file");
        eprintln!("Golden file written to {}", golden_path.display());
        return;
    }

    let expected = std::fs::read_to_string(&golden_path).expect("expected_output.json not found");

    // Strip engine_version before comparing — it's git-SHA-dependent.
    let strip_engine = |s: &str| -> String {
        let mut v: serde_json::Value = serde_json::from_str(s).unwrap();
        if let Some(obj) = v.as_object_mut() {
            obj.remove("engine_version");
        }
        serde_json::to_string_pretty(&v).unwrap()
    };

    assert_eq!(
        strip_engine(&actual_json),
        strip_engine(&expected),
        "Golden file mismatch — run with REGEN_GOLDEN=1 to regenerate"
    );
}

#[test]
fn idempotent_with_golden_fixture() {
    let fixture = load_fixture();
    let r1 = score(&fixture.framework, &fixture.responses).unwrap();
    let r2 = score(&fixture.framework, &fixture.responses).unwrap();

    let strip_engine = |r: &ScoringResult| {
        let mut v: serde_json::Value = serde_json::to_value(r).unwrap();
        if let Some(obj) = v.as_object_mut() {
            obj.remove("engine_version");
        }
        serde_json::to_string(&v).unwrap()
    };

    assert_eq!(strip_engine(&r1), strip_engine(&r2));
}
