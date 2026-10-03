//! Quality-gate structure validation.
//!
//! A repository that runs bounded quality gates declares where its workflows
//! live and what its gate contract names. This validator checks only the
//! structure that declaration implies: where each gate, propagation, and
//! workflow module sits, which headings each gate and propagation carries, the
//! verdicts a gate names, and that no gate can run more than three cycles.
//! Whether a gate is well written is the gate's own judgement, never RHINO's.
//!
//! Each finding carries the rule it answers to, `QG01` to `QG11`, as its
//! `rule` detail. A declaration this check could not apply as written is
//! refused before this module runs and before any file is read: `QG09`, a
//! configured cycle ceiling outside 1 to 3, while the configuration is read,
//! and the rest by the declaration check `repo-config validate` shares. Every
//! policy this module receives is already one it can check.

use crate::config::WordBudget;
use crate::governance::word_budget;
use crate::report::{Detail, Finding, Report};
use crate::runtime::{Tree, TreeError};
use crate::v0_4::config::QualityGatesPolicy;
use crate::v0_4::validators::markdown_targets;
use std::collections::BTreeSet;

const CATEGORY: &str = "quality-gates";
const GATE_SUFFIX: &str = "-quality-gate.md";
const PROPAGATION_SUFFIX: &str = "-propagation.md";
const INDEX: &str = "README.md";
const CYCLE_INPUT: &str = "max-cycles";
const CYCLE_CEILING: std::ops::RangeInclusive<usize> = 1..=3;

/// Why a run could not happen at all.
enum Refusal {
    Invalid(String),
    Unreadable(String),
}

pub(crate) fn validate(
    tree: &dyn Tree,
    policy: &QualityGatesPolicy,
    budget: Option<&WordBudget>,
) -> Report {
    match audit(tree, policy, budget) {
        Ok(report) => report,
        Err(Refusal::Invalid(reason)) => Report::refused(CATEGORY, reason),
        Err(Refusal::Unreadable(reason)) => Report::unreadable(CATEGORY, reason),
    }
}

fn audit(
    tree: &dyn Tree,
    policy: &QualityGatesPolicy,
    budget: Option<&WordBudget>,
) -> Result<Report, Refusal> {
    for declared in [&policy.root, &policy.agents] {
        if tree.passes_through_link(declared) {
            return Err(Refusal::Unreadable(format!(
                "{declared}: passes through a symbolic link, which RHINO never follows"
            )));
        }
    }
    let limits = budget
        .map(word_budget::Limits::compile)
        .transpose()
        .map_err(Refusal::Invalid)?;

    let mut audit = Audit {
        tree,
        policy,
        report: Report::new(CATEGORY, "file"),
        read: BTreeSet::new(),
    };
    audit.workflow_root();
    audit.module_directories(limits.as_ref())?;
    audit.placement();
    audit.families();
    audit.gates()?;
    audit.propagations()?;
    Ok(audit.report)
}

struct Audit<'a> {
    tree: &'a dyn Tree,
    policy: &'a QualityGatesPolicy,
    report: Report,
    /// Paths already listed as read: a gate that is also a split workflow's
    /// entrypoint is read by two rules but inspected once.
    read: BTreeSet<String>,
}

