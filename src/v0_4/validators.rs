//! Repository-neutral policy validators for the grouped v0.4 document.
//!
//! Each validator receives only the policy its repository declared and the
//! read-only tree port. Absence refuses rather than creating a hidden product
//! default for a repository's documentation or governance tree.

use crate::config::Config;
use crate::report::{Detail, Finding, Report};
use crate::runtime::{Tree, TreeError};
use crate::scan::Scope;
use crate::v0_4::config::{
    ConventionsPolicy, GovernancePolicy, LayerPolicy, MarkdownPolicy, ReadmeIndexPolicy,
    TraceabilityPolicy, VendorPolicy,
};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};

pub(crate) fn license(policy: Option<&ConventionsPolicy>, tree: &dyn Tree) -> Report {
    let Some(policy) = policy.and_then(|policy| policy.license.as_ref()) else {
        return undeclared("license", "policies.conventions.license");
    };
    let mut report = Report::new("license", "license file");
    let mut seen = BTreeSet::new();

    if policy.exclusions.iter().any(|path| !exact_path(path)) {
        return invalid(
            "license",
            "license exclusions must be exact repository-relative paths",
        );
    }

    for entry in &policy.paths {
        if !exact_path(&entry.path) {
            return invalid(
                "license",
                "license paths must be exact repository-relative paths",
            );
        }
        if !seen.insert(&entry.path) {
            return invalid("license", "license paths must not repeat");
        }
        if policy.exclusions.iter().any(|path| path == &entry.path) {
            continue;
        }
        if let Some(digest) = &entry.sha256
            && !is_sha256(digest)
        {
            return invalid(
                "license",
                "a license sha256 must be 64 lowercase hexadecimal characters",
            );
        }

        let contents = match read(tree, &entry.path) {
            Ok(Some(contents)) => contents,
            Ok(None) => {
                report.found(Finding::new(
                    "missing-license",
                    &entry.path,
                    "the configured license file is absent",
                ));
                continue;
            }
            Err(reason) => return invalid("license", reason),
        };
        report.scanned(&entry.path);

        for identifier in &entry.identifiers {
            if !contents.contains(identifier) {
                report.found(
                    Finding::new(
                        "license-identifier-mismatch",
                        &entry.path,
                        "the configured license identifier is absent",
                    )
                    .with("identifier", Detail::Text(identifier.clone())),
                );
            }
        }
        if entry
            .sha256
            .as_ref()
            .is_some_and(|expected| sha256(&contents) != *expected)
        {
            report.found(Finding::new(
                "license-digest-mismatch",
                &entry.path,
                "the configured license digest does not match",
            ));
        }
    }
    report
}

/// Reuse the established portable reader through a configuration projection.
/// The grouped model owns which section appears; the reader owns only Markdown
/// syntax. The projection therefore supplies no repository defaults.
pub(crate) fn frontmatter(policy: Option<&MarkdownPolicy>, tree: &dyn Tree) -> Report {
    let Some(frontmatter) = policy.and_then(|policy| policy.frontmatter.as_ref()) else {
        return undeclared("frontmatter", "policies.markdown.frontmatter");
    };
    let config = Config {
        frontmatter: Some(frontmatter.clone()),
        ..Config::default()
    };
    crate::markdown::frontmatter::validate(tree, &config, frontmatter)
}

pub(crate) fn internal_link(policy: Option<&MarkdownPolicy>, tree: &dyn Tree) -> Report {
    let Some(internal_link) = policy.and_then(|policy| policy.internal_link.as_ref()) else {
        return undeclared("internal-link", "policies.markdown.internal-link");
    };
    let config = Config {
        internal_link: internal_link.clone(),
        ..Config::default()
    };
    crate::markdown::internal_link::validate(tree, &config)
}

pub(crate) fn mermaid(policy: Option<&MarkdownPolicy>, tree: &dyn Tree, scope: &Scope) -> Report {
    let Some(mermaid) = policy.and_then(|policy| policy.mermaid.as_ref()) else {
        return undeclared("mermaid", "policies.markdown.mermaid");
    };
    let config = Config {
        mermaid: Some(mermaid.clone()),
        ..Config::default()
    };
    crate::markdown::mermaid::validate(tree, &config, mermaid, scope)
}

pub(crate) fn readme_index(policy: Option<&MarkdownPolicy>, tree: &dyn Tree) -> Report {
    let Some(policy) = policy.and_then(|policy| policy.readme_index.as_ref()) else {
        return undeclared("readme-index", "policies.markdown.readme-index");
    };
    readme_index_policy(policy, tree)
}

