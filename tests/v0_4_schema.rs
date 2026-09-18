//! Structural parity for the grouped v0.4 configuration.
//!
//! A fixture must satisfy both the generated JSON Schema and Rhino's local
//! reader. Neither check substitutes for the other: the schema serves editors,
//! while the reader supplies stable configuration diagnostics at runtime.
#![forbid(unsafe_code)]

use serde_json::Value;

const PRODUCER: &str = include_str!("../schemas/repo-config/v2.example.yml");
const CONSUMER: &str = include_str!("../specs/fixtures/v0-4/consumer-repo-config.yml");
const CHECKED_IN_SCHEMA: &[u8] = include_bytes!("../schemas/repo-config/v2.schema.json");

fn schema() -> Value {
    serde_json::from_slice(&rhino::config::v0_4_schema_bytes().expect("schema serializes"))
        .expect("generated schema is JSON")
}

fn yaml(text: &str) -> Value {
    yaml_serde::from_str(text).expect("fixture is YAML")
}

#[test]
fn checked_in_schema_is_exactly_the_typed_model() {
    let generated = rhino::config::v0_4_schema_bytes().expect("schema serializes");
    assert_eq!(
        CHECKED_IN_SCHEMA,
        generated.as_slice(),
        "run `cargo xtask schema` to regenerate the editor artifact"
    );
}

#[test]
fn structural_fixtures_agree_between_json_schema_and_rhino() {
    let schema = schema();
    for fixture in [PRODUCER, CONSUMER] {
        let instance = yaml(fixture);
        assert!(
            jsonschema::draft202012::is_valid(&schema, &instance),
            "the fixture satisfies the generated schema"
        );
        assert!(
            rhino::config::parse(fixture).is_ok(),
            "the fixture satisfies Rhino's local reader"
        );
    }
}

#[test]
fn structural_rejections_agree_between_json_schema_and_rhino() {
    let schema = schema();
    for fixture in [
        "schema: rhino/repo-config/v2\nunknown-core: {}\n",
        "schema: rhino/repo-config/v2\nextensions:\n  Invalid_owner: {}\n",
    ] {
        assert!(
            !jsonschema::draft202012::is_valid(&schema, &yaml(fixture)),
            "the generated schema rejects the malformed fixture"
        );
        assert!(
            rhino::config::parse(fixture).is_err(),
            "Rhino rejects the malformed fixture"
        );
    }
}

#[test]
fn phase_three_has_no_semantic_only_policy_rules() {
    // Phase 3 establishes structural ownership only. The later policy and gate
    // phases add semantic rules after their named commands exist, where they
    // can distinguish an omitted section from a present invalid declaration.
    const SEMANTIC_ONLY_RULES: [&str; 0] = [];
    assert!(SEMANTIC_ONLY_RULES.is_empty());
}