impl Audit<'_> {
    fn group(&self, name: &str) -> String {
        format!("{}/{name}", self.policy.root)
    }

    fn gate_directory(&self) -> String {
        self.group(&self.policy.gate_group)
    }

    fn present(&self, path: &str) -> bool {
        !matches!(self.tree.read(path), Err(TreeError::NotFound))
    }

    fn read(&mut self, path: &str) -> Result<String, Refusal> {
        let text = self.tree.read(path).map_err(|error| {
            Refusal::Unreadable(match error {
                TreeError::NotFound => format!("{path}: vanished while it was read"),
                TreeError::Unreadable(reason) => format!("{path}: {reason}"),
                TreeError::NotText => format!("{path}: holds no text"),
            })
        })?;
        if self.read.insert(path.to_string()) {
            self.report.scanned(path);
        }
        Ok(text)
    }

    fn found(&mut self, rule: &'static str, finding: Finding) {
        self.report
            .found(finding.with("rule", Detail::Text(rule.to_string())));
    }

    /// QG01: the workflow root holds its index and the declared groups only.
    fn workflow_root(&mut self) {
        for entry in self.tree.children(&self.policy.root) {
            let name = base_name(&entry);
            let allowed = if self.tree.is_directory(&entry) {
                self.policy.groups.iter().any(|group| group == name)
            } else {
                name == INDEX
            };
            if !allowed {
                self.found(
                    "QG01",
                    Finding::new(
                        "unexpected-workflow-entry",
                        &entry,
                        "the workflow root holds only its README and the declared groups",
                    ),
                );
            }
        }
    }

    /// QG10 and QG11: every directory inside a group is one workflow's
    /// modules, beside that workflow's entrypoint and listed by its index.
    fn module_directories(
        &mut self,
        limits: Option<&word_budget::Limits<'_>>,
    ) -> Result<(), Refusal> {
        for group in self.policy.groups.clone() {
            let group = self.group(&group);
            for directory in self.tree.children(&group) {
                if self.tree.is_directory(&directory) {
                    self.module_directory(&directory, limits)?;
                }
            }
        }
        Ok(())
    }

    fn module_directory(
        &mut self,
        directory: &str,
        limits: Option<&word_budget::Limits<'_>>,
    ) -> Result<(), Refusal> {
        let entrypoint = format!("{directory}.md");
        let paired = self.present(&entrypoint);
        if !paired {
            self.found(
                "QG10",
                Finding::new(
                    "unpaired-workflow-module-directory",
                    directory,
                    "a module directory sits beside a workflow file of the same name",
                ),
            );
        }

        let mut modules = Vec::new();
        let mut indexed = false;
        for entry in self.tree.children(directory) {
            let file = !self.tree.is_directory(&entry);
            let name = base_name(&entry);
            if file && name == INDEX {
                indexed = true;
            } else if file && is_module(name) {
                modules.push(entry);
            } else {
                self.found(
                    "QG10",
                    Finding::new(
                        "unexpected-workflow-module-entry",
                        &entry,
                        "a module directory holds only README.md and NNN-*.md modules",
                    ),
                );
            }
        }

        let index = format!("{directory}/{INDEX}");
        if indexed {
            let text = self.read(&index)?;
            let listed = markdown_targets(&index, &text);
            for module in modules.iter().filter(|module| !listed.contains(*module)) {
                self.found(
                    "QG10",
                    Finding::new(
                        "unlisted-workflow-module",
                        module,
                        "the module directory's README does not list this module",
                    ),
                );
            }
        } else {
            self.found(
                "QG10",
                Finding::new(
                    "missing-workflow-module-index",
                    &index,
                    "a module directory carries a README listing its modules",
                ),
            );
        }

        let governed = limits
            .filter(|_| paired && !modules.is_empty())
            .and_then(|limits| limits.limit(&entrypoint).map(|limit| (limits, limit)));
        if let Some((limits, limit)) = governed {
            let mut words = 0;
            for path in std::iter::once(&entrypoint).chain(modules.iter()) {
                words += limits.count(&self.read(path)?);
            }
            if words <= limit {
                self.found(
                    "QG11",
                    Finding::new(
                        "unnecessary-workflow-split",
                        &entrypoint,
                        "the workflow and its modules fit the word budget as one file",
                    )
                    .with("words", Detail::Count(words))
                    .with("limit", Detail::Count(limit)),
                );
            }
        }
        Ok(())
    }

    /// QG03: a gate or propagation sits directly in the gate group, and only
    /// for a declared family.
    fn placement(&mut self) {
        let gate_directory = self.gate_directory();
        let prefix = format!("{}/", self.policy.root);
        let declared = self.declared();
        for path in self.tree.files() {
            if !path.starts_with(&prefix) {
                continue;
            }
            let Some(family) = gate_family(&path) else {
                continue;
            };
            if parent(&path) != gate_directory {
                self.found(
                    "QG03",
                    Finding::new(
                        "misplaced-quality-gate-file",
                        &path,
                        "every gate and propagation sits directly in the gate group",
                    ),
                );
            } else if !declared.contains(family) {
                self.found(
                    "QG03",
                    Finding::new(
                        "undeclared-quality-gate-family",
                        &path,
                        "the gate family is not declared",
                    )
                    .with("family", Detail::Text(family.to_string())),
                );
            }
        }
    }

    fn declared(&self) -> BTreeSet<String> {
        self.policy
            .gates
            .iter()
            .map(|gate| gate.family.clone())
            .collect()
    }

    /// QG02 and QG08: each declared family holds its gate, its propagation,
    /// its judge, and its repairer -- `<family>-checker` and `<family>-fixer`
    /// unless the family names other agents.
    fn families(&mut self) {
        let gate_directory = self.gate_directory();
        let mut gates = self.policy.gates.clone();
        gates.sort_by(|left, right| left.family.cmp(&right.family));
        for gate in gates {
            let family = gate.family.clone();
            for suffix in [GATE_SUFFIX, PROPAGATION_SUFFIX] {
                let path = format!("{gate_directory}/{family}{suffix}");
                if !self.present(&path) {
                    self.found(
                        "QG02",
                        Finding::new(
                            "missing-quality-gate-file",
                            &path,
                            "a declared gate family has no file here",
                        )
                        .with("family", Detail::Text(family.clone())),
                    );
                }
            }
            for (role, agent) in [("judge", gate.judge()), ("repairer", gate.repairer())] {
                let path = format!("{}/{agent}.md", self.policy.agents);
                if !self.present(&path) {
                    self.found(
                        "QG08",
                        Finding::new(
                            "missing-gate-agent",
                            &path,
                            "a declared gate family has no agent here",
                        )
                        .with("family", Detail::Text(family.clone()))
                        .with("role", Detail::Text(role.to_string()))
                        .with("agent", Detail::Text(agent)),
                    );
                }
            }
        }
    }

    /// The gate or propagation files directly in the gate group.
    fn gate_files(&self, suffix: &str) -> Vec<String> {
        let gate_directory = self.gate_directory();
        self.tree
            .children(&gate_directory)
            .into_iter()
            .filter(|path| {
                !self.tree.is_directory(path)
                    && gate_family(path).is_some()
                    && path.ends_with(suffix)
            })
            .collect()
    }

    /// QG04, QG06, and QG07, for every gate in the gate group.
    fn gates(&mut self) -> Result<(), Refusal> {
        for path in self.gate_files(GATE_SUFFIX) {
            let text = self.read(&path)?;
            let document = Document::parse(&text);
            self.gate_headings(&path, &document);
            self.cycle_ceiling(&path, &document);
            self.retired_inputs(&path, &text);
            self.verdicts(&path, &document);
        }
        Ok(())
    }

    /// QG04: the declared gate headings, all present, in declared order.
    fn gate_headings(&mut self, path: &str, document: &Document) {
        let mut furthest = 0;
        for heading in self.policy.gate_headings.clone() {
            match document.heading(&heading) {
                None => self.found(
                    "QG04",
                    Finding::new(
                        "missing-gate-heading",
                        path,
                        "a required gate heading is absent",
                    )
                    .with("heading", Detail::Text(heading)),
                ),
                Some((position, line)) if position < furthest => {
                    self.found(
                        "QG04",
                        Finding::new(
                            "gate-heading-out-of-order",
                            path,
                            "a required gate heading appears before one it must follow",
                        )
                        .at_line(line)
                        .with("heading", Detail::Text(heading)),
                    );
                }
                Some((position, _)) => furthest = position,
            }
        }
    }

    /// QG06: the cycle input's values and default stay within 1 to 3.
    fn cycle_ceiling(&mut self, path: &str, document: &Document) {
        let rows: Vec<(usize, Vec<String>)> = document
            .lines
            .iter()
            .filter_map(|(line, text)| table_row(text).map(|cells| (*line, cells)))
            .filter(|(_, cells)| {
                cells
                    .first()
                    .is_some_and(|name| name.trim_matches('`') == CYCLE_INPUT)
            })
            .collect();
        if rows.is_empty() {
            self.found(
                "QG06",
                Finding::new(
                    "unbounded-gate-cycles",
                    path,
                    "the gate declares no max-cycles input bounded by 1 to 3",
                ),
            );
        }
        for (line, cells) in rows {
            let default = cells
                .last()
                .and_then(|cell| cell.trim_matches('`').parse::<usize>().ok());
            let bounded = cells.len() > 1
                && default.is_some_and(|value| CYCLE_CEILING.contains(&value))
                && cells[1..]
                    .iter()
                    .flat_map(|cell| integers(cell))
                    .all(|value| CYCLE_CEILING.contains(&value));
            if !bounded {
                self.found(
                    "QG06",
                    Finding::new(
                        "unbounded-gate-cycles",
                        path,
                        "the max-cycles input allows a value or default outside 1 to 3",
                    )
                    .at_line(line),
                );
            }
        }
    }

    /// QG06: no gate names a retired input anywhere in its text.
    fn retired_inputs(&mut self, path: &str, text: &str) {
        for input in self.policy.retired_inputs.clone() {
            if let Some(line) = text
                .lines()
                .position(|line| names_token(line, &input))
                .map(|index| index + 1)
            {
                self.found(
                    "QG06",
                    Finding::new("retired-gate-input", path, "the gate names a retired input")
                        .at_line(line)
                        .with("input", Detail::Text(input)),
                );
            }
        }
    }

    /// QG07: the verdict section names only declared verdicts.
    fn verdicts(&mut self, path: &str, document: &Document) {
        let Some((position, start)) = document.heading(&self.policy.verdict_heading) else {
            return;
        };
        let end = document
            .headings
            .get(position + 1)
            .map_or(usize::MAX, |(line, _, _)| *line);
        for (line, text) in document
            .lines
            .iter()
            .filter(|(line, _)| *line > start && *line < end)
        {
            for verdict in code_spans(text).filter(|span| is_verdict_shape(span)) {
                if !self.policy.verdicts.iter().any(|known| known == verdict) {
                    self.found(
                        "QG07",
                        Finding::new(
                            "unknown-gate-verdict",
                            path,
                            "the gate names a verdict outside the declared verdicts",
                        )
                        .at_line(*line)
                        .with("verdict", Detail::Text(verdict.to_string())),
                    );
                }
            }
        }
    }

    /// QG05: every declared propagation heading is present.
    fn propagations(&mut self) -> Result<(), Refusal> {
        for path in self.gate_files(PROPAGATION_SUFFIX) {
            let text = self.read(&path)?;
            let document = Document::parse(&text);
            for heading in self.policy.propagation_headings.clone() {
                if document.heading(&heading).is_none() {
                    self.found(
                        "QG05",
                        Finding::new(
                            "missing-propagation-heading",
                            &path,
                            "a required propagation heading is absent",
                        )
                        .with("heading", Detail::Text(heading)),
                    );
                }
            }
        }
        Ok(())
    }
}