fn readme_index_policy(policy: &ReadmeIndexPolicy, tree: &dyn Tree) -> Report {
    let mut report = Report::new("readme-index", "directory");
    for declared in &policy.trees {
        if !exact_path(&declared.path) || declared.exclusions.iter().any(|path| !exact_path(path)) {
            return invalid(
                "readme-index",
                "README index tree paths and exclusions must be exact paths relative to the repository",
            );
        }
        let index = format!("{}/README.md", declared.path);
        let contents = match read(tree, &index) {
            Ok(Some(contents)) => contents,
            Ok(None) => {
                report.inspected_one();
                report.found(Finding::new(
                    "missing-readme-index",
                    &index,
                    "the declared directory has no README index",
                ));
                continue;
            }
            Err(reason) => return invalid("readme-index", reason),
        };
        report.scanned(&index);

        if declared.require_direct_children {
            let linked = markdown_targets(&index, &contents);
            for child in tree.children(&declared.path) {
                if child == index || excluded(&child, &declared.path, &declared.exclusions) {
                    continue;
                }
                if !linked.contains(&child) {
                    report.found(
                        Finding::new(
                            "missing-readme-index-child",
                            &index,
                            "the declared README index omits a direct child",
                        )
                        .with("child", Detail::Text(child)),
                    );
                }
            }
        }

        for annotation in &declared.annotations {
            let Some(path) = child_path(&declared.path, &annotation.path) else {
                return invalid(
                    "readme-index",
                    "README annotations must name exact paths relative to their declared tree",
                );
            };
            let text = match read(tree, &path) {
                Ok(Some(text)) => text,
                Ok(None) => {
                    report.found(Finding::new(
                        "missing-readme-annotation",
                        path,
                        "the configured annotation file is absent",
                    ));
                    continue;
                }
                Err(reason) => return invalid("readme-index", reason),
            };
            report.scanned(&path);
            if !text.contains(&annotation.text) {
                report.found(Finding::new(
                    "missing-readme-annotation",
                    path,
                    "the configured README annotation is absent",
                ));
            }
        }
    }
    report
}

pub(crate) fn vendor(policy: Option<&GovernancePolicy>, tree: &dyn Tree) -> Report {
    let Some(policy) = policy.and_then(|policy| policy.vendor.as_ref()) else {
        return undeclared("vendor", "policies.governance.vendor");
    };
    vendor_policy(policy, tree)
}

fn vendor_policy(policy: &VendorPolicy, tree: &dyn Tree) -> Report {
    if policy.roots.iter().any(|root| !exact_path(root))
        || policy
            .excluded_binding_roots
            .iter()
            .any(|root| !exact_path(root))
    {
        return invalid(
            "vendor",
            "vendor roots and excluded binding roots must be exact repository-relative paths",
        );
    }
    if policy
        .forbidden_terms
        .iter()
        .any(|term| term.trim().is_empty())
    {
        return invalid("vendor", "vendor forbidden terms must not be empty");
    }
    if policy.vocabulary_exceptions.iter().any(|exception| {
        exception.term.trim().is_empty() || exception.paths.iter().any(|path| !exact_path(path))
    }) {
        return invalid(
            "vendor",
            "vendor exception terms and paths must be exact repository-relative paths",
        );
    }
    let mut report = Report::new("vendor", "file");
    for path in tree.files() {
        if !policy.roots.iter().any(|root| under(&path, root))
            || policy
                .excluded_binding_roots
                .iter()
                .any(|root| under(&path, root))
        {
            continue;
        }
        let text = match read(tree, &path) {
            Ok(Some(text)) => text,
            Ok(None) => continue,
            Err(reason) => return invalid("vendor", reason),
        };
        report.scanned(&path);
        for term in &policy.forbidden_terms {
            if text.contains(term) && !vendor_exception(policy, &path, term) {
                report.found(
                    Finding::new(
                        "forbidden-vendor-term",
                        &path,
                        "a configured vendor term appears outside its exact exception",
                    )
                    .with("term", Detail::Text(term.clone())),
                );
            }
        }
    }
    report
}

pub(crate) fn layers(policy: Option<&GovernancePolicy>, tree: &dyn Tree) -> Report {
    let Some(policy) = policy.and_then(|policy| policy.layers.as_ref()) else {
        return undeclared("layers", "policies.governance.layers");
    };
    layer_policy(policy, tree)
}

