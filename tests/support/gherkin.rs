//! A minimal Gherkin reader.
//!
//! Hand-written rather than taken from a crate for one reason: the static
//! coverage check enumerates every scenario in the corpus without executing
//! anything, and it must do so from the same parse the adapters bind against. A
//! runner that owned its own discovery would let the two drift, which is the
//! failure that check exists to prevent.

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

/// One step line, with its inline table if it had one.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Step {
    pub keyword: String,
    pub text: String,
    pub table: Vec<Vec<String>>,
    pub bullets: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Scenario {
    pub feature: String,
    pub name: String,
    pub tags: Vec<String>,
    pub steps: Vec<Step>,
    pub is_outline: bool,
    pub examples: Vec<BTreeMap<String, String>>,
}

#[derive(Debug, Clone)]
pub struct Corpus {
    pub scenarios: Vec<Scenario>,
}

impl Corpus {
    /// Read every `.feature` file below `root`, sorted by path so the order is
    /// stable across filesystems.
    pub fn read(root: &Path) -> Corpus {
        let mut files: Vec<PathBuf> = Vec::new();
        collect(root, &mut files);
        files.sort();

        let mut scenarios = Vec::new();
        for path in files {
            let text = fs::read_to_string(&path)
                .unwrap_or_else(|error| panic!("unreadable feature {}: {error}", path.display()));
            let stem = path
                .file_stem()
                .and_then(|value| value.to_str())
                .unwrap_or_default()
                .to_string();
            scenarios.extend(parse(&stem, &text));
        }
        Corpus { scenarios }
    }

    /// The corpus at the crate root, which is what every adapter binds against.
    pub fn canonical() -> Corpus {
        Corpus::read(Path::new(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/specs/behaviours"
        )))
    }

    pub fn keys(&self) -> Vec<(String, String)> {
        self.scenarios
            .iter()
            .map(|scenario| (scenario.feature.clone(), scenario.name.clone()))
            .collect()
    }

    pub fn find(&self, feature: &str, name: &str) -> Option<&Scenario> {
        self.scenarios
            .iter()
            .find(|scenario| scenario.feature == feature && scenario.name == name)
    }

    /// Duplicate scenario names inside one feature would make a binding
    /// ambiguous, so the corpus refuses them rather than silently binding one.
    pub fn duplicates(&self) -> Vec<(String, String)> {
        let mut seen: BTreeMap<(String, String), usize> = BTreeMap::new();
        for key in self.keys() {
            *seen.entry(key).or_insert(0) += 1;
        }
        seen.into_iter()
            .filter(|(_, count)| *count > 1)
            .map(|(key, _)| key)
            .collect()
    }
}

impl Scenario {
    /// A Scenario Outline runs once per example row; a plain Scenario runs once.
    /// Expanding here keeps every consumer counting the same executable units.
    pub fn expansions(&self) -> Vec<Vec<Step>> {
        if !self.is_outline {
            return vec![self.steps.clone()];
        }
        self.examples
            .iter()
            .map(|row| {
                self.steps
                    .iter()
                    .map(|step| Step {
                        keyword: step.keyword.clone(),
                        text: substitute(&step.text, row),
                        table: step
                            .table
                            .iter()
                            .map(|cells| cells.iter().map(|c| substitute(c, row)).collect())
                            .collect(),
                        bullets: step.bullets.iter().map(|b| substitute(b, row)).collect(),
                    })
                    .collect()
            })
            .collect()
    }
}

fn substitute(text: &str, row: &BTreeMap<String, String>) -> String {
    let mut result = text.to_string();
    for (key, value) in row {
        result = result.replace(&format!("<{key}>"), value);
    }
    result
}

fn collect(directory: &Path, into: &mut Vec<PathBuf>) {
    let Ok(entries) = fs::read_dir(directory) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect(&path, into);
        } else if path.extension().and_then(|value| value.to_str()) == Some("feature") {
            into.push(path);
        }
    }
}

fn split_row(line: &str) -> Vec<String> {
    let trimmed = line.trim();
    let inner = trimmed
        .strip_prefix('|')
        .and_then(|rest| rest.strip_suffix('|'))
        .unwrap_or(trimmed);
    inner
        .split('|')
        .map(|cell| cell.trim().to_string())
        .collect()
}

const STEP_KEYWORDS: [&str; 5] = ["Given ", "When ", "Then ", "And ", "But "];

fn parse(feature: &str, text: &str) -> Vec<Scenario> {
    let mut scenarios: Vec<Scenario> = Vec::new();
    let mut pending_tags: Vec<String> = Vec::new();
    let mut in_examples = false;
    let mut example_header: Vec<String> = Vec::new();

    for line in text.lines() {
        let trimmed = line.trim();

        if trimmed.starts_with('#') || trimmed.is_empty() {
            continue;
        }

        if trimmed.starts_with('@') {
            pending_tags = trimmed
                .split_whitespace()
                .map(|tag| tag.trim_start_matches('@').to_string())
                .collect();
            continue;
        }

        let outline = trimmed.strip_prefix("Scenario Outline:");
        let plain = trimmed.strip_prefix("Scenario:");
        if let Some(name) = outline.or(plain) {
            scenarios.push(Scenario {
                feature: feature.to_string(),
                name: name.trim().to_string(),
                tags: std::mem::take(&mut pending_tags),
                steps: Vec::new(),
                is_outline: outline.is_some(),
                examples: Vec::new(),
            });
            in_examples = false;
            example_header.clear();
            continue;
        }

        if trimmed.starts_with("Examples:") {
            in_examples = true;
            example_header.clear();
            continue;
        }

        let Some(scenario) = scenarios.last_mut() else {
            // Feature-level narrative, before the first scenario.
            continue;
        };

        if in_examples && trimmed.starts_with('|') {
            let cells = split_row(trimmed);
            if example_header.is_empty() {
                example_header = cells;
            } else {
                scenario
                    .examples
                    .push(example_header.iter().cloned().zip(cells).collect());
            }
            continue;
        }

        if trimmed.starts_with('|') {
            if let Some(step) = scenario.steps.last_mut() {
                step.table.push(split_row(trimmed));
            }
            continue;
        }

        // A `*` line is a bullet belonging to the step above it: the corpus uses
        // it for unordered expected sets, where a table would imply columns.
        if let Some(bullet) = trimmed.strip_prefix("* ") {
            if let Some(step) = scenario.steps.last_mut() {
                step.bullets.push(bullet.trim().to_string());
            }
            continue;
        }

        for keyword in STEP_KEYWORDS {
            if let Some(body) = trimmed.strip_prefix(keyword) {
                scenario.steps.push(Step {
                    keyword: keyword.trim().to_string(),
                    text: body.trim().to_string(),
                    table: Vec::new(),
                    bullets: Vec::new(),
                });
                break;
            }
        }
    }

    scenarios
}