/// A Markdown file's lines outside fenced code, and the level-one and
/// level-two headings among them. Fenced lines are examples, never structure.
struct Document {
    lines: Vec<(usize, String)>,
    /// Line number, level, and title.
    headings: Vec<(usize, usize, String)>,
}

impl Document {
    fn parse(text: &str) -> Self {
        let mut lines = Vec::new();
        let mut headings = Vec::new();
        let mut fenced = false;
        for (index, line) in text.lines().enumerate() {
            let number = index + 1;
            let trimmed = line.trim_start();
            if trimmed.starts_with("```") || trimmed.starts_with("~~~") {
                fenced = !fenced;
                continue;
            }
            if fenced {
                continue;
            }
            for (marker, level) in [("# ", 1), ("## ", 2)] {
                if let Some(title) = line.strip_prefix(marker) {
                    headings.push((number, level, title.trim().to_string()));
                }
            }
            lines.push((number, line.to_string()));
        }
        Self { lines, headings }
    }

    /// The first level-two heading with this exact title, and its position
    /// among all headings.
    fn heading(&self, title: &str) -> Option<(usize, usize)> {
        self.headings
            .iter()
            .enumerate()
            .find(|(_, (_, level, text))| *level == 2 && text == title)
            .map(|(position, (line, _, _))| (position, *line))
    }
}