fn layer_policy(policy: &LayerPolicy, tree: &dyn Tree) -> Report {
    if !exact_path(&policy.root) || policy.order.iter().any(|layer| !simple_name(layer)) {
        return invalid(
            "layers",
            "a layer root and every layer name must be exact repository-relative names",
        );
    }
    let expected: BTreeSet<String> = policy.order.iter().cloned().collect();
    if expected.len() != policy.order.len()
        || policy
            .categories
            .keys()
            .any(|layer| !expected.contains(layer))
        || policy
            .categories
            .values()
            .flatten()
            .any(|category| !simple_name(category))
    {
        return invalid(
            "layers",
            "layer order and categories must use unique exact simple names",
        );
    }

    let mut report = Report::new("layers", "governance directory");
    let actual: BTreeSet<String> = tree
        .children(&policy.root)
        .into_iter()
        .filter_map(|path| {
            path.strip_prefix(&format!("{}/", policy.root))
                .map(str::to_string)
        })
        .collect();
    report.inspected(actual.len());
    for layer in &policy.order {
        let path = format!("{}/{}", policy.root, layer);
        if !actual.contains(layer) {
            report.found(Finding::new(
                "missing-governance-layer",
                path,
                "a declared governance layer is absent",
            ));
            continue;
        }
        let actual_categories: BTreeSet<String> = tree
            .children(&path)
            .into_iter()
            .filter(|child| tree.is_directory(child))
            .filter_map(|child| child.strip_prefix(&format!("{path}/")).map(str::to_string))
            .collect();
        for category in policy.categories.get(layer).into_iter().flatten() {
            if !actual_categories.contains(category) {
                report.found(Finding::new(
                    "missing-governance-category",
                    format!("{path}/{category}"),
                    "a declared governance category is absent",
                ));
            }
        }
        for category in actual_categories {
            if !policy
                .categories
                .get(layer)
                .is_some_and(|expected| expected.contains(&category))
            {
                report.found(Finding::new(
                    "unexpected-governance-category",
                    format!("{path}/{category}"),
                    "the governance category is not declared for its layer",
                ));
            }
        }
    }
    for layer in actual.difference(&expected) {
        report.found(Finding::new(
            "unexpected-governance-layer",
            format!("{}/{}", policy.root, layer),
            "the governance layer is not declared",
        ));
    }
    report
}

pub(crate) fn traceability(policy: Option<&GovernancePolicy>, tree: &dyn Tree) -> Report {
    let Some(policy) = policy.and_then(|policy| policy.traceability.as_ref()) else {
        return undeclared("traceability", "policies.governance.traceability");
    };
    traceability_policy(policy, tree)
}

fn traceability_policy(policy: &TraceabilityPolicy, tree: &dyn Tree) -> Report {
    let mut artifacts = BTreeMap::new();
    for artifact in &policy.artifacts {
        if !simple_name(&artifact.id) || !exact_path(&artifact.path) {
            return invalid(
                "traceability",
                "traceability artifacts require unique simple identifiers and exact repository-relative paths",
            );
        }
        if artifacts
            .insert(artifact.id.clone(), artifact.path.clone())
            .is_some()
        {
            return invalid(
                "traceability",
                "traceability artifact identifiers must not repeat",
            );
        }
    }
    if policy.relationships.iter().any(|relationship| {
        !artifacts.contains_key(&relationship.from) || !artifacts.contains_key(&relationship.to)
    }) {
        return invalid(
            "traceability",
            "every traceability relationship must name declared artifacts",
        );
    }

    let mut report = Report::new("traceability", "artifact");
    let mut contents = BTreeMap::new();
    for (id, path) in &artifacts {
        match read(tree, path) {
            Ok(Some(text)) => {
                report.scanned(path);
                contents.insert(id.clone(), text);
            }
            Ok(None) => {
                report.found(Finding::new(
                    "missing-traceability-artifact",
                    path,
                    "the declared traceability artifact is absent",
                ));
            }
            Err(reason) => return invalid("traceability", reason),
        }
    }
    for relationship in &policy.relationships {
        let source = &artifacts[&relationship.from];
        let target = &artifacts[&relationship.to];
        if contents
            .get(&relationship.from)
            .is_some_and(|text| !markdown_targets(source, text).contains(target))
        {
            report.found(
                Finding::new(
                    "missing-traceability-relationship",
                    source,
                    "the declared artifact relationship has no matching local link",
                )
                .with("target", Detail::Text(target.clone())),
            );
        }
    }
    report
}

fn undeclared(category: &'static str, path: &str) -> Report {
    Report::refused(
        category,
        format!("{path} is not declared, and RHINO holds no default for it"),
    )
}

fn invalid(category: &'static str, reason: impl Into<String>) -> Report {
    Report::refused(category, reason.into())
}

fn read(tree: &dyn Tree, path: &str) -> Result<Option<String>, String> {
    match tree.read(path) {
        Ok(text) => Ok(Some(text)),
        Err(TreeError::NotFound) => Ok(None),
        Err(TreeError::Unreadable(reason)) => Err(format!("{path}: {reason}")),
        Err(TreeError::NotText) => Err(format!("{path}: holds no text")),
    }
}

fn exact_path(path: &str) -> bool {
    !path.trim().is_empty()
        && !path.starts_with('/')
        && path.split('/').all(|part| {
            !part.is_empty()
                && part != "."
                && part != ".."
                && !part.contains('\\')
                && !part.contains(['*', '?', '[', ']', '{', '}'])
        })
}

fn simple_name(value: &str) -> bool {
    !value.is_empty()
        && value.chars().all(|character| {
            character.is_ascii_lowercase() || character.is_ascii_digit() || character == '-'
        })
}

fn under(path: &str, root: &str) -> bool {
    path == root || path.starts_with(&format!("{root}/"))
}

fn excluded(path: &str, root: &str, exclusions: &[String]) -> bool {
    path.strip_prefix(&format!("{root}/"))
        .is_some_and(|relative| exclusions.iter().any(|excluded| under(relative, excluded)))
}

fn child_path(root: &str, relative: &str) -> Option<String> {
    exact_path(relative).then(|| format!("{root}/{relative}"))
}

fn vendor_exception(policy: &VendorPolicy, path: &str, term: &str) -> bool {
    policy.vocabulary_exceptions.iter().any(|exception| {
        exception.term == term && exception.paths.iter().any(|allowed| allowed == path)
    })
}

fn markdown_targets(path: &str, text: &str) -> BTreeSet<String> {
    text.split("](")
        .skip(1)
        .filter_map(|tail| tail.split_once(')').map(|(target, _)| target))
        .filter_map(|target| target.split_whitespace().next())
        .filter_map(|target| target.split(['?', '#']).next())
        .filter_map(|target| resolve_target(path, target))
        .collect()
}

fn resolve_target(source: &str, target: &str) -> Option<String> {
    if target.is_empty()
        || target.starts_with('/')
        || target.contains("://")
        || target.starts_with("mailto:")
    {
        return None;
    }
    let base = source
        .rsplit_once('/')
        .map_or("", |(directory, _)| directory);
    let mut parts: Vec<&str> = base.split('/').filter(|part| !part.is_empty()).collect();
    for part in target.split('/') {
        match part {
            "" | "." => {}
            ".." => {
                parts.pop()?;
            }
            part => parts.push(part),
        }
    }
    (!parts.is_empty()).then(|| parts.join("/"))
}

fn sha256(text: &str) -> String {
    Sha256::digest(text.as_bytes())
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}