fn base_name(path: &str) -> &str {
    path.rsplit_once('/').map_or(path, |(_, name)| name)
}

fn parent(path: &str) -> &str {
    path.rsplit_once('/').map_or("", |(directory, _)| directory)
}

/// The family a gate or propagation file is named for.
fn gate_family(path: &str) -> Option<&str> {
    let name = base_name(path);
    [GATE_SUFFIX, PROPAGATION_SUFFIX]
        .iter()
        .find_map(|suffix| name.strip_suffix(suffix))
        .filter(|family| !family.is_empty())
}

/// `NNN-<name>.md`: three digits, a hyphen, a name, and the extension.
fn is_module(name: &str) -> bool {
    let bytes = name.as_bytes();
    name.len() > "000-.md".len()
        && bytes[..3].iter().all(u8::is_ascii_digit)
        && bytes[3] == b'-'
        && name.ends_with(".md")
}

fn table_row(line: &str) -> Option<Vec<String>> {
    let inner = line.trim().strip_prefix('|')?;
    let inner = inner.strip_suffix('|').unwrap_or(inner);
    Some(
        inner
            .split('|')
            .map(|cell| cell.trim().to_string())
            .collect(),
    )
}

fn integers(text: &str) -> impl Iterator<Item = usize> + '_ {
    text.split(|character: char| !character.is_ascii_digit())
        .filter_map(|digits| digits.parse().ok())
}