fn is_sha256(value: &str) -> bool {
    value.len() == 64
        && value
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cli::Format;
    use crate::config::{AuthoringRule, Frontmatter, FrontmatterSurface, InternalLink, Mermaid};
    use crate::runtime::MemoryTree;
    use crate::v0_4::config::{
        LicensePath, LicensePolicy, ReadmeAnnotation, ReadmeIndexTree, TraceabilityArtifact,
        TraceabilityRelationship, VocabularyException,
    };

    fn outcome(report: Report) -> crate::Outcome {
        report.render(Format::Text)
    }

    fn markdown() -> MarkdownPolicy {
        MarkdownPolicy {
            frontmatter: Some(Frontmatter {
                surfaces: vec![FrontmatterSurface {
                    glob: "*.md".to_string(),
                    require: vec!["title".to_string()],
                    values: BTreeMap::new(),
                    iso_date: Vec::new(),
                    forbid: Vec::new(),
                }],
            }),
            internal_link: Some(InternalLink::default()),
            mermaid: Some(Mermaid {
                authoring_rule: Some(AuthoringRule::PlainText),
                node_label_graphemes: 80,
                edge_label_graphemes: 80,
                fill_colors: Vec::new(),
                edge_colors: Vec::new(),
                text_colors: Vec::new(),
            }),
            ..MarkdownPolicy::default()
        }
    }

    #[test]
    fn declared_license_and_markdown_policies_cover_clean_findings_and_refusals() {
        let tree = MemoryTree::default();
        tree.write("LICENSE", "SPDX: MIT\n");
        tree.write("docs/ok.md", "---\ntitle: ok\n---\n[text](target.md)\n");
        tree.write("docs/target.md", "---\ntitle: target\n---\n");
        tree.write(
            "docs/diagram.md",
            "---\ntitle: diagram\n---\n```mermaid\ngraph TD\n```\n",
        );

        assert_eq!(outcome(license(None, &tree)).exit_code, 2);
        assert_eq!(outcome(frontmatter(None, &tree)).exit_code, 2);
        assert_eq!(outcome(internal_link(None, &tree)).exit_code, 2);
        assert_eq!(
            outcome(mermaid(None, &tree, &Scope::default())).exit_code,
            2
        );

        let clean = ConventionsPolicy {
            license: Some(LicensePolicy {
                paths: vec![LicensePath {
                    path: "LICENSE".to_string(),
                    identifiers: vec!["MIT".to_string()],
                    sha256: Some(sha256("SPDX: MIT\n")),
                }],
                exclusions: Vec::new(),
            }),
        };
        assert_eq!(outcome(license(Some(&clean), &tree)).exit_code, 0);

        let invalid_policies = [
            ConventionsPolicy {
                license: Some(LicensePolicy {
                    paths: Vec::new(),
                    exclusions: vec!["../outside".to_string()],
                }),
            },
            ConventionsPolicy {
                license: Some(LicensePolicy {
                    paths: vec![LicensePath {
                        path: "/LICENSE".to_string(),
                        identifiers: Vec::new(),
                        sha256: None,
                    }],
                    exclusions: Vec::new(),
                }),
            },
            ConventionsPolicy {
                license: Some(LicensePolicy {
                    paths: vec![
                        LicensePath {
                            path: "LICENSE".to_string(),
                            identifiers: Vec::new(),
                            sha256: None,
                        },
                        LicensePath {
                            path: "LICENSE".to_string(),
                            identifiers: Vec::new(),
                            sha256: None,
                        },
                    ],
                    exclusions: Vec::new(),
                }),
            },
            ConventionsPolicy {
                license: Some(LicensePolicy {
                    paths: vec![LicensePath {
                        path: "LICENSE".to_string(),
                        identifiers: Vec::new(),
                        sha256: Some("not-a-digest".to_string()),
                    }],
                    exclusions: Vec::new(),
                }),
            },
        ];
        for policy in &invalid_policies {
            assert_eq!(outcome(license(Some(policy), &tree)).exit_code, 2);
        }

        let findings = ConventionsPolicy {
            license: Some(LicensePolicy {
                paths: vec![
                    LicensePath {
                        path: "missing".to_string(),
                        identifiers: Vec::new(),
                        sha256: None,
                    },
                    LicensePath {
                        path: "LICENSE".to_string(),
                        identifiers: vec!["Apache".to_string()],
                        sha256: Some("0".repeat(64)),
                    },
                    LicensePath {
                        path: "ignored".to_string(),
                        identifiers: vec!["unread".to_string()],
                        sha256: None,
                    },
                ],
                exclusions: vec!["ignored".to_string()],
            }),
        };
        let reported = outcome(license(Some(&findings), &tree));
        assert_eq!(reported.exit_code, 1);
        assert!(
            reported
                .stderr
                .contains("the configured license file is absent")
        );
        assert!(
            reported
                .stderr
                .contains("the configured license identifier is absent")
        );
        assert!(
            reported
                .stderr
                .contains("the configured license digest does not match")
        );

        let markdown = markdown();
        assert_eq!(outcome(frontmatter(Some(&markdown), &tree)).exit_code, 0);
        assert_eq!(outcome(internal_link(Some(&markdown), &tree)).exit_code, 0);
        assert_eq!(
            outcome(mermaid(Some(&markdown), &tree, &Scope::default())).exit_code,
            1
        );
    }

    #[test]
    fn readme_and_vendor_policies_cover_declared_tree_shapes_and_fail_closed_reads() {
        let tree = MemoryTree::default();
        tree.write("docs/README.md", "[guide](guide.md)\n");
        tree.write("docs/guide.md", "annotated\n");
        tree.write("docs/missing.md", "not indexed\n");
        tree.write("portable/code.md", "vendor-name\n");
        tree.write("portable/allowed.md", "vendor-name\n");
        tree.write("bindings/ignored.md", "vendor-name\n");

        assert_eq!(outcome(readme_index(None, &tree)).exit_code, 2);
        assert_eq!(outcome(vendor(None, &tree)).exit_code, 2);
        let invalid_readme = MarkdownPolicy {
            readme_index: Some(ReadmeIndexPolicy {
                trees: vec![ReadmeIndexTree {
                    path: "../docs".to_string(),
                    require_direct_children: false,
                    annotations: Vec::new(),
                    exclusions: Vec::new(),
                }],
            }),
            ..MarkdownPolicy::default()
        };
        assert_eq!(
            outcome(readme_index(Some(&invalid_readme), &tree)).exit_code,
            2
        );

        let readme = MarkdownPolicy {
            readme_index: Some(ReadmeIndexPolicy {
                trees: vec![ReadmeIndexTree {
                    path: "docs".to_string(),
                    require_direct_children: true,
                    annotations: vec![ReadmeAnnotation {
                        path: "guide.md".to_string(),
                        text: "annotated".to_string(),
                    }],
                    exclusions: vec!["missing.md".to_string()],
                }],
            }),
            ..MarkdownPolicy::default()
        };
        assert_eq!(outcome(readme_index(Some(&readme), &tree)).exit_code, 0);
        let missing_index = MarkdownPolicy {
            readme_index: Some(ReadmeIndexPolicy {
                trees: vec![ReadmeIndexTree {
                    path: "absent".to_string(),
                    require_direct_children: false,
                    annotations: Vec::new(),
                    exclusions: Vec::new(),
                }],
            }),
            ..MarkdownPolicy::default()
        };
        assert_eq!(
            outcome(readme_index(Some(&missing_index), &tree)).exit_code,
            1
        );

        let vendor_policy = GovernancePolicy {
            vendor: Some(VendorPolicy {
                roots: vec!["portable".to_string()],
                excluded_binding_roots: vec!["bindings".to_string()],
                forbidden_terms: vec!["vendor-name".to_string()],
                vocabulary_exceptions: vec![VocabularyException {
                    term: "vendor-name".to_string(),
                    paths: vec!["portable/allowed.md".to_string()],
                }],
            }),
            ..GovernancePolicy::default()
        };
        let reported = outcome(vendor(Some(&vendor_policy), &tree));
        assert_eq!(reported.exit_code, 1);
        assert!(
            reported
                .stderr
                .contains("a configured vendor term appears outside its exact exception")
        );
        assert!(reported.stderr.contains("portable/code.md"));

        let invalid_vendor = GovernancePolicy {
            vendor: Some(VendorPolicy {
                roots: vec!["/portable".to_string()],
                excluded_binding_roots: Vec::new(),
                forbidden_terms: vec![String::new()],
                vocabulary_exceptions: Vec::new(),
            }),
            ..GovernancePolicy::default()
        };
        assert_eq!(outcome(vendor(Some(&invalid_vendor), &tree)).exit_code, 2);
    }

    #[test]
    fn layer_and_traceability_policies_cover_every_declared_difference() {
        let tree = MemoryTree::default();
        tree.write("governance/principles/README.md", "present\n");
        tree.write("governance/extra/README.md", "unexpected\n");
        tree.write("docs/source.md", "no link\n");
        tree.write("docs/target.md", "target\n");

        assert_eq!(outcome(layers(None, &tree)).exit_code, 2);
        assert_eq!(outcome(traceability(None, &tree)).exit_code, 2);
        let layers_policy = GovernancePolicy {
            layers: Some(LayerPolicy {
                root: "governance".to_string(),
                order: vec!["principles".to_string(), "workflows".to_string()],
                categories: BTreeMap::from([(
                    "principles".to_string(),
                    vec!["general".to_string()],
                )]),
            }),
            ..GovernancePolicy::default()
        };
        let layered = outcome(layers(Some(&layers_policy), &tree));
        assert_eq!(layered.exit_code, 1);
        assert!(
            layered
                .stderr
                .contains("a declared governance layer is absent")
        );
        assert!(
            layered
                .stderr
                .contains("a declared governance category is absent")
        );
        assert!(
            layered
                .stderr
                .contains("the governance layer is not declared")
        );

        let invalid_layers = GovernancePolicy {
            layers: Some(LayerPolicy {
                root: "../governance".to_string(),
                order: vec!["Not-simple".to_string(), "Not-simple".to_string()],
                categories: BTreeMap::from([("missing".to_string(), vec!["bad_name".to_string()])]),
            }),
            ..GovernancePolicy::default()
        };
        assert_eq!(outcome(layers(Some(&invalid_layers), &tree)).exit_code, 2);

        let traceability_policy = GovernancePolicy {
            traceability: Some(TraceabilityPolicy {
                artifacts: vec![
                    TraceabilityArtifact {
                        id: "source".to_string(),
                        path: "docs/source.md".to_string(),
                    },
                    TraceabilityArtifact {
                        id: "target".to_string(),
                        path: "docs/target.md".to_string(),
                    },
                    TraceabilityArtifact {
                        id: "missing".to_string(),
                        path: "docs/missing.md".to_string(),
                    },
                ],
                relationships: vec![TraceabilityRelationship {
                    from: "source".to_string(),
                    to: "target".to_string(),
                }],
            }),
            ..GovernancePolicy::default()
        };
        let traceability_outcome = outcome(traceability(Some(&traceability_policy), &tree));
        assert_eq!(traceability_outcome.exit_code, 1);
        assert!(
            traceability_outcome
                .stderr
                .contains("the declared traceability artifact is absent")
        );
        assert!(
            traceability_outcome
                .stderr
                .contains("the declared artifact relationship has no matching local link")
        );

        let invalid_traceability = GovernancePolicy {
            traceability: Some(TraceabilityPolicy {
                artifacts: vec![TraceabilityArtifact {
                    id: "bad_id".to_string(),
                    path: "/outside".to_string(),
                }],
                relationships: vec![TraceabilityRelationship {
                    from: "missing".to_string(),
                    to: "also-missing".to_string(),
                }],
            }),
            ..GovernancePolicy::default()
        };
        assert_eq!(
            outcome(traceability(Some(&invalid_traceability), &tree)).exit_code,
            2
        );
    }

    #[test]
    fn validator_path_and_markdown_helpers_refuse_escapes_and_normalize_local_links() {
        assert!(exact_path("docs/a.md"));
        for path in ["", "/absolute", "docs//a", "docs/../a", "a\\b", "*.md"] {
            assert!(!exact_path(path), "{path}");
        }
        assert!(simple_name("v0-4"));
        assert!(!simple_name("Upper_case"));
        assert!(under("docs/a.md", "docs"));
        assert!(!under("documentation/a.md", "docs"));
        assert!(excluded(
            "docs/private/a.md",
            "docs",
            &["private".to_string()]
        ));
        assert_eq!(
            child_path("docs", "child.md"),
            Some("docs/child.md".to_string())
        );
        assert_eq!(child_path("docs", "../child.md"), None);
        assert_eq!(
            resolve_target("docs/a.md", "../target.md#fragment"),
            Some("target.md#fragment".to_string())
        );
        for target in [
            "",
            "/target.md",
            "https://example.test",
            "mailto:hi@example.test",
            "../../outside.md",
        ] {
            assert_eq!(resolve_target("docs/a.md", target), None, "{target}");
        }
        assert_eq!(
            markdown_targets("docs/a.md", "[good](b.md) [external](https://example.test)"),
            BTreeSet::from(["docs/b.md".to_string()])
        );
        assert!(is_sha256(&"a".repeat(64)));
        assert!(!is_sha256(&"A".repeat(64)));
    }

    #[test]
    fn validator_read_failures_and_remaining_declared_policy_findings_fail_closed() {
        let mut unreadable_license = MemoryTree::default();
        unreadable_license.write("LICENSE", "synthetic\n");
        unreadable_license.mark_unreadable("LICENSE");
        let license_policy = ConventionsPolicy {
            license: Some(LicensePolicy {
                paths: vec![LicensePath {
                    path: "LICENSE".to_string(),
                    identifiers: Vec::new(),
                    sha256: None,
                }],
                exclusions: Vec::new(),
            }),
        };
        assert_eq!(
            outcome(license(Some(&license_policy), &unreadable_license)).exit_code,
            2
        );

        let mut unreadable_index = MemoryTree::default();
        unreadable_index.write("docs/README.md", "index\n");
        unreadable_index.mark_binary("docs/README.md");
        let index_policy = MarkdownPolicy {
            readme_index: Some(ReadmeIndexPolicy {
                trees: vec![ReadmeIndexTree {
                    path: "docs".to_string(),
                    require_direct_children: false,
                    annotations: Vec::new(),
                    exclusions: Vec::new(),
                }],
            }),
            ..MarkdownPolicy::default()
        };
        assert_eq!(
            outcome(readme_index(Some(&index_policy), &unreadable_index)).exit_code,
            2
        );

        let annotations = MemoryTree::default();
        annotations.write("docs/README.md", "index\n");
        annotations.write("docs/present.md", "wrong text\n");
        let missing_annotation = MarkdownPolicy {
            readme_index: Some(ReadmeIndexPolicy {
                trees: vec![ReadmeIndexTree {
                    path: "docs".to_string(),
                    require_direct_children: false,
                    annotations: vec![
                        ReadmeAnnotation {
                            path: "missing.md".to_string(),
                            text: "required".to_string(),
                        },
                        ReadmeAnnotation {
                            path: "present.md".to_string(),
                            text: "required".to_string(),
                        },
                    ],
                    exclusions: Vec::new(),
                }],
            }),
            ..MarkdownPolicy::default()
        };
        let annotation_outcome = outcome(readme_index(Some(&missing_annotation), &annotations));
        assert_eq!(annotation_outcome.exit_code, 1);
        assert!(
            annotation_outcome
                .stderr
                .contains("the configured README annotation is absent")
        );
        let invalid_annotation = MarkdownPolicy {
            readme_index: Some(ReadmeIndexPolicy {
                trees: vec![ReadmeIndexTree {
                    path: "docs".to_string(),
                    require_direct_children: false,
                    annotations: vec![ReadmeAnnotation {
                        path: "../outside".to_string(),
                        text: "required".to_string(),
                    }],
                    exclusions: Vec::new(),
                }],
            }),
            ..MarkdownPolicy::default()
        };
        assert_eq!(
            outcome(readme_index(Some(&invalid_annotation), &annotations)).exit_code,
            2
        );

        let blank_vendor = GovernancePolicy {
            vendor: Some(VendorPolicy {
                roots: vec!["portable".to_string()],
                excluded_binding_roots: Vec::new(),
                forbidden_terms: vec![" ".to_string()],
                vocabulary_exceptions: Vec::new(),
            }),
            ..GovernancePolicy::default()
        };
        assert_eq!(
            outcome(vendor(Some(&blank_vendor), &annotations)).exit_code,
            2
        );
        let mut unavailable_vendor = MemoryTree::default();
        unavailable_vendor.write("portable/source.md", "synthetic\n");
        unavailable_vendor.mark_unreadable("portable/source.md");
        let vendor_policy = GovernancePolicy {
            vendor: Some(VendorPolicy {
                roots: vec!["portable".to_string()],
                excluded_binding_roots: Vec::new(),
                forbidden_terms: vec!["term".to_string()],
                vocabulary_exceptions: Vec::new(),
            }),
            ..GovernancePolicy::default()
        };
        assert_eq!(
            outcome(vendor(Some(&vendor_policy), &unavailable_vendor)).exit_code,
            2
        );

        let layers_tree = MemoryTree::default();
        layers_tree.write("governance/principles/custom/README.md", "synthetic\n");
        let layer_policy = GovernancePolicy {
            layers: Some(LayerPolicy {
                root: "governance".to_string(),
                order: vec!["principles".to_string()],
                categories: BTreeMap::from([("principles".to_string(), Vec::new())]),
            }),
            ..GovernancePolicy::default()
        };
        assert!(
            outcome(layers(Some(&layer_policy), &layers_tree))
                .stderr
                .contains("the governance category is not declared")
        );

        let duplicate_traceability = GovernancePolicy {
            traceability: Some(TraceabilityPolicy {
                artifacts: vec![
                    TraceabilityArtifact {
                        id: "artifact".to_string(),
                        path: "docs/a.md".to_string(),
                    },
                    TraceabilityArtifact {
                        id: "artifact".to_string(),
                        path: "docs/b.md".to_string(),
                    },
                ],
                relationships: Vec::new(),
            }),
            ..GovernancePolicy::default()
        };
        assert_eq!(
            outcome(traceability(Some(&duplicate_traceability), &annotations)).exit_code,
            2
        );
        let unknown_traceability = GovernancePolicy {
            traceability: Some(TraceabilityPolicy {
                artifacts: vec![TraceabilityArtifact {
                    id: "artifact".to_string(),
                    path: "docs/a.md".to_string(),
                }],
                relationships: vec![TraceabilityRelationship {
                    from: "artifact".to_string(),
                    to: "missing".to_string(),
                }],
            }),
            ..GovernancePolicy::default()
        };
        assert_eq!(
            outcome(traceability(Some(&unknown_traceability), &annotations)).exit_code,
            2
        );
        let mut unreadable_traceability = MemoryTree::default();
        unreadable_traceability.write("docs/a.md", "synthetic\n");
        unreadable_traceability.mark_binary("docs/a.md");
        let unreadable_traceability_policy = GovernancePolicy {
            traceability: Some(TraceabilityPolicy {
                artifacts: vec![TraceabilityArtifact {
                    id: "artifact".to_string(),
                    path: "docs/a.md".to_string(),
                }],
                relationships: Vec::new(),
            }),
            ..GovernancePolicy::default()
        };
        assert_eq!(
            outcome(traceability(
                Some(&unreadable_traceability_policy),
                &unreadable_traceability,
            ))
            .exit_code,
            2
        );
    }
}