/// Whether `token` appears in `line` as a whole name, not inside a longer one.
fn names_token(line: &str, token: &str) -> bool {
    let joins = |character: Option<char>| {
        character.is_some_and(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_')
    };
    line.match_indices(token).any(|(at, _)| {
        !joins(line[..at].chars().next_back()) && !joins(line[at + token.len()..].chars().next())
    })
}

fn code_spans(line: &str) -> impl Iterator<Item = &str> {
    line.split('`').skip(1).step_by(2)
}

/// An all-capital code span such as `PASS_WITH_FINDINGS` reads as a verdict.
fn is_verdict_shape(span: &str) -> bool {
    span.len() > 1
        && span.starts_with(|c: char| c.is_ascii_uppercase())
        && span
            .chars()
            .all(|c| c.is_ascii_uppercase() || c.is_ascii_digit() || c == '_')
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cli::Format;
    use crate::config::{Surface, WordRule};
    use crate::runtime::MemoryTree;
    use crate::v0_4::config::QualityGateFamily;

    const GATE: &str = "# Gate\n\n## Entry\n\n## Inputs\n\n| Input | Default |\n| `max-cycles` | 3 |\n\n## Verdict\n\n`PASS`\n";
    const PROPAGATION: &str = "# Propagation\n\n## Contract\n\n## Scope\n";

    fn policy() -> QualityGatesPolicy {
        QualityGatesPolicy {
            root: "flows".to_string(),
            agents: "agents".to_string(),
            groups: vec!["plan".to_string(), "quality".to_string()],
            gate_group: "quality".to_string(),
            gate_headings: vec![
                "Entry".to_string(),
                "Inputs".to_string(),
                "Verdict".to_string(),
            ],
            verdict_heading: "Verdict".to_string(),
            verdicts: vec!["PASS".to_string()],
            retired_inputs: vec!["max-audits".to_string()],
            propagation_headings: vec!["Contract".to_string(), "Scope".to_string()],
            defaults: None,
            gates: vec![QualityGateFamily {
                family: "plan".to_string(),
                subject: None,
                judge: None,
                repairer: None,
            }],
        }
    }

    fn conforming() -> MemoryTree {
        let tree = MemoryTree::default();
        tree.write("flows/README.md", "# Flows\n");
        tree.write("flows/quality/plan-quality-gate.md", GATE);
        tree.write("flows/quality/plan-propagation.md", PROPAGATION);
        tree.write("agents/plan-checker.md", "# Checker\n");
        tree.write("agents/plan-fixer.md", "# Fixer\n");
        tree
    }

    fn budget(fail: usize, glob: &str) -> WordBudget {
        WordBudget {
            count: WordRule::WhitespaceSeparated,
            surfaces: vec![Surface {
                glob: glob.to_string(),
                fail,
                warn: None,
                target: None,
            }],
        }
    }

    fn json(tree: &MemoryTree, policy: &QualityGatesPolicy, budget: Option<&WordBudget>) -> String {
        let outcome = validate(tree, policy, budget).render(Format::Json);
        format!("{}{}", outcome.stdout, outcome.stderr)
    }

    #[test]
    fn a_conforming_layout_is_clean_and_lists_what_it_read() {
        let outcome = validate(&conforming(), &policy(), None).render(Format::Json);
        assert_eq!(outcome.exit_code, 0, "{}", outcome.stdout);
        assert!(
            outcome
                .stdout
                .contains("flows/quality/plan-quality-gate.md")
        );
    }

    #[test]
    fn a_declared_root_behind_a_link_or_an_unusable_budget_is_refused() {
        let mut linked = conforming();
        linked.mark_link("flows");
        assert_eq!(
            validate(&linked, &policy(), None)
                .render(Format::Text)
                .exit_code,
            2
        );
        let outcome =
            validate(&conforming(), &policy(), Some(&budget(5, "["))).render(Format::Text);
        assert_eq!(outcome.exit_code, 2);
    }

    #[test]
    fn a_file_that_cannot_be_read_refuses_the_run() {
        for mark in [
            MemoryTree::mark_unreadable,
            MemoryTree::mark_vanished,
            MemoryTree::mark_binary,
        ] {
            let mut tree = conforming();
            mark(&mut tree, "flows/quality/plan-quality-gate.md");
            let outcome = validate(&tree, &policy(), None).render(Format::Text);
            assert_eq!(outcome.exit_code, 2, "{}", outcome.stderr);
            assert!(outcome.stderr.contains("plan-quality-gate.md"));
        }
        let mut tree = conforming();
        tree.write("flows/quality/plan-propagation.md", PROPAGATION);
        tree.mark_unreadable("flows/quality/plan-propagation.md");
        assert_eq!(
            validate(&tree, &policy(), None)
                .render(Format::Text)
                .exit_code,
            2
        );
    }

    #[test]
    fn the_workflow_root_holds_its_index_and_declared_groups_only() {
        let tree = conforming();
        tree.write("flows/notes.md", "# Notes\n");
        tree.write("flows/README.md/stray.md", "# Stray\n");
        let report = json(&tree, &policy(), None);
        assert!(report.contains("\"path\":\"flows/notes.md\""), "{report}");
        assert!(report.contains("\"path\":\"flows/README.md\""), "{report}");
    }

    #[test]
    fn a_module_directory_is_paired_indexed_and_holds_only_modules() {
        let tree = conforming();
        tree.write("flows/plan/run.md", "# Run\n");
        tree.write("flows/plan/run/README.md", "- [One](001-one.md)\n");
        tree.write("flows/plan/run/001-one.md", "# One\n");
        tree.write("flows/plan/run/002-two.md", "# Two\n");
        tree.write("flows/plan/run/notes.md", "# Notes\n");
        tree.write("flows/plan/run/nested/001-deep.md", "# Deep\n");
        tree.write("flows/plan/bare/001-one.md", "# One\n");
        let report = json(&tree, &policy(), None);
        for expected in [
            "unlisted-workflow-module\",\"path\":\"flows/plan/run/002-two.md",
            "unexpected-workflow-module-entry\",\"path\":\"flows/plan/run/notes.md",
            "unexpected-workflow-module-entry\",\"path\":\"flows/plan/run/nested",
            "missing-workflow-module-index\",\"path\":\"flows/plan/bare/README.md",
            "unpaired-workflow-module-directory\",\"path\":\"flows/plan/bare",
        ] {
            assert!(report.contains(expected), "{expected}\n{report}");
        }
        assert!(!report.contains("001-one.md\",\"message\":\"the module directory"));
        for name in ["001-a.md", "123-long-name.md"] {
            assert!(is_module(name));
        }
        for name in ["001-.md", "01-a.md", "001_a.md", "abc-a.md", "001-a.txt"] {
            assert!(!is_module(name), "{name}");
        }
    }

    #[test]
    fn a_split_is_measured_only_against_a_budget_that_governs_its_entrypoint() {
        let tree = conforming();
        tree.write("flows/plan/run.md", "one two three");
        tree.write("flows/plan/run/README.md", "- [One](001-one.md)\n");
        tree.write("flows/plan/run/001-one.md", "four five");
        let fits = json(&tree, &policy(), Some(&budget(5, "flows/**/*.md")));
        assert!(fits.contains("\"words\":5,\"limit\":5"), "{fits}");
        let over = json(&tree, &policy(), Some(&budget(4, "flows/**/*.md")));
        assert!(!over.contains("unnecessary-workflow-split"), "{over}");
        let ungoverned = json(&tree, &policy(), Some(&budget(50, "docs/**/*.md")));
        assert!(
            !ungoverned.contains("unnecessary-workflow-split"),
            "{ungoverned}"
        );
        tree.write("flows/plan/empty.md", "one");
        tree.write("flows/plan/empty/README.md", "# Empty\n");
        let unsplit = json(&tree, &policy(), Some(&budget(50, "flows/**/*.md")));
        assert!(
            !unsplit.contains("unnecessary-workflow-split\",\"path\":\"flows/plan/empty.md"),
            "{unsplit}"
        );
    }

    #[test]
    fn a_split_gate_is_listed_and_counted_once() {
        let tree = conforming();
        tree.write(
            "flows/quality/plan-quality-gate/README.md",
            "- [One](001-one.md)\n",
        );
        tree.write("flows/quality/plan-quality-gate/001-one.md", "four five");
        let report: serde_json::Value =
            serde_json::from_str(&json(&tree, &policy(), Some(&budget(500, "flows/**/*.md"))))
                .expect("the report is one JSON object");
        let scanned: Vec<&str> = report["scanned"]
            .as_array()
            .expect("the report lists what it read")
            .iter()
            .filter_map(serde_json::Value::as_str)
            .collect();
        let unique: BTreeSet<&str> = scanned.iter().copied().collect();
        assert_eq!(scanned.len(), unique.len(), "{scanned:?}");
        assert_eq!(report["inspected"], unique.len(), "{report}");
    }

    #[test]
    fn gate_files_sit_in_the_gate_group_for_declared_families_only() {
        let tree = conforming();
        tree.write("flows/plan/docs-propagation.md", PROPAGATION);
        tree.write("flows/quality/ci-quality-gate.md", GATE);
        tree.write("elsewhere/x-quality-gate.md", GATE);
        tree.write("flows/quality/quality-gate.md", "# Bare\n");
        let report = json(&tree, &policy(), None);
        assert!(
            report.contains(
                "misplaced-quality-gate-file\",\"path\":\"flows/plan/docs-propagation.md"
            )
        );
        assert!(report.contains(
            "undeclared-quality-gate-family\",\"path\":\"flows/quality/ci-quality-gate.md"
        ));
        assert!(!report.contains("elsewhere"), "{report}");
        assert!(
            !report.contains("flows/quality/quality-gate.md\",\"message"),
            "{report}"
        );
    }

    #[test]
    fn a_declared_family_names_each_missing_file_and_agent() {
        let tree = MemoryTree::default();
        let report = json(&tree, &policy(), None);
        for path in [
            "flows/quality/plan-quality-gate.md",
            "flows/quality/plan-propagation.md",
            "agents/plan-checker.md",
            "agents/plan-fixer.md",
        ] {
            assert!(
                report.contains(&format!("\"path\":\"{path}\"")),
                "{path}\n{report}"
            );
        }
    }

    #[test]
    fn gate_headings_are_matched_in_order_and_outside_fences() {
        let tree = conforming();
        tree.write(
            "flows/quality/plan-quality-gate.md",
            "# Gate\n\n```text\n## Entry\n```\n\n## Verdict\n\n`PASS`\n\n## Inputs\n\n| `max-cycles` | 3 |\n",
        );
        let report = json(&tree, &policy(), None);
        assert!(
            report.contains("\"kind\":\"missing-gate-heading\""),
            "{report}"
        );
        assert!(
            report.contains("\"kind\":\"gate-heading-out-of-order\""),
            "{report}"
        );
        let no_verdict = conforming();
        no_verdict.write(
            "flows/quality/plan-quality-gate.md",
            "## Entry\n\n## Inputs\n\n| `max-cycles` | 3 |\n\n`MAYBE`\n",
        );
        let report = json(&no_verdict, &policy(), None);
        assert!(!report.contains("unknown-gate-verdict"), "{report}");
    }

    #[test]
    fn the_cycle_input_is_bounded_by_three_and_retired_inputs_are_named() {
        for (row, bounded) in [
            ("| `max-cycles` | 1, 2, or 3 | 3 |", true),
            ("| max-cycles | 3", true),
            ("| `max-cycles` | 1 to 4 | 3 |", false),
            ("| `max-cycles` | 1, 2, or 3 | 0 |", false),
            ("| `max-cycles` | 1, 2, or 3 | three |", false),
            ("| `max-cycles` |", false),
            ("| Input | Default |", false),
        ] {
            let tree = conforming();
            tree.write(
                "flows/quality/plan-quality-gate.md",
                &format!("## Entry\n\n## Inputs\n\n{row}\n\n## Verdict\n\n`PASS`\n"),
            );
            let report = json(&tree, &policy(), None);
            assert_eq!(
                !report.contains("unbounded-gate-cycles"),
                bounded,
                "{row}\n{report}"
            );
        }
        assert!(names_token("no `max-audits` here", "max-audits"));
        assert!(names_token("max-audits", "max-audits"));
        assert!(!names_token("the max-audits-ceiling", "max-audits"));
        assert!(!names_token("a min_max-audits", "max-audits"));
    }

    #[test]
    fn only_capitalised_code_spans_in_the_verdict_section_read_as_verdicts() {
        let tree = conforming();
        tree.write(
            "flows/quality/plan-quality-gate.md",
            "## Entry\n\n## Inputs\n\n| `max-cycles` | 3 |\n\n## Verdict\n\n`PASS`, `mode`, `A`, `HOLD_2`\n\n## After\n\n`LATER`\n",
        );
        let report = json(&tree, &policy(), None);
        assert!(report.contains("\"verdict\":\"HOLD_2\""), "{report}");
        for absent in [
            "\"verdict\":\"PASS\"",
            "\"verdict\":\"mode\"",
            "\"verdict\":\"A\"",
            "LATER",
        ] {
            assert!(!report.contains(absent), "{absent}\n{report}");
        }
    }

    #[test]
    fn a_family_may_name_its_judge_and_repairer_in_place_of_its_defaults() {
        let mut declared = policy();
        declared.gates[0].judge = Some("plan-tester".to_string());
        declared.gates[0].repairer = Some("developer".to_string());
        let tree = MemoryTree::default();
        tree.write("flows/README.md", "# Flows\n");
        tree.write("flows/quality/plan-quality-gate.md", GATE);
        tree.write("flows/quality/plan-propagation.md", PROPAGATION);
        tree.write("agents/developer.md", "# Developer\n");

        let report = json(&tree, &declared, None);
        assert!(report.contains("agents/plan-tester.md"), "{report}");
        assert!(report.contains("\"role\":\"judge\""), "{report}");
        assert!(report.contains("\"agent\":\"plan-tester\""), "{report}");
        assert!(!report.contains("plan-checker"), "{report}");
        assert!(!report.contains("plan-fixer"), "{report}");
        assert!(!report.contains("\"role\":\"repairer\""), "{report}");

        tree.write("agents/plan-tester.md", "# Tester\n");
        assert_eq!(
            validate(&tree, &declared, None)
                .render(Format::Json)
                .exit_code,
            0
        );

        let defaults = json(&MemoryTree::default(), &policy(), None);
        assert!(
            defaults.contains("\"agent\":\"plan-checker\""),
            "{defaults}"
        );
        assert!(defaults.contains("\"agent\":\"plan-fixer\""), "{defaults}");
    }

    #[test]
    fn diagnostics_carry_their_rule_and_repeat_byte_for_byte() {
        let tree = MemoryTree::default();
        tree.write("flows/extra.md", "# Extra\n");
        let first = json(&tree, &policy(), None);
        assert_eq!(first, json(&tree, &policy(), None));
        assert!(first.contains("\"rule\":\"QG01\""), "{first}");
        assert!(first.contains("\"rule\":\"QG02\""), "{first}");
        assert!(first.contains("\"rule\":\"QG08\""), "{first}");
    }
}
